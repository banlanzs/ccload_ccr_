package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ccLoad+ccr/internal/model"
	"ccLoad+ccr/internal/util"

	"github.com/gin-gonic/gin"
)

// ============================================================
// Admin API: 获取渠道可用模型列表
// ============================================================

// FetchModelsRequest 获取模型列表请求参数
type FetchModelsRequest struct {
	ChannelType string `json:"channel_type" binding:"required"`
	URL         string `json:"url" binding:"required"`
	APIKey      string `json:"api_key" binding:"required"`

	// 格式转换配置（可选）
	EnableConversion       bool   `json:"enable_conversion"`
	ConversionSourceFormat string `json:"conversion_source_format,omitempty"`
	ConversionTargetFormat string `json:"conversion_target_format,omitempty"`
}

// FetchModelsResponse 获取模型列表响应
type FetchModelsResponse struct {
	Models      []model.ModelEntry `json:"models"`          // 模型列表（包含redirect_model便于编辑）
	ChannelType string             `json:"channel_type"`    // 渠道类型
	Source      string             `json:"source"`          // 数据来源: "api"(从API获取) 或 "predefined"(预定义)
	Debug       *FetchModelsDebug  `json:"debug,omitempty"` // 调试信息（仅开发环境）
}

// FetchModelsDebug 调试信息结构
type FetchModelsDebug struct {
	NormalizedType string `json:"normalized_type"` // 规范化后的渠道类型
	FetcherType    string `json:"fetcher_type"`    // 使用的Fetcher类型
	ChannelURL     string `json:"channel_url"`     // 渠道URL（脱敏）
}

// HandleFetchModels 获取指定渠道的可用模型列表
// 路由: GET /admin/channels/:id/models/fetch
// 功能:
//   - 根据渠道类型调用对应的Models API
//   - Anthropic/Codex/OpenAI/Gemini: 调用官方/v1/models接口
//   - 其它渠道: 返回预定义列表
//
// 设计模式: 适配器模式(Adapter Pattern) + 策略模式(Strategy Pattern)
func (s *Server) HandleFetchModels(c *gin.Context) {
	// 1. 解析路径参数
	channelID, err := ParseInt64Param(c, "id")
	if err != nil {
		RespondErrorMsg(c, http.StatusBadRequest, "无效的渠道ID")
		return
	}

	// 2. 查询渠道配置
	channel, err := s.channelCache.GetConfig(c.Request.Context(), channelID)
	if err != nil {
		RespondErrorMsg(c, http.StatusNotFound, "渠道不存在")
		return
	}

	// 3. 获取第一个API Key（用于调用Models API）
	keys, err := s.store.GetAPIKeys(c.Request.Context(), channelID)
	if err != nil || len(keys) == 0 {
		RespondErrorMsg(c, http.StatusBadRequest, "该渠道没有可用的API Key")
		return
	}
	apiKey := keys[0].APIKey

	// 4. 根据渠道配置执行模型抓取（支持query参数覆盖渠道类型）
	channelType := c.Query("channel_type")
	if channelType == "" {
		channelType = channel.ChannelType
	}

	// 从渠道配置读取格式转换配置
	response, err := fetchModelsForConfig(
		c.Request.Context(),
		channelType,
		channel.URL,
		apiKey,
		channel.EnableConversion,
		channel.ConversionSourceFormat,
		channel.ConversionTargetFormat,
	)
	if err != nil {
		// [INFO] 修复：统一返回200，通过success字段区分成功/失败（上游错误是预期内的）
		RespondErrorMsg(c, http.StatusOK, err.Error())
		return
	}

	RespondJSON(c, http.StatusOK, response)
}

