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

func TestApplySuccessLimit_FillAfterMandatory(t *testing.T) {
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
		}
		got := ApplySuccessLimit(in)
		if len(got) != 4 {
			t.Fatalf("expected 4 results, got %d", len(got))
		}
		countries := map[string]int{}
		for _, r := range got {
			countries[r.Country]++
		}
		if countries["HK"] < 1 || countries["SG"] < 1 || countries["JP"] < 1 {
			t.Fatalf("each region should keep at least one, got %v", countries)
		}
	})
}

func TestApplySuccessLimit_MandatorySoftExceedsLimit(t *testing.T) {
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
		if len(got) != 3 {
			t.Fatalf("expected soft exceed to 3, got %d", len(got))
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
