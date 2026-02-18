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

	// 新格式转换配置（可选）
	EnableConversion       bool   `json:"enable_conversion"`
	ConversionSourceFormat string `json:"conversion_source_format,omitempty"`
	ConversionTargetFormat string `json:"conversion_target_format,omitempty"`

	// 旧的 CCR 格式转换配置（可选）
	EnableCCR      bool   `json:"enable_ccr"`
	CCRTransformer string `json:"ccr_transformer"`
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

// BatchRefreshModelsRequest 批量刷新模型请求
type BatchRefreshModelsRequest struct {
	ChannelIDs  []int64 `json:"channel_ids"`
	Mode        string  `json:"mode"`                   // merge(增量,默认) / replace(覆盖)
	ChannelType string  `json:"channel_type,omitempty"` // 可选：覆盖渠道类型
}

// BatchRefreshModelsItem 批量刷新单渠道结果
type BatchRefreshModelsItem struct {
	ChannelID   int64  `json:"channel_id"`
	ChannelName string `json:"channel_name,omitempty"`
	Status      string `json:"status"` // updated / unchanged / failed
	Error       string `json:"error,omitempty"`
	Fetched     int    `json:"fetched"`
	Added       int    `json:"added,omitempty"`   // merge模式
	Removed     int    `json:"removed,omitempty"` // replace模式
	Total       int    `json:"total"`             // 刷新后总模型数
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
		channel.EnableCCR,
		channel.CCRTransformer,
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
	req.CCRTransformer = strings.TrimSpace(req.CCRTransformer)

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
		req.EnableCCR,
		req.CCRTransformer,
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
// 1. 优先使用新格式转换系统（EnableConversion + ConversionSourceFormat）
// 2. 回退到旧的 CCR 系统（EnableCCR + CCRTransformer）
// 3. 最后使用渠道类型
func resolveEffectiveFormat(channelType string, enableConversion bool, sourceFormat string, enableCCR bool, ccrTransformer string) string {
	// 新格式转换系统
	if enableConversion && sourceFormat != "" {
		normalized := util.NormalizeChannelType(sourceFormat)
		if normalized != "" {
			return sourceFormat
		}
	}

	// 旧的 CCR 系统
	if enableCCR && ccrTransformer != "" {
		switch ccrTransformer {
		case "openai_to_claude":
			return "openai" // 源格式是 OpenAI
		case "claude_to_openai":
			return "anthropic" // 源格式是 Claude (Anthropic)
		}
	}

	return channelType
}

// HandleBatchRefreshModels 批量获取并刷新渠道模型
// 路由: POST /admin/channels/models/refresh-batch
func (s *Server) HandleBatchRefreshModels(c *gin.Context) {
	var req BatchRefreshModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondErrorMsg(c, http.StatusBadRequest, "参数无效: "+err.Error())
		return
	}

	channelIDs := normalizeBatchChannelIDs(req.ChannelIDs)
	if len(channelIDs) == 0 {
		RespondErrorMsg(c, http.StatusBadRequest, "channel_ids不能为空")
		return
	}

	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = "merge"
	}
	if mode != "merge" && mode != "replace" {
		RespondErrorMsg(c, http.StatusBadRequest, "mode 仅支持 merge 或 replace")
		return
	}

	overrideType := strings.TrimSpace(req.ChannelType)
	ctx := c.Request.Context()

	results := make([]BatchRefreshModelsItem, 0, len(channelIDs))
	updated := 0
	unchanged := 0
	failed := 0
	changed := false

	for _, channelID := range channelIDs {
		item := BatchRefreshModelsItem{ChannelID: channelID}

		cfg, err := s.store.GetConfig(ctx, channelID)
		if err != nil {
			item.Status = "failed"
			item.Error = "渠道不存在"
			failed++
			results = append(results, item)
			continue
		}
		item.ChannelName = cfg.Name

		keys, err := s.store.GetAPIKeys(ctx, channelID)
		if err != nil || len(keys) == 0 {
			item.Status = "failed"
			item.Error = "该渠道没有可用的API Key"
			failed++
			results = append(results, item)
			continue
		}

		apiKey := strings.TrimSpace(keys[0].APIKey)
		if apiKey == "" {
			item.Status = "failed"
			item.Error = "该渠道没有可用的API Key"
			failed++
			results = append(results, item)
			continue
		}

		channelType := overrideType
		if channelType == "" {
			channelType = cfg.ChannelType
		}

		// 从渠道配置读取格式转换配置
		resp, err := fetchModelsForConfig(
			ctx,
			channelType,
			cfg.URL,
			apiKey,
			cfg.EnableConversion,
			cfg.ConversionSourceFormat,
			cfg.ConversionTargetFormat,
			cfg.EnableCCR,
			cfg.CCRTransformer,
		)
		if err != nil {
			item.Status = "failed"
			item.Error = err.Error()
			failed++
			results = append(results, item)
			continue
		}

		fetched := normalizeModelEntriesForSave(resp.Models)
		item.Fetched = len(fetched)

		switch mode {
		case "replace":
			removed, hasChange := replaceModelEntries(cfg, fetched)
			item.Removed = removed
			item.Total = len(cfg.ModelEntries)

			if !hasChange {
				item.Status = "unchanged"
				unchanged++
				results = append(results, item)
				continue
			}
		default: // merge
			added, hasChange := mergeModelEntries(cfg, fetched)
			item.Added = added
			item.Total = len(cfg.ModelEntries)

			if !hasChange {
				item.Status = "unchanged"
				unchanged++
				results = append(results, item)
				continue
			}
		}

		if _, err := s.store.UpdateConfig(ctx, channelID, cfg); err != nil {
			item.Status = "failed"
			item.Error = "保存模型失败: " + err.Error()
			failed++
			results = append(results, item)
			continue
		}

		item.Status = "updated"
		updated++
		changed = true
		results = append(results, item)
	}

	if changed {
		s.InvalidateChannelListCache()
	}

	RespondJSON(c, http.StatusOK, gin.H{
		"mode":      mode,
		"total":     len(channelIDs),
		"updated":   updated,
		"unchanged": unchanged,
		"failed":    failed,
		"results":   results,
	})
}

