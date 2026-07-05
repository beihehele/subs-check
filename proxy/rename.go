package proxies

import (
	"strconv"
	"strings"

	"github.com/biter777/countries"
)

// FormatRename builds the country base segment for node names.
// seq<=0 omits the _N suffix (filter / preview); seq>0 appends _1, _2, ...
func FormatRename(countryCode string, seq int) string {
	flag := CountryCodeToFlag(countryCode)
	code := strings.ToUpper(strings.TrimSpace(countryCode))
	if countries.ByName(code) == countries.Unknown {
		if seq <= 0 {
			return flag
		}
		return flag + "_" + strconv.Itoa(seq)
	}
	if seq <= 0 {
		return flag + code
	}
	return flag + code + "_" + strconv.Itoa(seq)
}

// RenameSeqKey returns the per-country counter bucket used when assigning _N suffixes.
func RenameSeqKey(countryCode string) string {
	code := strings.ToUpper(strings.TrimSpace(countryCode))
	if code == "" {
		return ""
	}
	if countries.ByName(code) == countries.Unknown {
		return ""
	}
	return code
}

// Rename is kept for compatibility; prefer FormatRename with an explicit sequence.
func Rename(name string) string {
	return FormatRename(name, 1)
}

// ResetRenameCounter is a no-op; numbering is assigned at save time.
func ResetRenameCounter() {}

func CountryCodeToFlag(countryCode string) string {
	code := strings.ToUpper(countryCode)
	country := countries.ByName(code)
	if country == countries.Unknown {
		return "❓Other"
	}
	return country.Emoji()
}
