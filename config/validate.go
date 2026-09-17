package config

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"

	"github.com/biter777/countries"
)

var SupportedPlatforms = []string{"iprisk", "openai", "youtube", "netflix", "disney", "gemini", "claude", "spotify", "tiktok", "telegram"}

func (c *Config) Validate() error {
	for _, pattern := range append(append([]string{}, c.Filter...), c.NodeFilter.NameInclude, c.NodeFilter.NameExclude) {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("无效的过滤正则 %q: %w", pattern, err)
		}
	}
	if !slices.Contains([]string{"", "compact", "stable"}, c.NameMode) {
		return fmt.Errorf("name-mode 必须为 compact 或 stable")
	}
	if c.NameMode == "stable" && !c.RenameNode {
		return fmt.Errorf("name-mode: stable 需要 rename-node: true")
	}
	if !slices.Contains([]string{"", "balanced", "quality", "hybrid"}, c.Selection.Mode) {
		return fmt.Errorf("selection.mode 必须为 balanced、quality 或 hybrid")
	}
	if !slices.Contains([]string{"", "speed", "latency", "balanced"}, c.Selection.Sort) {
		return fmt.Errorf("selection.sort 必须为 speed、latency 或 balanced")
	}
	if c.NodeFilter.MaxLatency < 0 || c.Selection.MinPerRegion < 0 || c.Selection.MaxPerSource < 0 || c.Selection.MaxPerIP < 0 || c.Selection.ExploreSlots < 0 || c.SpeedMinSampleKB < 0 {
		return fmt.Errorf("筛选阈值和配额不能小于 0")
	}
	if risk := c.NodeFilter.MaxIPRisk; risk != nil && (*risk < 0 || *risk > 100) {
		return fmt.Errorf("max-ip-risk 必须在 0 到 100 之间")
	}
	seen := map[string]bool{}
	for _, p := range c.Platforms {
		if !slices.Contains(SupportedPlatforms, p) {
			return fmt.Errorf("未知检测平台 %q", p)
		}
		if seen[p] {
			return fmt.Errorf("重复检测平台 %q", p)
		}
		seen[p] = true
	}
	for _, p := range c.NodeFilter.RequirePlatforms {
		if p == "iprisk" || !slices.Contains(SupportedPlatforms, p) {
			return fmt.Errorf("不支持的平台筛选 %q", p)
		}
		if !c.MediaCheck || !seen[p] {
			return fmt.Errorf("筛选 %s 需要 media-check: true 并在 platforms 中启用它", p)
		}
	}
	if c.NodeFilter.MaxIPRisk != nil && (!c.MediaCheck || !seen["iprisk"]) {
		return fmt.Errorf("max-ip-risk 需要启用 media-check 和 iprisk")
	}
	regionKey := func(code string) string {
		code = strings.ToUpper(strings.TrimSpace(code))
		if code == "UK" {
			code = "GB"
		}
		if code == "OTHER" {
			return code
		}
		country := countries.ByName(code)
		if country == countries.Unknown {
			return ""
		}
		return country.Alpha2()
	}
	validRegion := func(code string) bool { return regionKey(code) != "" }
	for _, region := range append(append([]string{}, c.NodeFilter.Regions...), c.Selection.PreferredRegions...) {
		if !validRegion(region) {
			return fmt.Errorf("未知地区 %q", region)
		}
	}
	weights := map[string]bool{}
	for region, weight := range c.Selection.RegionWeights {
		key := regionKey(region)
		if weights[key] {
			return fmt.Errorf("重复地区权重 %q", region)
		}
		weights[key] = true
		if !validRegion(region) || weight <= 0 || math.IsNaN(weight) || math.IsInf(weight, 0) {
			return fmt.Errorf("无效地区权重 %q", region)
		}
	}
	return nil
}
