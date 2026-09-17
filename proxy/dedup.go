package proxies

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// NodeID hashes the complete connection configuration, excluding display and
// subscription metadata. JSON sorts keys, including nested transport options.
// Unknown connection fields are retained to avoid merging distinct connections.
func NodeID(proxy map[string]any) string {
	identity := PublicProxy(proxy)
	delete(identity, "name")
	delete(identity, "sub_tag")
	for _, field := range []string{"server", "type"} {
		if value, ok := identity[field].(string); ok {
			identity[field] = strings.ToLower(strings.TrimSpace(value))
		}
	}
	if identity["type"] == "hy2" {
		identity["type"] = "hysteria2"
	}
	if port, ok := identity["port"]; ok {
		identity["port"] = fmt.Sprint(port)
	}
	data, err := json.Marshal(identity)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// PublicProxy copies a node without private subscription URLs or internal metadata.
func PublicProxy(proxy map[string]any) map[string]any {
	out := make(map[string]any, len(proxy))
	for key, value := range proxy {
		if key != "sub_url" && key != "sub_urls" && !strings.HasPrefix(key, "_subs_check_") {
			out[key] = value
		}
	}
	return out
}

func SubscriptionSources(proxy map[string]any) []string {
	seen := make(map[string]bool)
	if url, ok := proxy["sub_url"].(string); ok && url != "" {
		seen[url] = true
	}
	switch urls := proxy["sub_urls"].(type) {
	case []string:
		for _, url := range urls {
			if url != "" {
				seen[url] = true
			}
		}
	case []any:
		for _, value := range urls {
			if url, ok := value.(string); ok && url != "" {
				seen[url] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for url := range seen {
		out = append(out, url)
	}
	sort.Strings(out)
	return out
}

func DeduplicateProxies(proxies []map[string]any) []map[string]any {
	seen := make(map[string]int)
	result := make([]map[string]any, 0, len(proxies))
	for _, proxy := range proxies {
		server, _ := proxy["server"].(string)
		if strings.TrimSpace(server) == "" {
			continue
		}
		id := NodeID(proxy)
		if idx, ok := seen[id]; ok && id != "" {
			merged := map[string]any{"sub_urls": append(SubscriptionSources(result[idx]), SubscriptionSources(proxy)...)}
			result[idx]["sub_urls"] = SubscriptionSources(merged)
			continue
		}
		copy := make(map[string]any, len(proxy)+1)
		for k, v := range proxy {
			copy[k] = v
		}
		copy["sub_urls"] = SubscriptionSources(proxy)
		if id != "" {
			seen[id] = len(result)
		}
		result = append(result, copy)
	}
	return result
}
