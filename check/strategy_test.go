package check

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/beihehele/subs-check/check/platform"
	"github.com/beihehele/subs-check/config"
	proxies "github.com/beihehele/subs-check/proxy"
)

func TestRequestedNameTags(t *testing.T) {
	withConfig(t, config.Config{RenameNode: true, Platforms: []string{"iprisk", "youtube", "netflix", "disney", "openai", "gemini", "claude", "spotify", "telegram"}}, func() {
		r := Result{Country: "SG", IPRisk: "0%", Youtube: "SG", Netflix: &platform.NetflixResult{Full: true, Region: "SG"}, Disney: &platform.DisneyResult{Unlocked: true}, Openai: &platform.OpenAIResult{Full: true, Region: "SG"}, Gemini: "SG", Claude: "SG", Spotify: "SG", Telegram: &platform.TelegramResult{Reachable: true}, Proxy: map[string]any{"name": "old"}}
		got := RenderName(r, false)
		if got != "🇸🇬SG|0%|YT-SG|NF-SG|GPT⁺-SG|TG" {
			t.Fatalf("name=%q", got)
		}
	})
}

func TestRegionFrozenAcrossNameMutation(t *testing.T) {
	withConfig(t, config.Config{RenameNode: true, SpeedTestUrl: "test"}, func() {
		r := []Result{{Proxy: map[string]any{"name": "HK-01"}, Speed: 100}, {Proxy: map[string]any{"name": "JP-01"}, Speed: 200}}
		AssignDisplayNames(r)
		out := SortResultsByRegion(r)
		if out[0].Region != "HK" || out[0].Proxy["name"] != "🇭🇰HK_1|100KB/s" || out[1].Proxy["name"] != "🇯🇵JP_1|200KB/s" {
			t.Fatalf("output=%+v", out)
		}
		if out[0].Country != "" || out[0].RegionSource != "name" {
			t.Fatal("inference presented as measurement")
		}
		if regionKey(Result{Country: "GB"}) != regionKey(Result{Country: "UK"}) {
			t.Fatal("GB/UK split")
		}
		if regionKey(Result{Proxy: map[string]any{"name": "unknown|NF-US"}}) != regionOther {
			t.Fatal("media region used as exit region")
		}
	})
}

func TestStableNamesIgnoreRankAndMeasurements(t *testing.T) {
	withConfig(t, config.Config{RenameNode: true, NameMode: "stable", SpeedTestUrl: "test", Platforms: []string{"telegram"}}, func() {
		p := map[string]any{"name": "old", "server": "example.com", "port": 443, "type": "trojan", "password": "secret"}
		r := []Result{{Proxy: p, Country: "SG", Speed: 100, Telegram: &platform.TelegramResult{Reachable: true}}}
		AssignDisplayNames(r)
		name := p["name"]
		r[0].Speed = 200
		r[0].Country = "JP"
		r[0].Region = "JP"
		r[0].Telegram = nil
		p["sub_tag"] = "changed"
		AssignDisplayNames(r)
		if p["name"] != name || !strings.HasPrefix(name.(string), "SC-") || strings.Contains(name.(string), "|") {
			t.Fatalf("unstable %v -> %v", name, p["name"])
		}
	})
}

func TestStructuredFilterANDAndUnknown(t *testing.T) {
	risk := 50
	withConfig(t, config.Config{NodeFilter: config.NodeFilterConfig{Regions: []string{"HK", "SG"}, RequirePlatforms: []string{"openai", "telegram"}, MaxIPRisk: &risk}}, func() {
		r := Result{Country: "HK", IPRisk: "10%", Openai: &platform.OpenAIResult{Full: true}, Telegram: &platform.TelegramResult{Reachable: true}, Proxy: map[string]any{"name": "n"}}
		if !MatchesFilter(r, nil) {
			t.Fatal("valid result rejected")
		}
		r.Country = "JP"
		if MatchesFilter(r, nil) {
			t.Fatal("OR between fields")
		}
		r.Country = "HK"
		r.Telegram = nil
		r.Proxy["sub_tag"] = "TG"
		if MatchesFilter(r, nil) {
			t.Fatal("tag spoofed detection")
		}
		r.Telegram = &platform.TelegramResult{Reachable: true}
		r.IPRisk = ""
		if MatchesFilter(r, nil) {
			t.Fatal("unknown risk passed")
		}
	})
	withConfig(t, config.Config{Filter: []string{"["}}, func() {
		if MatchesFilter(Result{Proxy: map[string]any{"name": "n"}}, CompileFilterPatterns()) {
			t.Fatal("invalid regex failed open")
		}
	})
}

