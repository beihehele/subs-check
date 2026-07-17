package check

import (
	"testing"

	"github.com/beihehele/subs-check/config"
)

func resultWith(country string, speed, latency, idx int) Result {
	return Result{
		Proxy:   map[string]any{"name": idx},
		Country: country,
		Speed:   speed,
		Latency: latency,
	}
}

func TestSortResultsByRegion_ContiguousBlocks(t *testing.T) {
	in := []Result{
		resultWith("JP", 100, 0, 0),
		resultWith("HK", 200, 0, 1),
		resultWith("US", 300, 0, 2),
		resultWith("HK", 150, 0, 3),
	}
	got := SortResultsByRegion(in)
	blockKeys := make([]string, 0)
	for _, r := range got {
		k := regionKey(r)
		if len(blockKeys) == 0 || blockKeys[len(blockKeys)-1] != k {
			blockKeys = append(blockKeys, k)
		}
	}
	for i := 1; i < len(blockKeys); i++ {
		if compareRegionKeys(blockKeys[i], blockKeys[i-1]) < 0 {
			t.Fatalf("region blocks interleaved: %v", blockKeys)
		}
	}
	if blockKeys[0] != "HK" {
		t.Fatalf("expected HK first, got %v", blockKeys)
	}
}

func TestSortResultsByRegion_OtherLast(t *testing.T) {
	in := []Result{
		{Proxy: map[string]any{"name": "unknown-1"}},
		resultWith("HK", 100, 0, 1),
	}
	got := SortResultsByRegion(in)
	if regionKey(got[len(got)-1]) != regionOther {
		t.Fatalf("expected OTHER last, got %q", regionKey(got[len(got)-1]))
	}
}

func TestApplySuccessLimit_ZeroKeepsAllGrouped(t *testing.T) {
	withConfig(t, config.Config{
		SpeedTestUrl: "http://example.invalid/dl",
		SuccessLimit: 0,
	}, func() {
		in := []Result{
			resultWith("JP", 100, 50, 0),
			resultWith("HK", 200, 30, 1),
			resultWith("HK", 150, 20, 2),
		}
		got := ApplySuccessLimit(in)
		if len(got) != 3 {
			t.Fatalf("expected 3 results, got %d", len(got))
		}
		if got[0].Country != "HK" || got[0].Speed != 200 {
			t.Fatalf("expected HK fastest first, got %+v", got[0])
		}
		if got[1].Country != "HK" || got[1].Speed != 150 {
			t.Fatalf("expected HK second node, got %+v", got[1])
		}
		if got[2].Country != "JP" {
			t.Fatalf("expected JP group last, got %+v", got[2])
		}
	})
}

func TestApplySuccessLimit_EvenDistribute(t *testing.T) {
	withConfig(t, config.Config{
		SpeedTestUrl: "http://example.invalid/dl",
		SuccessLimit: 4,
	}, func() {
		in := []Result{
			resultWith("HK", 300, 10, 0),
			resultWith("HK", 200, 10, 1),
			resultWith("HK", 100, 10, 2),
			resultWith("SG", 250, 10, 3),
			resultWith("SG", 100, 10, 4),
			resultWith("JP", 400, 10, 5),
		}
		got := ApplySuccessLimit(in)
		if len(got) != 4 {
			t.Fatalf("expected 4 results, got %d", len(got))
		}
		countries := map[string]int{}
		for _, r := range got {
			countries[r.Country]++
		}
		// quality order JP>HK>SG; base=1 rem=1 → JP would get 2 but only has 1,
		// unused slot reclaimed to next best with capacity → HK=2, SG=1, JP=1.
		if countries["HK"] != 2 || countries["SG"] != 1 || countries["JP"] != 1 {
			t.Fatalf("expected HK:2 SG:1 JP:1, got %v", countries)
		}
		// Within-region picks must be the fastest nodes.
		hkSpeeds := []int{}
		for _, r := range got {
			if r.Country == "HK" {
				hkSpeeds = append(hkSpeeds, r.Speed)
			}
		}
		if hkSpeeds[0] != 300 || hkSpeeds[1] != 200 {
			t.Fatalf("expected HK 300 then 200, got %v", hkSpeeds)
		}
	})
}