// HandleFetchModelsPreview 支持未保存的渠道配置直接测试模型列表
// 路由: POST /admin/channels/models/fetch
func (s *Server) HandleFetchModelsPreview(c *gin.Context) {
	var req FetchModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondErrorMsg(c, http.StatusBadRequest, "参数无效: "+err.Error())
		return
	}

	req.ChannelType = strings.TrimSpace(req.ChannelType)
	req.URL = strings.TrimSpace(req.URL)
	req.APIKey = strings.TrimSpace(req.APIKey)
	req.ConversionSourceFormat = strings.TrimSpace(req.ConversionSourceFormat)
	req.ConversionTargetFormat = strings.TrimSpace(req.ConversionTargetFormat)

	if req.ChannelType == "" || req.URL == "" || req.APIKey == "" {
		RespondErrorMsg(c, http.StatusBadRequest, "channel_type、url、api_key为必填字段")
		return
	}

	response, err := fetchModelsForConfig(
		c.Request.Context(),
		req.ChannelType,
		req.URL,
		req.APIKey,
		req.EnableConversion,
		req.ConversionSourceFormat,
		req.ConversionTargetFormat,
	)
	if err != nil {
		// [INFO] 修复：统一返回200，通过success字段区分成功/失败（上游错误是预期内的）
		RespondErrorMsg(c, http.StatusOK, err.Error())
		return
	}
	RespondJSON(c, http.StatusOK, response)
}

// resolveEffectiveFormat 解析用于模型发现的有效协议格式
// 规则：
// 1. 如果启用了格式转换且源格式有效，使用源格式
// 2. 否则使用渠道类型
func resolveEffectiveFormat(channelType string, enableConversion bool, sourceFormat string) string {
	if enableConversion && sourceFormat != "" {
		normalized := util.NormalizeChannelType(sourceFormat)
		if normalized != "" {
			return sourceFormat
		}
	}
	return channelType
}

func fetchModelsForConfig(
	ctx context.Context,
	channelType, channelURL, apiKey string,
	enableConversion bool,
	sourceFormat, targetFormat string,
) (*FetchModelsResponse, error) {
	// 解析有效格式（用于选择 fetcher）
	effectiveFormat := resolveEffectiveFormat(channelType, enableConversion, sourceFormat)

	normalizedType := util.NormalizeChannelType(effectiveFormat)
	source := determineSource(effectiveFormat)

	var (
		modelNames []string
		fetcherStr string
		err        error
	)

	// 配置校验：如果启用转换，source 和 target 必须有效
	if enableConversion {
		if sourceFormat == "" || targetFormat == "" {
			return nil, fmt.Errorf("启用格式转换时，源格式和目标格式不能为空")
		}
		if util.NormalizeChannelType(sourceFormat) == "" {
			return nil, fmt.Errorf("无效的源格式: %s", sourceFormat)
		}
		if util.NormalizeChannelType(targetFormat) == "" {
			return nil, fmt.Errorf("无效的目标格式: %s", targetFormat)
		}
	}

	// 预定义模型列表
	if source == "predefined" {
		modelNames = util.PredefinedModels(normalizedType)
		if len(modelNames) == 0 {
			return nil, fmt.Errorf("渠道类型:%s 暂无预设模型列表", normalizedType)
		}
		fetcherStr = "predefined"
	} else {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		// 使用有效格式选择 fetcher
		fetcher := util.NewModelsFetcher(effectiveFormat)
		fetcherStr = fmt.Sprintf("%T", fetcher)

		modelNames, err = fetcher.FetchModels(ctx, channelURL, apiKey)
		if err != nil {
			return nil, fmt.Errorf(
				"获取模型列表失败(渠道类型:%s, 有效格式:%s, 规范化类型:%s, 数据来源:%s): %w",
				channelType, effectiveFormat, normalizedType, source, err,
			)
		}
	}

	// 转换为 ModelEntry 格式
	models := make([]model.ModelEntry, len(modelNames))
	for i, name := range modelNames {
		models[i] = model.ModelEntry{
			Model:         name,
			RedirectModel: name,
		}
	}

	return &FetchModelsResponse{
		Models:      models,
		ChannelType: channelType,
		Source:      source,
		Debug: &FetchModelsDebug{
			NormalizedType: normalizedType,
			FetcherType:    fetcherStr,
			ChannelURL:     channelURL,
		},
	}, nil
}

// determineSource 判断模型列表来源（辅助函数）
func determineSource(channelType string) string {
	switch util.NormalizeChannelType(channelType) {
	case util.ChannelTypeOpenAI, util.ChannelTypeGemini, util.ChannelTypeAnthropic, util.ChannelTypeCodex:
		return "api" // 从API获取
	default:
		return "predefined" // 预定义列表
	}
}
