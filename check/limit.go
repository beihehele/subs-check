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

// regionKey returns the grouping key for a result. Country is preferred;
// empty Country falls into OTHER.
func regionKey(r Result) string {
	if r.Country != "" {
		return strings.ToUpper(r.Country)
	}
	return regionOther
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
	sort.Strings(regionKeys)

	for _, k := range regionKeys {
		sortIndexed(groups[k], hasSpeed)
	}

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