func fetchModelsForConfig(
	ctx context.Context,
	channelType, channelURL, apiKey string,
	enableConversion bool,
	sourceFormat, targetFormat string,
	enableCCR bool,
	ccrTransformer string,
) (*FetchModelsResponse, error) {
	// 解析有效格式（用于选择 fetcher）
	effectiveFormat := resolveEffectiveFormat(channelType, enableConversion, sourceFormat, enableCCR, ccrTransformer)

	normalizedType := util.NormalizeChannelType(effectiveFormat)
	source := determineSource(effectiveFormat)

	var (
		modelNames []string
		fetcherStr string
		err        error
	)

	// 互斥验证：CCR 和新格式转换系统不能同时启用
	if enableCCR && enableConversion {
		return nil, fmt.Errorf("CCR 格式转换和新格式转换系统不能同时启用，请只选择其中一个")
	}

	// 配置校验：如果启用转换，source 必须有效，target 可以为空（使用 channelType）
	if enableConversion {
		if sourceFormat == "" {
			return nil, fmt.Errorf("启用格式转换时，源格式不能为空")
		}
		if util.NormalizeChannelType(sourceFormat) == "" {
			return nil, fmt.Errorf("无效的源格式: %s", sourceFormat)
		}
		// 目标格式为空时，使用渠道类型
		if targetFormat == "" {
			targetFormat = channelType
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

func normalizeModelEntriesForSave(entries []model.ModelEntry) []model.ModelEntry {
	if len(entries) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(entries))
	normalized := make([]model.ModelEntry, 0, len(entries))
	for _, entry := range entries {
		clean := entry
		if err := clean.Validate(); err != nil {
			continue
		}
		if clean.Model == "" {
			continue
		}
		if clean.RedirectModel == clean.Model {
			clean.RedirectModel = ""
		}
		key := strings.ToLower(clean.Model)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, clean)
	}
	return normalized
}

func mergeModelEntries(cfg *model.Config, fetched []model.ModelEntry) (added int, changed bool) {
	existing := make(map[string]struct{}, len(cfg.ModelEntries))
	for _, entry := range cfg.ModelEntries {
		existing[strings.ToLower(entry.Model)] = struct{}{}
	}

	for _, entry := range fetched {
		key := strings.ToLower(entry.Model)
		if _, exists := existing[key]; exists {
			continue
		}
		cfg.ModelEntries = append(cfg.ModelEntries, entry)
		existing[key] = struct{}{}
		added++
	}

	return added, added > 0
}

func replaceModelEntries(cfg *model.Config, fetched []model.ModelEntry) (removed int, changed bool) {
	oldEntries := cfg.ModelEntries
	oldSet := make(map[string]struct{}, len(oldEntries))
	newSet := make(map[string]struct{}, len(fetched))

	for _, entry := range oldEntries {
		oldSet[strings.ToLower(entry.Model)] = struct{}{}
	}
	for _, entry := range fetched {
		newSet[strings.ToLower(entry.Model)] = struct{}{}
	}
	for key := range oldSet {
		if _, exists := newSet[key]; !exists {
			removed++
		}
	}

	if len(oldEntries) == len(fetched) {
		same := true
		for i := range oldEntries {
			if oldEntries[i].Model != fetched[i].Model || oldEntries[i].RedirectModel != fetched[i].RedirectModel {
				same = false
				break
			}
		}
		if same {
			return 0, false
		}
	}

	cfg.ModelEntries = fetched
	return removed, true
}
