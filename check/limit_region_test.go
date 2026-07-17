package check

import "testing"

func TestInferRegionFromName(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"香港 01", "HK"},
		{"🇸🇬 Singapore", "SG"},
		{"Singapore Node", "SG"},
		{"US-LA-01", "US"},
		{"SG-01", "SG"},
		{"IN-01", "IN"},
		{"INDIA-MUMBAI", "IN"},
		{"FINLAND", ""},
		{"未知节点", ""},
	}
	for _, tc := range cases {
		if got := inferRegionFromName(tc.name); got != tc.want {
			t.Fatalf("inferRegionFromName(%q) = %q, want %q", tc.name, got, tc.want)
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
