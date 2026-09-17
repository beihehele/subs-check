package check

import (
	"fmt"
	"log/slog"

	"github.com/beihehele/subs-check/config"
	proxies "github.com/beihehele/subs-check/proxy"
	"github.com/beihehele/subs-check/substats"
)

func (pc *ProxyChecker) subscriptionStats(all []map[string]any, qualified []Result) map[string]substats.CheckStat {
	stats := make(map[string]substats.CheckStat)
	// Only actual fetch failures/empty feeds count as zero; policy-filtered
	// nodes and cancelled runs must not make a healthy subscription expire.
	for _, url := range pc.subscriptionURLs {
		stats[url] = substats.CheckStat{}
	}
	for i, node := range all {
		if i >= len(pc.aliveChecked) || !pc.aliveChecked[i] {
			continue
		}
		for _, url := range proxies.SubscriptionSources(node) {
			s := stats[url]
			s.Total++
			if pc.alivePassed[i] {
				s.Success++
			}
			stats[url] = s
		}
	}
	for _, stage := range []struct {
		results  []Result
		selected bool
	}{{qualified, false}, {pc.results, true}} {
		for _, r := range stage.results {
			for _, url := range proxies.SubscriptionSources(r.Proxy) {
				s := stats[url]
				if stage.selected {
					s.Selected++
				} else {
					s.Qualified++
				}
				stats[url] = s
			}
		}
	}
	return stats
}

func (pc *ProxyChecker) trackSubscriptions(all []map[string]any, qualified []Result) {
	stats := pc.subscriptionStats(all, qualified)
	substats.Track(stats)
	for url, s := range stats {
		if s.Total > 0 && float32(s.Success)/float32(s.Total) < config.GlobalConfig.SuccessRate {
			slog.Warn(fmt.Sprintf("订阅存活率过低: %s", url), "检测", s.Total, "存活", s.Success, "达标", s.Qualified, "入选", s.Selected)
		}
	}
}
