package model

import (
	"errors"
	"strings"
)

// VirtualModel 虚拟模型定义
type VirtualModel struct {
	ID                int64    `json:"id"`
	Name              string   `json:"name"`                       // 虚拟模型名称（唯一）
	Alias             string   `json:"alias,omitempty"`            // 别名
	Description       string   `json:"description,omitempty"`      // 描述
	Enabled           bool     `json:"enabled"`                    // 是否启用
	DefaultFallback   string   `json:"default_fallback,omitempty"` // 默认回退模型
	AssociationsCount int      `json:"associations_count"`         // 关联规则数量
	CreatedAt         JSONTime `json:"created_at"`
	UpdatedAt         JSONTime `json:"updated_at"`
}

// Validate 验证虚拟模型字段
func (v *VirtualModel) Validate() error {
	v.Name = strings.TrimSpace(v.Name)
	if v.Name == "" {
		return errors.New("virtual model name cannot be empty")
	}
	if len(v.Name) > 128 {
		return errors.New("virtual model name too long (max 128 characters)")
	}
	if strings.ContainsAny(v.Name, "\x00\r\n") {
		return errors.New("virtual model name contains illegal characters")
	}

	v.Alias = strings.TrimSpace(v.Alias)
	if len(v.Alias) > 128 {
		return errors.New("alias too long (max 128 characters)")
	}

	v.DefaultFallback = strings.TrimSpace(v.DefaultFallback)
	if len(v.DefaultFallback) > 256 {
		return errors.New("default fallback too long (max 256 characters)")
	}

	return nil
}

// MatchType 匹配类型
type MatchType string

const (
	MatchTypeExact    MatchType = "exact"    // 精确匹配
	MatchTypePrefix   MatchType = "prefix"   // 前缀匹配
	MatchTypeSuffix   MatchType = "suffix"   // 后缀匹配
	MatchTypeContains MatchType = "contains" // 包含匹配
	MatchTypeRegex    MatchType = "regex"    // 正则匹配
	MatchTypeWildcard MatchType = "wildcard" // 通配符匹配
)

// IsValid 检查匹配类型是否有效
func (m MatchType) IsValid() bool {
	switch m {
	case MatchTypeExact, MatchTypePrefix, MatchTypeSuffix, MatchTypeContains, MatchTypeRegex, MatchTypeWildcard:
		return true
	default:
		return false
	}
}

// ModelAssociation 模型关联规则
type ModelAssociation struct {
	ID             int64     `json:"id"`
	VirtualModelID int64     `json:"virtual_model_id"` // 虚拟模型ID
	ChannelID      int64     `json:"channel_id"`       // 渠道ID（0表示全局匹配）
	ChannelTags    string    `json:"channel_tags"`     // 渠道标签（逗号分隔，用于标签匹配）
	MatchType      MatchType `json:"match_type"`       // 匹配类型
	Pattern        string    `json:"pattern"`          // 匹配模式
	Priority       int       `json:"priority"`         // 优先级（数值越大优先级越高）
	Enabled        bool      `json:"enabled"`          // 是否启用
	CreatedAt      JSONTime  `json:"created_at"`
	UpdatedAt      JSONTime  `json:"updated_at"`
}

// IsGlobalMatch 判断是否为全局匹配
func (m *ModelAssociation) IsGlobalMatch() bool {
	return m.ChannelID == 0 && m.ChannelTags == ""
}

// IsChannelTagsMatch 判断是否为标签匹配
func (m *ModelAssociation) IsChannelTagsMatch() bool {
	return m.ChannelTags != ""
}

// IsChannelMatch 判断是否为渠道内匹配
func (m *ModelAssociation) IsChannelMatch() bool {
	return m.ChannelID > 0 && m.ChannelTags == ""
}

// Validate 验证模型关联规则
func (m *ModelAssociation) Validate() error {
	if m.VirtualModelID <= 0 {
		return errors.New("virtual_model_id must be positive")
	}

	// 验证关联类型：channel_id 和 channel_tags 不能同时为空
	if m.ChannelID == 0 && m.ChannelTags == "" {
		// 全局匹配：允许
	} else if m.ChannelID > 0 && m.ChannelTags == "" {
		// 渠道内匹配：允许
	} else if m.ChannelID == 0 && m.ChannelTags != "" {
		// 标签匹配：允许
	} else {
		return errors.New("channel_id and channel_tags cannot both be set")
	}

	if !m.MatchType.IsValid() {
		return errors.New("invalid match_type")
	}

	m.Pattern = strings.TrimSpace(m.Pattern)
	if m.Pattern == "" {
		return errors.New("pattern cannot be empty")
	}
	if len(m.Pattern) > 256 {
		return errors.New("pattern too long (max 256 characters)")
	}
	if strings.ContainsAny(m.Pattern, "\x00\r\n") {
		return errors.New("pattern contains illegal characters")
	}

	return nil
}

// ModelAssociationWithDetails 带详情的模型关联规则（用于API响应）
type ModelAssociationWithDetails struct {
	ModelAssociation
	VirtualModelName string `json:"virtual_model_name,omitempty"` // 虚拟模型名称
	ChannelName      string `json:"channel_name,omitempty"`       // 渠道名称
}

// ParseTags 解析逗号分隔的标签字符串
func ParseTags(tagsStr string) []string {
	tagsStr = strings.TrimSpace(tagsStr)
	if tagsStr == "" {
		return []string{}
	}
	parts := strings.Split(tagsStr, ",")
	tags := make([]string, 0, len(parts))
	for _, tag := range parts {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

// FormatTags 格式化标签列表为逗号分隔字符串
func FormatTags(tags []string) string {
	filtered := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			filtered = append(filtered, tag)
		}
	}
	return strings.Join(filtered, ",")
}
