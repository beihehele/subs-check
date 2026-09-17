package check

import (
	"math"
	"sort"

	"github.com/beihehele/subs-check/config"
	proxies "github.com/beihehele/subs-check/proxy"
)

// selectWithPolicy applies quality/hybrid modes and optional diversity caps.
// Hard caps win over floors; exhausted groups return their capacity to others.
func selectWithPolicy(results []Result, groups map[string][]indexedResult, keys []string) []Result {
	c := config.GlobalConfig.Selection
	limit := int(config.GlobalConfig.SuccessLimit)
	if limit <= 0 || limit > len(results) {
		limit = len(results)
	}
	hasSpeed := config.GlobalConfig.SpeedTestUrl != ""
	all := make([]indexedResult, 0, len(results))
	for _, k := range keys {
		all = append(all, groups[k]...)
	}
	sortIndexed(all, hasSpeed)
	rank := make(map[int]int, len(all))
	for i, item := range all {
		rank[item.idx] = i
	}
	selected := make(map[int]bool)
	regionCount := map[string]int{}
	sourceCount := map[string]int{}
	ipCount := map[string]int{}
	add := func(item indexedResult) bool {
		if len(selected) >= limit || selected[item.idx] {
			return false
		}
		sources := proxies.SubscriptionSources(item.r.Proxy)
		if c.MaxPerSource > 0 {
			for _, s := range sources {
				if sourceCount[s] >= c.MaxPerSource {
					return false
				}
			}
		}
		if c.MaxPerIP > 0 && item.r.IP != "" && ipCount[item.r.IP] >= c.MaxPerIP {
			return false
		}
		selected[item.idx] = true
		regionCount[regionKey(item.r)]++
		for _, s := range sources {
			sourceCount[s]++
		}
		if item.r.IP != "" {
			ipCount[item.r.IP]++
		}
		return true
	}
	next := func(region string) (indexedResult, bool) {
		for _, item := range groups[region] {
			if selected[item.idx] {
				continue
			}
			blocked := c.MaxPerIP > 0 && item.r.IP != "" && ipCount[item.r.IP] >= c.MaxPerIP
			if c.MaxPerSource > 0 {
				for _, s := range proxies.SubscriptionSources(item.r.Proxy) {
					if sourceCount[s] >= c.MaxPerSource {
						blocked = true
					}
				}
			}
			if !blocked {
				return item, true
			}
		}
		return indexedResult{}, false
	}
	// Exploratory candidates must pass all current checks and caps.
	if c.History && c.ExploreSlots > 0 {
		count := 0
		for _, item := range all {
			if item.r.Observations < 3 && add(item) {
				count++
				if count >= c.ExploreSlots {
					break
				}
			}
		}
	}
	if c.Mode == "hybrid" {
		preferred := c.PreferredRegions
		if len(preferred) == 0 {
			preferred = keys
		}
		floor := c.MinPerRegion
		if floor == 0 {
			floor = 1
		}
		for round := 0; round < min(floor, limit); round++ {
			for _, raw := range preferred {
				k := normalizeRegion(raw)
				if regionCount[k] > round {
					continue
				}
				if item, ok := next(k); ok {
					add(item)
				}
			}
		}
	}
	if c.Mode == "quality" {
		for _, item := range all {
			add(item)
		}
	} else if c.Mode == "hybrid" {
		// Weighted fair allocation of remaining capacity; equal priorities use
		// the next node's quality rather than a fixed geographical order.
		for len(selected) < limit {
			best := indexedResult{}
			found := false
			bestLoad := math.Inf(1)
			for _, k := range keys {
				item, ok := next(k)
				if !ok {
					continue
				}
				weight := 1.0
				for raw, w := range c.RegionWeights {
					if normalizeRegion(raw) == k {
						weight = w
					}
				}
				// A weight is a preference, scaled by the next eligible node's
				// inverse global rank under the configured quality comparator.
				quality := 1 / float64(rank[item.idx]+1)
				load := float64(regionCount[k]+1) / (weight * quality)
				if !found || load < bestLoad || (load == bestLoad && lessResult(item, best, hasSpeed)) {
					best = item
					bestLoad = load
					found = true
				}
			}
			if !found {
				break
			}
			add(best)
		}
	} else {
		// Balanced mode with caps: each round gives every non-exhausted region
		// one slot, visiting higher-quality regions first.
		order := append([]string{}, keys...)
		sort.SliceStable(order, func(i, j int) bool { return lessResult(groups[order[i]][0], groups[order[j]][0], hasSpeed) })
		for len(selected) < limit {
			progress := false
			for _, k := range order {
				if item, ok := next(k); ok && add(item) {
					progress = true
				}
			}
			if !progress {
				break
			}
		}
	}
	return buildRegionGroupedOutput(groups, keys, selected)
}
