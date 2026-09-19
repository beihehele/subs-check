package config

import (
	_ "embed"
	"sync"
	"time"
)

type Config struct {
	PrintProgress        bool             `yaml:"print-progress"`
	IPv6                 bool             `yaml:"ipv6"`
	Concurrent           int              `yaml:"concurrent"`
	SpeedConcurrent      int              `yaml:"speed-concurrent"`
	MediaConcurrent      int              `yaml:"media-concurrent"`
	ShuffleTestOrder     bool             `yaml:"shuffle-test-order"`
	CheckInterval        int              `yaml:"check-interval"`
	CronExpression       string           `yaml:"cron-expression"`
	AliveTestUrl         string           `yaml:"alive-test-url"`
	SpeedTestUrl         string           `yaml:"speed-test-url"`
	DownloadTimeout      int              `yaml:"download-timeout"`
	DownloadMB           int              `yaml:"download-mb"`
	TotalSpeedLimit      int              `yaml:"total-speed-limit"`
	MinSpeed             int              `yaml:"min-speed"`
	Timeout              int              `yaml:"timeout"`
	MediaCheckTimeout    int              `yaml:"media-check-timeout"`
	FilterRegex          string           `yaml:"filter-regex"`
	SaveMethod           string           `yaml:"save-method"`
	WebDAVURL            string           `yaml:"webdav-url"`
	WebDAVUsername       string           `yaml:"webdav-username"`
	WebDAVPassword       string           `yaml:"webdav-password"`
	GithubToken          string           `yaml:"github-token"`
	GithubGistID         string           `yaml:"github-gist-id"`
	GithubAPIMirror      string           `yaml:"github-api-mirror"`
	WorkerURL            string           `yaml:"worker-url"`
	WorkerToken          string           `yaml:"worker-token"`
	S3Endpoint           string           `yaml:"s3-endpoint"`
	S3AccessID           string           `yaml:"s3-access-id"`
	S3SecretKey          string           `yaml:"s3-secret-key"`
	S3Bucket             string           `yaml:"s3-bucket"`
	S3UseSSL             bool             `yaml:"s3-use-ssl"`
	S3BucketLookup       string           `yaml:"s3-bucket-lookup"`
	SubUrlsReTry         int              `yaml:"sub-urls-retry"`
	SubUrlsRetryInterval int              `yaml:"sub-urls-retry-interval"`
	SubUrlsTimeout       int              `yaml:"sub-urls-timeout"`
	SubUrlsConcurrent    int              `yaml:"sub-urls-concurrent"`
	SubUrlsGetUA         string           `yaml:"sub-urls-get-ua"`
	SubUrlsRemote        []string         `yaml:"sub-urls-remote"`
	SubUrls              []string         `yaml:"sub-urls"`
	SuccessRate          float32          `yaml:"success-rate"`
	DeadSubDays          int              `yaml:"dead-sub-days"`
	DeadSubRecheckDays   int              `yaml:"dead-sub-recheck-days"`
	MihomoApiUrl         string           `yaml:"mihomo-api-url"`
	MihomoApiSecret      string           `yaml:"mihomo-api-secret"`
	ListenPort           string           `yaml:"listen-port"`
	RenameNode           bool             `yaml:"rename-node"`
	OutputDir            string           `yaml:"output-dir"`
	AppriseApiServer     string           `yaml:"apprise-api-server"`
	RecipientUrl         []string         `yaml:"recipient-url"`
	NotifyTitle          string           `yaml:"notify-title"`
	SubStorePort         string           `yaml:"sub-store-port"`
	SubStorePath         string           `yaml:"sub-store-path"`
	SubStoreSyncCron     string           `yaml:"sub-store-sync-cron"`
	SubStorePushService  string           `yaml:"sub-store-push-service"`
	SubStoreProduceCron  string           `yaml:"sub-store-produce-cron"`
	MihomoOverwriteUrl   string           `yaml:"mihomo-overwrite-url"`
	MediaCheck           bool             `yaml:"media-check"`
	Platforms            []string         `yaml:"platforms"`
	SuccessLimit         int32            `yaml:"success-limit"`
	NodePrefix           string           `yaml:"node-prefix"`
	NodeType             []string         `yaml:"node-type"`
	EnableWebUI          bool             `yaml:"enable-web-ui"`
	WebBasePath          string           `yaml:"web-base-path"`
	APIKey               string           `yaml:"api-key"`
	GithubProxy          string           `yaml:"github-proxy"`
	Proxy                string           `yaml:"proxy"`
	CallbackScript       string           `yaml:"callback-script"`
	Filter               []string         `yaml:"filter"`
	NodeFilter           NodeFilterConfig `yaml:"node-filter"`
	Selection            SelectionConfig  `yaml:"selection"`
	NameMode             string           `yaml:"name-mode"`
	SpeedMinSampleKB     int              `yaml:"speed-min-sample-kb"`
	SpeedRetest          bool             `yaml:"speed-retest"`
	KeepDays             int              `yaml:"keep-days"`
	DNS                  DNSConfig        `yaml:"dns"`
}

type NodeFilterConfig struct {
	Regions          []string `yaml:"regions"`
	Protocols        []string `yaml:"protocols"`
	MaxLatency       int      `yaml:"max-latency"`
	MaxIPRisk        *int     `yaml:"max-ip-risk"`
	RequirePlatforms []string `yaml:"require-platforms"`
	NameInclude      string   `yaml:"name-include"`
	NameExclude      string   `yaml:"name-exclude"`
}

