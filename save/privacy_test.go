package save

import (
	"strings"
	"testing"

	"github.com/beihehele/subs-check/check"
)

func TestPublicExportOmitsPrivateSources(t *testing.T) {
	r := []check.Result{{Proxy: map[string]any{
		"name": "node", "type": "trojan", "server": "example.com", "port": 443,
		"password": "node-password", "sub_url": "private-source-token",
		"sub_urls":             []string{"private-source-token", "second-source-token"},
		"_subs_check_internal": "private-internal",
	}}}
	public, err := marshalProxies(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"sub_url", "private-source-token", "second-source-token", "private-internal"} {
		if strings.Contains(string(public), secret) {
			t.Fatalf("public export contains %q", secret)
		}
	}
	if !strings.Contains(string(public), "node-password") {
		t.Fatal("connection credentials stripped")
	}
	history, err := marshalHistoryProxies(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(history), "second-source-token") {
		t.Fatal("history lost attribution")
	}
	if r[0].Proxy["sub_url"] != "private-source-token" {
		t.Fatal("public export mutated source")
	}
}