func TestApplySuccessLimit_EvenDistributeRemToBest(t *testing.T) {
	withConfig(t, config.Config{
		SpeedTestUrl: "http://example.invalid/dl",
		SuccessLimit: 4,
	}, func() {
		in := []Result{
			resultWith("HK", 300, 10, 0),
			resultWith("HK", 200, 10, 1),
			resultWith("SG", 250, 10, 2),
			resultWith("SG", 100, 10, 3),
			resultWith("JP", 400, 10, 4),
			resultWith("JP", 350, 10, 5),
		}
		got := ApplySuccessLimit(in)
		if len(got) != 4 {
			t.Fatalf("expected 4 results, got %d", len(got))
		}
		countries := map[string]int{}
		for _, r := range got {
			countries[r.Country]++
		}
		// quality JP>HK>SG; base=1 rem=1 → JP=2, HK=1, SG=1.
		if countries["JP"] != 2 || countries["HK"] != 1 || countries["SG"] != 1 {
			t.Fatalf("expected JP:2 HK:1 SG:1, got %v", countries)
		}
		jpSpeeds := []int{}
		for _, r := range got {
			if r.Country == "JP" {
				jpSpeeds = append(jpSpeeds, r.Speed)
			}
		}
		if jpSpeeds[0] != 400 || jpSpeeds[1] != 350 {
			t.Fatalf("expected JP 400 then 350, got %v", jpSpeeds)
		}
	})
}

func TestApplySuccessLimit_HardCapKeepBestRegions(t *testing.T) {
	withConfig(t, config.Config{
		SpeedTestUrl: "http://example.invalid/dl",
		SuccessLimit: 2,
	}, func() {
		in := []Result{
			resultWith("HK", 100, 10, 0),
			resultWith("SG", 200, 10, 1),
			resultWith("JP", 300, 10, 2),
		}
		got := ApplySuccessLimit(in)
		if len(got) != 2 {
			t.Fatalf("expected hard cap to 2, got %d", len(got))
		}
		countries := map[string]int{}
		for _, r := range got {
			countries[r.Country]++
		}
		if countries["JP"] != 1 || countries["SG"] != 1 || countries["HK"] != 0 {
			t.Fatalf("expected JP+SG (best regions), got %v", countries)
		}
	})
}

func TestApplySuccessLimit_HardCapOneNodePerKeptRegion(t *testing.T) {
	withConfig(t, config.Config{
		SpeedTestUrl: "http://example.invalid/dl",
		SuccessLimit: 2,
	}, func() {
		in := []Result{
			resultWith("HK", 100, 10, 0),
			resultWith("HK", 90, 10, 1),
			resultWith("SG", 200, 10, 2),
			resultWith("SG", 190, 10, 3),
			resultWith("JP", 300, 10, 4),
			resultWith("JP", 290, 10, 5),
		}
		got := ApplySuccessLimit(in)
		if len(got) != 2 {
			t.Fatalf("expected hard cap to 2, got %d", len(got))
		}
		countries := map[string]int{}
		speeds := map[string]int{}
		for _, r := range got {
			countries[r.Country]++
			speeds[r.Country] = r.Speed
		}
		if countries["JP"] != 1 || countries["SG"] != 1 || countries["HK"] != 0 {
			t.Fatalf("expected one node each from JP+SG, got %v", countries)
		}
		if speeds["JP"] != 300 || speeds["SG"] != 200 {
			t.Fatalf("expected best node per kept region, got speeds %v", speeds)
		}
	})
}

func TestApplySuccessLimit_OutputRegionPriorityOrder(t *testing.T) {
	withConfig(t, config.Config{
		SpeedTestUrl: "http://example.invalid/dl",
		SuccessLimit: 2,
	}, func() {
		in := []Result{
			resultWith("HK", 100, 10, 0),
			resultWith("SG", 200, 10, 1),
			resultWith("JP", 300, 10, 2),
		}
		got := ApplySuccessLimit(in)
		// Selection by quality keeps JP+SG, but output follows region priority: SG then JP.
		if len(got) != 2 || got[0].Country != "SG" || got[1].Country != "JP" {
			t.Fatalf("expected output order SG then JP, got %+v", got)
		}
	})
}

func TestApplySuccessLimit_SortByLatencyWithoutSpeed(t *testing.T) {
	withConfig(t, config.Config{
		SpeedTestUrl: "",
		SuccessLimit: 0,
	}, func() {
		in := []Result{
			resultWith("HK", 0, 80, 0),
			resultWith("HK", 0, 20, 1),
		}
		got := ApplySuccessLimit(in)
		if len(got) != 2 || got[0].Latency != 20 {
			t.Fatalf("expected lower latency first, got %+v", got)
		}
	})
}
