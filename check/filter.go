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
	return matchesFilter(r, patterns, compileInputFilterPatterns())
}

func matchesFilter(r Result, patterns []*regexp.Regexp, input inputFilterPatterns) bool {
	if !matchesStructuredFilter(r, input) {
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
	input := compileInputFilterPatterns()

	slog.Info(fmt.Sprintf("应用节点过滤规则，共 %d 个正则表达式", len(patterns)))

	var filtered []Result
	for _, r := range results {
		if matchesFilter(r, patterns, input) {
			filtered = append(filtered, r)
		}
	}

	slog.Info(fmt.Sprintf("过滤后节点数量: %d (过滤前: %d)", len(filtered), len(results)))
	return filtered
}

// inputFilterPatterns contains the regular expressions used by the cheap
// name-based node filter. They are compiled once per check run and reused by
// every worker instead of being compiled for every node.
type inputFilterPatterns struct {
	include *regexp.Regexp
	exclude *regexp.Regexp
}

func compileInputFilterPatterns() inputFilterPatterns {
	f := config.GlobalConfig.NodeFilter
	return inputFilterPatterns{
		include: compileInputFilterPattern(f.NameInclude, "name-include"),
		exclude: compileInputFilterPattern(f.NameExclude, "name-exclude"),
	}
}

func compileInputFilterPattern(pattern, field string) *regexp.Regexp {
	if pattern == "" {
		return nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		// Config validation rejects this before a check starts. Keep this
		// helper fail-closed for callers that construct config in-process.
		slog.Error(fmt.Sprintf("%s 正则表达式无效，拒绝放行: %v", field, err))
		return regexp.MustCompile(`a\A`)
	}
	return re
}

// matchesInputFilter only examines cheap, original connection metadata.
func matchesInputFilter(proxy map[string]any, patterns inputFilterPatterns) bool {
	f := config.GlobalConfig.NodeFilter
	typ, _ := proxy["type"].(string)
	if len(f.Protocols) > 0 && !slices.Contains(f.Protocols, typ) {
		return false
	}
	name, _ := proxy["name"].(string)
	if patterns.include != nil && !patterns.include.MatchString(name) {
		return false
	}
	if patterns.exclude != nil && patterns.exclude.MatchString(name) {
		return false
	}
	return true
}

func matchesStructuredFilter(r Result, input inputFilterPatterns) bool {
	if r.Proxy == nil || !matchesInputFilter(r.Proxy, input) {
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
