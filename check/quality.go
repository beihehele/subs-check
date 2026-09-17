package check

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/beihehele/subs-check/config"
	proxies "github.com/beihehele/subs-check/proxy"
	"github.com/beihehele/subs-check/utils"
)

type qualityRecord struct {
	Seen        int       `json:"seen"`
	Reliability float64   `json:"reliability"`
	Speed       float64   `json:"speed"`
	Latency     float64   `json:"latency"`
	Updated     time.Time `json:"updated"`
}

var qualityPath = func() string { return filepath.Join(utils.CacheDir(), "node-quality.json") }

func updateQualityHistory(nodes []map[string]any, checked, alive []bool, qualified []Result) {
	if !config.GlobalConfig.Selection.History {
		return
	}
	history := make(map[string]qualityRecord)
	data, err := os.ReadFile(qualityPath())
	if err == nil {
		if err := json.Unmarshal(data, &history); err != nil {
			slog.Warn("节点质量历史损坏，本轮重新建立", "err", err)
			history = make(map[string]qualityRecord)
		}
	} else if !os.IsNotExist(err) {
		slog.Warn("读取节点质量历史失败", "err", err)
	}
	if history == nil {
		history = make(map[string]qualityRecord)
	}
	now := time.Now()
	for id, record := range history {
		if now.Sub(record.Updated) > 30*24*time.Hour {
			delete(history, id)
		}
	}
	for i, node := range nodes {
		if i >= len(checked) || !checked[i] {
			continue
		}
		id := proxies.NodeID(node)
		if id == "" {
			continue
		}
		r := history[id]
		value := 0.0
		if alive[i] {
			value = 1
		}
		if r.Seen == 0 {
			r.Reliability = value
		} else {
			r.Reliability = 0.7*r.Reliability + 0.3*value
		}
		r.Seen++
		r.Updated = now
		history[id] = r
	}
	for i := range qualified {
		id := proxies.NodeID(qualified[i].Proxy)
		if id == "" {
			continue
		}
		r := history[id]
		if qualified[i].Speed > 0 {
			r.Speed = smooth(r.Speed, float64(qualified[i].Speed))
		}
		if qualified[i].Latency > 0 {
			r.Latency = smooth(r.Latency, float64(qualified[i].Latency))
		}
		history[id] = r
		qualified[i].Reliability = r.Reliability
		qualified[i].Observations = r.Seen
		qualified[i].SmoothedSpeed = r.Speed
		qualified[i].SmoothedLatency = r.Latency
	}
	data, err = json.Marshal(history)
	if err == nil {
		err = utils.WriteFileAtomic(qualityPath(), data)
	}
	if err != nil {
		slog.Warn("保存节点质量历史失败", "err", err)
	}
}

func smooth(old, current float64) float64 {
	if old <= 0 {
		return current
	}
	return old*0.7 + current*0.3
}

func reliabilityTier(r Result) int {
	if r.Observations < 3 {
		return 1
	} // new nodes remain eligible, with explicit exploration slots
	if r.Reliability >= 0.95 {
		return 0
	}
	if r.Reliability >= 0.8 {
		return 1
	}
	return 2
}

func qualityLess(a, b indexedResult, hasSpeed bool) bool {
	c := config.GlobalConfig.Selection
	if c.History && reliabilityTier(a.r) != reliabilityTier(b.r) {
		return reliabilityTier(a.r) < reliabilityTier(b.r)
	}
	as, bs := float64(a.r.Speed), float64(b.r.Speed)
	al, bl := float64(a.r.Latency), float64(b.r.Latency)
	if c.History {
		if a.r.SmoothedSpeed > 0 {
			as = a.r.SmoothedSpeed
		}
		if b.r.SmoothedSpeed > 0 {
			bs = b.r.SmoothedSpeed
		}
		if a.r.SmoothedLatency > 0 {
			al = a.r.SmoothedLatency
		}
		if b.r.SmoothedLatency > 0 {
			bl = b.r.SmoothedLatency
		}
	}
	if al <= 0 {
		al = 1e9
	}
	if bl <= 0 {
		bl = 1e9
	}
	if c.Sort == "latency" && al != bl {
		return al < bl
	}
	if c.Sort == "balanced" && int(al/100) != int(bl/100) {
		return int(al/100) < int(bl/100)
	}
	if hasSpeed && as != bs {
		return as > bs
	}
	if al != bl {
		return al < bl
	}
	// Legacy mode retains subscription-order ties; explicit strategies use identity.
	if c.Sort != "" || c.History {
		ai, bi := proxies.NodeID(a.r.Proxy), proxies.NodeID(b.r.Proxy)
		if ai != bi {
			return ai < bi
		}
	}
	return a.idx < b.idx
}
