package proxies

import "testing"

func TestDedupCompleteConnectionAndSources(t *testing.T) {
	base := map[string]any{"type": "vless", "server": "EXAMPLE.com", "port": 443, "uuid": "secret", "network": "ws", "ws-opts": map[string]any{"path": "/a", "headers": map[string]any{"Host": "h"}}}
	clone := func() map[string]any {
		m := map[string]any{}
		for k, v := range base {
			m[k] = v
		}
		return m
	}
	a, b, c, d := clone(), clone(), clone(), clone()
	a["name"], a["sub_url"] = "first", "source-a"
	b["name"], b["sub_url"], b["server"], b["port"] = "alias", "source-b", "example.com", "443"
	c["ws-opts"] = map[string]any{"path": "/b", "headers": map[string]any{"Host": "h"}}
	d["type"] = "vmess"
	got := DeduplicateProxies([]map[string]any{a, b, c, d})
	if len(got) != 3 {
		t.Fatalf("distinct connections collapsed: %d", len(got))
	}
	if sources := SubscriptionSources(got[0]); len(sources) != 2 || sources[0] != "source-a" || sources[1] != "source-b" {
		t.Fatalf("sources=%v", sources)
	}
	if NodeID(a) != NodeID(b) || NodeID(a) == NodeID(c) || NodeID(a) == NodeID(d) {
		t.Fatal("incorrect identity")
	}
	if _, ok := a["sub_urls"]; ok {
		t.Fatal("dedup mutated caller metadata")
	}
	public := PublicProxy(got[0])
	if _, ok := public["sub_urls"]; ok {
		t.Fatal("private source URLs leaked")
	}
	if _, ok := public["sub_url"]; ok {
		t.Fatal("private source URL leaked")
	}
}
