package check

import (
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/beihehele/subs-check/config"
	proxies "github.com/beihehele/subs-check/proxy"
)

// CompileFilterPatterns compiles the configured filter regex list.
// Invalid patterns fail closed; config validation normally rejects them earlier.
func CompileFilterPatterns() []*regexp.Regexp {
	if len(config.GlobalConfig.Filter) == 0 {
		return nil
	}
	var patterns []*regexp.Regexp
	for _, pattern := range config.GlobalConfig.Filter {
		re, err := regexp.Compile(pattern)
		if err != nil {
			slog.Error(fmt.Sprintf("过滤正则表达式无效，拒绝放行: %s, 错误: %v", pattern, err))
			return []*regexp.Regexp{regexp.MustCompile(`a\A`)}
		}
		patterns = append(patterns, re)
	}
	return patterns
}

// MatchesFilter reports whether r's rendered name (without speed tag)
// matches any pattern. An empty pattern slice counts as "passes".
func MatchesFilter(r Result, patterns []*regexp.Regexp) bool {
	if !matchesStructuredFilter(r) {
		return false
	}
	if len(patterns) == 0 {
		return true
	}
	if r.Proxy == nil {
		return false
	}
	// Stable names omit dynamic tags, but legacy filters still see measured tags.
	parts := RenderNameParts(r, false)
	if config.GlobalConfig.RenameNode {
		parts.Base = config.GlobalConfig.NodePrefix + proxies.FormatRename(regionKey(r), 0)
	}
	name := parts.TaggedString()
	for _, re := range patterns {
		if re.MatchString(name) {
			return true
		}
	}
	return false
}

// FilterResults 根据配置的正则表达式过滤节点。
//
// 只有渲染后的展示名(不含速度标签)匹配任一正则的节点才会被保留。
// 这里用 RenderName(r, false) 而不是 r.Proxy["name"] 是为了让 filter 能看到
// 国家+媒体标签的完整视图,同时保持 proxy["name"] 不被修改。
func FilterResults(results []Result) []Result {
	patterns := CompileFilterPatterns()

	slog.Info(fmt.Sprintf("应用节点过滤规则，共 %d 个正则表达式", len(patterns)))

	var filtered []Result
	for _, r := range results {
		if MatchesFilter(r, patterns) {
			filtered = append(filtered, r)
		}
	}

	slog.Info(fmt.Sprintf("过滤后节点数量: %d (过滤前: %d)", len(filtered), len(results)))
	return filtered
}

// matchesInputFilter only examines cheap, original connection metadata.
func matchesInputFilter(proxy map[string]any) bool {
	f := config.GlobalConfig.NodeFilter
	typ, _ := proxy["type"].(string)
	if len(f.Protocols) > 0 && !slices.Contains(f.Protocols, typ) {
		return false
	}
	name, _ := proxy["name"].(string)
	for _, rule := range []struct {
		pattern string
		include bool
	}{{f.NameInclude, true}, {f.NameExclude, false}} {
		if rule.pattern == "" {
			continue
		}
		re, err := regexp.Compile(rule.pattern)
		if err != nil || re.MatchString(name) != rule.include {
			return false
		}
	}
	return true
}

func matchesStructuredFilter(r Result) bool {
	if r.Proxy == nil || !matchesInputFilter(r.Proxy) {
		return false
	}
	f := config.GlobalConfig.NodeFilter
	if len(f.Regions) > 0 {
		matched := false
		for _, code := range f.Regions {
			if normalizeRegion(code) == regionKey(r) {
				matched = true
			}
		}
		if !matched {
			return false
		}
	}
	if f.MaxLatency > 0 && (r.Latency <= 0 || r.Latency > f.MaxLatency) {
		return false
	}
	if f.MaxIPRisk != nil {
		risk, err := strconv.Atoi(strings.TrimSuffix(r.IPRisk, "%"))
		if err != nil || risk < 0 || risk > *f.MaxIPRisk {
			return false
		}
	}
	for _, p := range f.RequirePlatforms {
		if mediaTagFor(p, &r) == "" {
			return false
		}
	}
	return true
}
