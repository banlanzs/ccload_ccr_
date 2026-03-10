package app

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"ccLoad+ccr/internal/model"
	"ccLoad+ccr/internal/storage"
	"ccLoad+ccr/internal/util"
)

// ResolvedAssociationCandidate 模型解析候选
// 用于 selector 路由与 admin 预览返回
// Config 已注入虚拟模型映射（virtual -> resolved）
type ResolvedAssociationCandidate struct {
	Config        *model.Config
	VirtualModel  string
	ResolvedModel string
	ChannelID     int64
	ChannelName   string
	RuleID        int64
	MatchType     model.MatchType
	Pattern       string
	Priority      int
}

// ModelResolver 虚拟模型解析器
type ModelResolver struct {
	store storage.Store
}

// NewModelResolver 创建模型解析器
func NewModelResolver(store storage.Store) *ModelResolver {
	return &ModelResolver{store: store}
}

// Resolve 解析虚拟模型并返回可路由候选
// requestType 语义：渠道类型（openai/anthropic/gemini/codex）
func (r *ModelResolver) Resolve(ctx context.Context, virtualModelName string, requestType string) ([]*ResolvedAssociationCandidate, error) {
	if strings.TrimSpace(virtualModelName) == "" || virtualModelName == "*" {
		return nil, nil
	}

	vm, err := r.store.GetVirtualModelByName(ctx, virtualModelName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return nil, err
	}
	if vm == nil || !vm.Enabled {
		return nil, nil
	}

	rules, err := r.store.ListModelAssociations(ctx, vm.ID)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, nil
	}

	sort.SliceStable(rules, func(i, j int) bool {
		ri, rj := rules[i], rules[j]
		wi := matchTypeWeight(ri.MatchType)
		wj := matchTypeWeight(rj.MatchType)
		if wi != wj {
			return wi < wj // 权重越小优先级越高
		}
		if ri.Priority != rj.Priority {
			return ri.Priority > rj.Priority
		}
		return ri.ID < rj.ID
	})

	normalizedType := util.NormalizeChannelType(requestType)
	seenChannels := make(map[int64]struct{})
	candidates := make([]*ResolvedAssociationCandidate, 0)

	for _, rule := range rules {
		if rule == nil || !rule.Enabled {
			continue
		}

		// 根据匹配类型获取候选渠道列表
		var channels []*model.Config

		if rule.IsChannelMatch() {
			// 渠道内匹配：只检查指定渠道
			cfg, err := r.store.GetConfig(ctx, rule.ChannelID)
			if err != nil || cfg == nil || !cfg.Enabled {
				continue
			}
			if normalizedType != "" && cfg.GetChannelType() != normalizedType {
				continue
			}
			channels = []*model.Config{cfg}
		} else if rule.IsChannelTagsMatch() {
			// 标签匹配：获取所有启用渠道并过滤标签
			allChannels, err := r.store.ListConfigs(ctx)
			if err != nil {
				continue
			}
			requiredTags := model.ParseTags(rule.ChannelTags)
			for _, cfg := range allChannels {
				if !cfg.Enabled {
					continue
				}
				if normalizedType != "" && cfg.GetChannelType() != normalizedType {
					continue
				}
				// 检查渠道是否包含所有必需标签
				if containsAllTags(cfg.Tags, requiredTags) {
					channels = append(channels, cfg)
				}
			}
		} else {
			// 全局匹配：获取所有启用渠道
			allChannels, err := r.store.ListConfigs(ctx)
			if err != nil {
				continue
			}
			for _, cfg := range allChannels {
				if !cfg.Enabled {
					continue
				}
				if normalizedType != "" && cfg.GetChannelType() != normalizedType {
					continue
				}
				channels = append(channels, cfg)
			}
		}

		// 对每个候选渠道应用规则匹配
		for _, cfg := range channels {
			if _, exists := seenChannels[cfg.ID]; exists {
				continue
			}

			// 检查渠道是否被排除
			if rule.ShouldExcludeChannel(cfg.ID, cfg.Tags, cfg.Name) {
				continue
			}

			resolvedModel, matched := resolveModelByRule(cfg, rule)
			if !matched {
				continue
			}

			clonedCfg := cloneConfigWithVirtualMapping(cfg, virtualModelName, resolvedModel)
			candidates = append(candidates, &ResolvedAssociationCandidate{
				Config:        clonedCfg,
				VirtualModel:  virtualModelName,
				ResolvedModel: resolvedModel,
				ChannelID:     cfg.ID,
				ChannelName:   cfg.Name,
				RuleID:        rule.ID,
				MatchType:     rule.MatchType,
				Pattern:       rule.Pattern,
				Priority:      rule.Priority,
			})
			seenChannels[cfg.ID] = struct{}{}
		}
	}

	return candidates, nil
}

