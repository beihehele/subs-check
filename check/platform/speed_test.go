package platform

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/beihehele/subs-check/config"
)

func TestSpeedRejectsBadResponses(t *testing.T) {
	old := config.GlobalConfig
	config.GlobalConfig = &config.Config{DownloadTimeout: 2, SpeedMinSampleKB: 64}
	t.Cleanup(func() { config.GlobalConfig = old })
	for _, tc := range []struct {
		name              string
		status, size      int
		truncated, wantOK bool
	}{
		{"ok", 200, 128 * 1024, false, true},
		{"partial", 206, 128 * 1024, false, true},
		{"error-page", 503, 128 * 1024, false, false},
		{"empty", 200, 0, false, false},
		{"tiny", 200, 1024, false, false},
		{"truncated", 200, 128 * 1024, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.truncated {
					w.Header().Set("Content-Length", "262144")
				}
				w.WriteHeader(tc.status)
				w.Write(make([]byte, tc.size))
			}))
			defer server.Close()
			speed, size, err := CheckSpeed(server.Client(), nil, nil, server.URL)
			if (err == nil) != tc.wantOK {
				t.Fatalf("speed=%d size=%d err=%v", speed, size, err)
			}
			if tc.wantOK && (speed <= 0 || size != int64(tc.size)) {
				t.Fatalf("invalid measurement: %d %d", speed, size)
			}
		})
	}
}
