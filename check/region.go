package check

import (
	"strings"

	"github.com/biter777/countries"
)

func normalizeRegion(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "UK" {
		code = "GB"
	}
	if code == "" || countries.ByName(code) == countries.Unknown {
		return regionOther
	}
	return countries.ByName(code).Alpha2()
}

// ResolveRegion freezes grouping before names are mutated. Country remains
// the measured exit country; name-based fallback is explicitly marked.
func ResolveRegion(r *Result) {
	if r.Region != "" {
		return
	}
	if code := normalizeRegion(r.Country); code != regionOther {
		r.Country, r.Region, r.RegionSource = code, code, "measured"
		return
	}
	// Media tags may describe a service region, not the node's exit region.
	base := strings.SplitN(proxyDisplayName(*r), "|", 2)[0]
	r.Region = normalizeRegion(inferRegionFromName(base))
	if r.Region == regionOther {
		r.RegionSource = "unknown"
	} else {
		r.RegionSource = "name"
	}
}
