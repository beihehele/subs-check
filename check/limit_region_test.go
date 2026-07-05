package check

import "testing"

func TestInferRegionFromName(t *testing.T) {
	cases := map[string]string{
		"香港 01":     "HK",
		"🇸🇬 Singapore": "SG",
		"US-LA-01":  "US",
		"未知节点":      "",
	}
	for name, want := range cases {
		if got := inferRegionFromName(name); got != want {
			t.Fatalf("inferRegionFromName(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestRegionKeyFromNameFallback(t *testing.T) {
	r := Result{
		Proxy: map[string]any{"name": "台湾高速"},
	}
	if regionKey(r) != "TW" {
		t.Fatalf("expected TW from name fallback, got %q", regionKey(r))
	}
}
