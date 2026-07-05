package check

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/beihehele/subs-check/config"
)

const regionOther = "OTHER"

type indexedResult struct {
	r   Result
	idx int
}

var regionPriority = []string{
	"HK", "TW", "MO", "SG", "JP", "KR",
	"US", "CA", "UK", "DE", "FR", "NL", "AU",
	"IN", "RU", "TR", "VN", "TH", "MY", "PH",
}

// regionKey returns the grouping key for a result.
// Priority: Country → proxy name keywords → OTHER.
func regionKey(r Result) string {
	if r.Country != "" {
		return strings.ToUpper(r.Country)
	}
	if code := inferRegionFromName(proxyDisplayName(r)); code != "" {
		return code
	}
	return regionOther
}

func proxyDisplayName(r Result) string {
	if r.Proxy == nil {
		return ""
	}
	if n, ok := r.Proxy["name"].(string); ok {
		return strings.TrimSpace(n)
	}
	return ""
}

// inferRegionFromName extracts ISO-like region code from common keywords in node names.
func inferRegionFromName(name string) string {
	if name == "" {
		return ""
	}
	upper := strings.ToUpper(name)
	for code, keywords := range regionNameKeywords {
		for _, kw := range keywords {
			if strings.Contains(upper, strings.ToUpper(kw)) {
				return code
			}
		}
	}
	return ""
}

var regionNameKeywords = map[string][]string{
	"HK": {"香港", "HONG KONG", "HONGKONG", "HK"},
	"TW": {"台湾", "TAIWAN", "TW"},
	"SG": {"新加坡", "SINGAPORE", "SG"},
	"JP": {"日本", "JAPAN", "JP"},
	"US": {"美国", "UNITED STATES", "USA", "US"},
	"KR": {"韩国", "KOREA", "KR"},
	"UK": {"英国", "UNITED KINGDOM", "UK", "GB"},
	"DE": {"德国", "GERMANY", "DE"},
	"FR": {"法国", "FRANCE", "FR"},
	"CA": {"加拿大", "CANADA", "CA"},
	"AU": {"澳大利亚", "AUSTRALIA", "AU"},
	"NL": {"荷兰", "NETHERLANDS", "NL"},
	"IN": {"印度", "INDIA", "IN"},
	"RU": {"俄罗斯", "RUSSIA", "RU"},
	"TR": {"土耳其", "TURKEY", "TR"},
	"VN": {"越南", "VIETNAM", "VN"},
	"TH": {"泰国", "THAILAND", "TH"},
	"MY": {"马来西亚", "MALAYSIA", "MY"},
	"PH": {"菲律宾", "PHILIPPINES", "PH"},
}

func compareRegionKeys(a, b string) int {
	if a == regionOther && b != regionOther {
		return 1
	}
	if b == regionOther && a != regionOther {
		return -1
	}
	pa, oa := regionPriorityIndex(a)
	pb, ob := regionPriorityIndex(b)
	if oa && !ob {
		return 1
	}
	if !oa && ob {
		return -1
	}
	if pa != pb {
		return pa - pb
	}
	return strings.Compare(a, b)
}

func regionPriorityIndex(code string) (int, bool) {
	for i, c := range regionPriority {
		if c == code {
			return i, false
		}
	}
	return 0, true
}

func sortRegionKeys(keys []string) {
	sort.Slice(keys, func(i, j int) bool {
		return compareRegionKeys(keys[i], keys[j]) < 0
	})
}

// lessResult reports whether a should rank before b (a is better).
func lessResult(a, b indexedResult, hasSpeed bool) bool {
	if hasSpeed && a.r.Speed != b.r.Speed {
		return a.r.Speed > b.r.Speed
	}
	if a.r.Latency != b.r.Latency {
		return a.r.Latency < b.r.Latency
	}
	return a.idx < b.idx
}

func sortIndexed(items []indexedResult, hasSpeed bool) {
	sort.Slice(items, func(i, j int) bool {
		return lessResult(items[i], items[j], hasSpeed)
	})
}

