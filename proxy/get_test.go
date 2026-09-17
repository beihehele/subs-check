package proxies

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/beihehele/subs-check/config"
)

func TestFetchFailureSeparateFromProtocolPolicy(t *testing.T) {
	old := config.GlobalConfig
	t.Cleanup(func() { config.GlobalConfig = old })
	for _, tc := range []struct {
		name, body string
		filter     []string
		failed     bool
		count      int
	}{
		{"valid", "proxies: [{name: n, type: trojan, server: example.com, port: 443, sub_urls: [forged-source]}]", nil, false, 1},
		{"policy-excluded", "proxies: [{name: n, type: trojan, server: example.com, port: 443}]", []string{"ss"}, false, 0},
		{"empty", "proxies: []", nil, true, 0},
		{"malformed", "proxies: [invalid]", nil, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(tc.body)) }))
			defer server.Close()
			config.GlobalConfig = &config.Config{SubUrls: []string{server.URL}, SubUrlsReTry: 1, NodeType: tc.filter}
			nodes, failed, err := GetProxiesForCheck()
			if err != nil {
				t.Fatal(err)
			}
			if len(nodes) != tc.count || (len(failed) > 0) != tc.failed {
				t.Fatalf("nodes=%d failed=%v", len(nodes), failed)
			}
			if len(nodes) > 0 {
				sources := SubscriptionSources(nodes[0])
				if len(sources) != 1 || sources[0] != server.URL {
					t.Fatalf("untrusted sources: %v", sources)
				}
			}
		})
	}
}