func cloneConfigWithVirtualMapping(cfg *model.Config, virtualModelName, resolvedModel string) *model.Config {
	if cfg == nil {
		return nil
	}

	clone := *cfg
	clone.ModelEntries = make([]model.ModelEntry, len(cfg.ModelEntries), len(cfg.ModelEntries)+1)
	copy(clone.ModelEntries, cfg.ModelEntries)

	for i := range clone.ModelEntries {
		if clone.ModelEntries[i].Model == virtualModelName {
			// 已有同名模型定义时，保留原配置，避免覆盖用户显式设置
			return &clone
		}
	}

	entry := model.ModelEntry{Model: virtualModelName}
	if resolvedModel != virtualModelName {
		entry.RedirectModel = resolvedModel
	}
	clone.ModelEntries = append(clone.ModelEntries, entry)
	return &clone
}

func resolveModelByRule(cfg *model.Config, rule *model.ModelAssociation) (string, bool) {
	if cfg == nil || rule == nil {
		return "", false
	}

	for _, entry := range cfg.ModelEntries {
		if associationMatches(rule.MatchType, rule.Pattern, entry.Model) {
			return entry.Model, true
		}
	}
	return "", false
}

func associationMatches(matchType model.MatchType, pattern, value string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || value == "" {
		return false
	}

	switch matchType {
	case model.MatchTypeExact:
		return value == pattern
	case model.MatchTypePrefix:
		return strings.HasPrefix(value, pattern)
	case model.MatchTypeSuffix:
		return strings.HasSuffix(value, pattern)
	case model.MatchTypeContains:
		return strings.Contains(value, pattern)
	case model.MatchTypeRegex:
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		return re.MatchString(value)
	case model.MatchTypeWildcard:
		re, err := wildcardToRegex(pattern)
		if err != nil {
			return false
		}
		return re.MatchString(value)
	default:
		return false
	}
}

func wildcardToRegex(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	for _, r := range pattern {
		switch r {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		case '.', '+', '(', ')', '[', ']', '{', '}', '^', '$', '|', '\\':
			b.WriteString("\\")
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteString("$")

	re, err := regexp.Compile(b.String())
	if err != nil {
		return nil, fmt.Errorf("compile wildcard regex: %w", err)
	}
	return re, nil
}

func matchTypeWeight(matchType model.MatchType) int {
	switch matchType {
	case model.MatchTypeExact:
		return 0
	case model.MatchTypePrefix:
		return 1
	case model.MatchTypeSuffix:
		return 2
	case model.MatchTypeContains:
		return 3
	case model.MatchTypeRegex:
		return 4
	case model.MatchTypeWildcard:
		return 5
	default:
		return 100
	}
}

// containsAllTags 检查 channelTags 是否包含 requiredTags 中的所有标签
func containsAllTags(channelTags, requiredTags []string) bool {
	if len(requiredTags) == 0 {
		return false
	}
	tagSet := make(map[string]struct{}, len(channelTags))
	for _, tag := range channelTags {
		tagSet[strings.TrimSpace(tag)] = struct{}{}
	}
	for _, required := range requiredTags {
		if _, exists := tagSet[strings.TrimSpace(required)]; !exists {
			return false
		}
	}
	return true
}