func buildRegionGroups(results []Result) (map[string][]indexedResult, []string) {
	hasSpeed := config.GlobalConfig.SpeedTestUrl != ""

	indexed := make([]indexedResult, len(results))
	for i, r := range results {
		indexed[i] = indexedResult{r: r, idx: i}
	}

	groups := make(map[string][]indexedResult)
	for _, item := range indexed {
		key := regionKey(item.r)
		groups[key] = append(groups[key], item)
	}

	regionKeys := make([]string, 0, len(groups))
	for k := range groups {
		regionKeys = append(regionKeys, k)
	}
	sortRegionKeys(regionKeys)

	for _, k := range regionKeys {
		sortIndexed(groups[k], hasSpeed)
	}
	return groups, regionKeys
}

// SortResultsByRegion groups nodes by region for final output.
// Regions are ordered by common priority (HK/TW/SG... first, OTHER last);
// within each region nodes are sorted by speed desc, latency asc.
func SortResultsByRegion(results []Result) []Result {
	if len(results) == 0 {
		return results
	}
	groups, regionKeys := buildRegionGroups(results)
	return flattenRegionGroups(groups, regionKeys)
}

// OutputOrderIndices returns result slice indices in region-sorted output order.
func OutputOrderIndices(results []Result) []int {
	if len(results) == 0 {
		return nil
	}
	groups, regionKeys := buildRegionGroups(results)
	indices := make([]int, 0, len(results))
	for _, k := range regionKeys {
		for _, item := range groups[k] {
			indices = append(indices, item.idx)
		}
	}
	return indices
}

// ApplySuccessLimit groups passing results by region, sorts within each group
// (speed desc, latency asc, subscription idx asc), then optionally trims to
// success-limit. Mandatory picks (best node per region) always win; when the
// region count exceeds success-limit the total may soft-exceed the limit.
// When success-limit is 0, all nodes are kept but still region-grouped/sorted.
func ApplySuccessLimit(results []Result) []Result {
	if len(results) == 0 {
		return results
	}

	hasSpeed := config.GlobalConfig.SpeedTestUrl != ""
	limit := config.GlobalConfig.SuccessLimit
	groups, regionKeys := buildRegionGroups(results)

	if limit <= 0 {
		return flattenRegionGroups(groups, regionKeys)
	}

	mandatory := make([]indexedResult, 0, len(regionKeys))
	for _, k := range regionKeys {
		mandatory = append(mandatory, groups[k][0])
	}

	selectedIdx := make(map[int]bool, len(mandatory))
	selected := make([]indexedResult, 0, len(mandatory))

	for _, item := range mandatory {
		selected = append(selected, item)
		selectedIdx[item.idx] = true
	}

	if len(mandatory) > int(limit) {
		slog.Info(fmt.Sprintf(
			"地区数(%d)超过 success-limit(%d)，mandatory 优先，保留 %d 个节点",
			len(mandatory), limit, len(selected),
		))
		return buildRegionGroupedOutput(groups, regionKeys, selectedIdx)
	}

	var remaining []indexedResult
	for _, k := range regionKeys {
		if len(groups[k]) > 1 {
			remaining = append(remaining, groups[k][1:]...)
		}
	}
	sortIndexed(remaining, hasSpeed)

	for _, item := range remaining {
		if len(selected) >= int(limit) {
			break
		}
		if selectedIdx[item.idx] {
			continue
		}
		selected = append(selected, item)
		selectedIdx[item.idx] = true
	}

	if len(selected) < len(results) {
		slog.Info(fmt.Sprintf(
			"地区裁剪: %d → %d (%d 个地区)",
			len(results), len(selected), len(regionKeys),
		))
	}

	return buildRegionGroupedOutput(groups, regionKeys, selectedIdx)
}

func flattenRegionGroups(groups map[string][]indexedResult, regionKeys []string) []Result {
	out := make([]Result, 0)
	for _, k := range regionKeys {
		for _, item := range groups[k] {
			out = append(out, item.r)
		}
	}
	return out
}

func buildRegionGroupedOutput(groups map[string][]indexedResult, regionKeys []string, selectedIdx map[int]bool) []Result {
	out := make([]Result, 0, len(selectedIdx))
	for _, k := range regionKeys {
		for _, item := range groups[k] {
			if selectedIdx[item.idx] {
				out = append(out, item.r)
			}
		}
	}
	return out
}