type SelectionConfig struct {
	Mode             string             `yaml:"mode"` // balanced, quality, hybrid
	Sort             string             `yaml:"sort"` // speed, latency, balanced
	PreferredRegions []string           `yaml:"preferred-regions"`
	MinPerRegion     int                `yaml:"min-per-region"`
	RegionWeights    map[string]float64 `yaml:"region-weights"`
	MaxPerSource     int                `yaml:"max-per-source"`
	MaxPerIP         int                `yaml:"max-per-ip"`
	History          bool               `yaml:"history"`
	ExploreSlots     int                `yaml:"explore-slots"`
}

// DNSConfig selects the resolver used by every proxy probe.
// Leaving Enable=false uses mihomo's SystemResolver.
type DNSConfig struct {
	Enable                bool     `yaml:"enable"`
	Nameserver            []string `yaml:"nameserver"`
	ProxyServerNameserver []string `yaml:"proxy-server-nameserver"`
	DefaultNameserver     []string `yaml:"default-nameserver"`
}

var GlobalConfig = Defaults()

// configMu serializes runtime configuration replacement with a check run.
// Callers that only need to inspect configuration while a reload may be in
// progress should use Snapshot. The run/reload guards are intentionally kept
// here so packages do not need to coordinate on the mutable GlobalConfig
// pointer themselves.
var configMu sync.RWMutex

// Reload acquisition polls instead of blocking on Lock. sync.RWMutex gives
// queued writers priority over new readers, so a reload that blocks in Lock
// while a long check run holds the read side would also stall every later
// RLock until the run finished. TryLock never queues a writer, so readers keep
// making progress while the reload waits its turn.
const (
	reloadPollMin = 5 * time.Millisecond
	reloadPollMax = 500 * time.Millisecond
)

// AcquireRun prevents a hot reload from replacing GlobalConfig while a check
// pipeline is reading it. The returned function must be deferred by the
// caller.
func AcquireRun() func() {
	configMu.RLock()
	return configMu.RUnlock
}

// AcquireReload serializes a configuration replacement with active checks.
// The returned function must be called after the replacement and any dependent
// runtime state (for example DNS) has been initialized or rolled back.
//
// It must not be called by a goroutine that already holds AcquireRun.
func AcquireReload() func() {
	delay := reloadPollMin
	for !configMu.TryLock() {
		// A run is in flight. Yield and retry; because TryLock does not queue a
		// writer, readers are never blocked behind this pending reload.
		time.Sleep(delay)
		if delay < reloadPollMax {
			delay *= 2
		}
	}
	return configMu.Unlock
}

// Replace publishes a validated configuration for future operations.
func Replace(next *Config) {
	if next == nil {
		return
	}
	unlock := AcquireReload()
	defer unlock()
	ReplaceLocked(next)
}

// ReplaceLocked replaces the configuration while the caller owns the reload
// lock. It is used when applying a config and initializing dependent globals
// must be one transaction.
func ReplaceLocked(next *Config) {
	if next != nil {
		*GlobalConfig = *next
	}
}

// Snapshot returns a detached copy suitable for short-lived read-only work.
// Slices and maps are copied so callers cannot mutate the live configuration
// through a shared backing array.
func Snapshot() Config {
	configMu.RLock()
	defer configMu.RUnlock()
	return clone(*GlobalConfig)
}

func clone(src Config) Config {
	dst := src
	dst.SubUrls = append([]string(nil), src.SubUrls...)
	dst.SubUrlsRemote = append([]string(nil), src.SubUrlsRemote...)
	dst.RecipientUrl = append([]string(nil), src.RecipientUrl...)
	dst.Platforms = append([]string(nil), src.Platforms...)
	dst.NodeType = append([]string(nil), src.NodeType...)
	dst.Filter = append([]string(nil), src.Filter...)
	dst.NodeFilter.Regions = append([]string(nil), src.NodeFilter.Regions...)
	dst.NodeFilter.Protocols = append([]string(nil), src.NodeFilter.Protocols...)
	dst.NodeFilter.RequirePlatforms = append([]string(nil), src.NodeFilter.RequirePlatforms...)
	dst.Selection.PreferredRegions = append([]string(nil), src.Selection.PreferredRegions...)
	if src.Selection.RegionWeights != nil {
		dst.Selection.RegionWeights = make(map[string]float64, len(src.Selection.RegionWeights))
		for k, v := range src.Selection.RegionWeights {
			dst.Selection.RegionWeights[k] = v
		}
	}
	dst.DNS.Nameserver = append([]string(nil), src.DNS.Nameserver...)
	dst.DNS.ProxyServerNameserver = append([]string(nil), src.DNS.ProxyServerNameserver...)
	dst.DNS.DefaultNameserver = append([]string(nil), src.DNS.DefaultNameserver...)
	return dst
}

// Defaults returns independent defaults for a complete config load.
// Defaults returns neutral fallbacks for existing configs: fields they never
// set keep the old behavior. New users should copy config.example.yaml, which
// enables the recommended feature values.
func Defaults() *Config {
	return &Config{
		IPv6:               true,
		ListenPort:         ":8199",
		NotifyTitle:        "🔔 节点状态更新",
		MihomoOverwriteUrl: "http://127.0.0.1:8199/sub/ACL4SSR_Online_Full.yaml",
		MediaCheckTimeout:  10,
		Platforms:          []string{"iprisk", "youtube", "netflix", "openai", "telegram"},
		DownloadMB:         20,
		AliveTestUrl:       "http://gstatic.com/generate_204",
		SubUrlsGetUA:       "clash.meta (https://github.com/beihehele/subs-check)",
		SubUrlsReTry:       3,
		SubUrlsConcurrent:  20,
		DeadSubDays:        0,
		DeadSubRecheckDays: 0,
	}
}

//go:embed config.example.yaml
var DefaultConfigTemplate []byte

var GlobalProxies []map[string]any