func TestSubscriptionStatsBeforeSelectionAndAcrossSources(t *testing.T) {
	nodes := proxies.DeduplicateProxies([]map[string]any{
		{"server": "a", "type": "ss", "port": 1, "sub_url": "A"},
		{"server": "a", "type": "ss", "port": 1, "sub_url": "B"},
		{"server": "b", "type": "ss", "port": 1, "sub_url": "B"},
	})
	pc := &ProxyChecker{aliveChecked: []bool{true, true}, alivePassed: []bool{true, true}}
	qualified := []Result{{Proxy: nodes[0]}, {Proxy: nodes[1]}}
	pc.results = qualified[:1]
	stats := pc.subscriptionStats(nodes, qualified)
	if stats["B"].Success != 2 || stats["B"].Qualified != 2 || stats["B"].Selected != 1 || stats["A"].Success != 1 {
		t.Fatalf("stats=%+v", stats)
	}
	pc.aliveChecked = []bool{false, false}
	pc.results = nil
	if len(pc.subscriptionStats(nodes, nil)) != 0 {
		t.Fatal("policy-skipped nodes counted as dead")
	}
}

func TestSelectionModesAndCaps(t *testing.T) {
	makeResults := func() []Result {
		var out []Result
		for _, country := range []string{"HK", "JP", "US"} {
			for i := 0; i < 4; i++ {
				out = append(out, Result{Country: country, Speed: 1000 - i*10, Latency: 20, Proxy: map[string]any{"name": country, "sub_url": country}, IP: country})
			}
		}
		return out
	}
	withConfig(t, config.Config{SuccessLimit: 5, SpeedTestUrl: "test", Selection: config.SelectionConfig{Mode: "hybrid", PreferredRegions: []string{"HK", "JP"}, MinPerRegion: 2}}, func() {
		out := ApplySuccessLimit(makeResults())
		counts := map[string]int{}
		for _, r := range out {
			counts[r.Country]++
		}
		if len(out) != 5 || counts["HK"] < 2 || counts["JP"] < 2 {
			t.Fatalf("counts=%v", counts)
		}
		config.GlobalConfig.Selection.MaxPerIP = 1
		if out := ApplySuccessLimit(makeResults()); len(out) != 3 {
			t.Fatalf("IP cap returned %d", len(out))
		}
		config.GlobalConfig.Selection.MaxPerIP = 0
		config.GlobalConfig.Selection.MaxPerSource = 1
		if out := ApplySuccessLimit(makeResults()); len(out) != 3 {
			t.Fatalf("source cap returned %d", len(out))
		}
		config.GlobalConfig.Selection = config.SelectionConfig{Mode: "quality"}
		config.GlobalConfig.SuccessLimit = 1
		r := makeResults()
		r[len(r)-1].Speed = 9999
		if got := ApplySuccessLimit(r); len(got) != 1 || got[0].Speed != 9999 {
			t.Fatal("quality selection missed best")
		}
	})
}

func TestQualityHistoryTracksAliveBeforePolicy(t *testing.T) {
	old := qualityPath
	defer func() { qualityPath = old }()
	path := filepath.Join(t.TempDir(), "quality.json")
	qualityPath = func() string { return path }
	withConfig(t, config.Config{Selection: config.SelectionConfig{History: true}}, func() {
		p := map[string]any{"server": "a", "port": 1, "type": "ss"}
		nodes := []map[string]any{p}
		for range 3 {
			r := []Result{{Proxy: p, Speed: 100, Latency: 50}}
			updateQualityHistory(nodes, []bool{true}, []bool{true}, r)
			if r[0].Reliability != 1 {
				t.Fatal("reliability")
			}
		}
		r := []Result{{Proxy: p, Speed: 200, Latency: 100}}
		updateQualityHistory(nodes, []bool{true}, []bool{true}, r)
		if r[0].Observations != 4 || r[0].SmoothedSpeed != 130 || r[0].SmoothedLatency != 65 {
			t.Fatalf("record=%+v", r[0])
		}
	})
}
