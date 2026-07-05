package substats

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/beihehele/subs-check/config"
	"github.com/beihehele/subs-check/save/method"
)

const (
	statsFile = "sub-stats.json"
	deadFile  = "dead-subs.txt"
)

// CheckStat is the per-run success count for one subscription URL.
type CheckStat struct {
	Total   int
	Success int
}

// Stat persists check history for a subscription URL across runs.
type Stat struct {
	FirstCheckAt  time.Time `json:"firstCheckAt"`
	LastCheckAt   time.Time `json:"lastCheckAt"`
	LastSuccessAt time.Time `json:"lastSuccessAt,omitempty"`
	LastRecheckAt time.Time `json:"lastRecheckAt,omitempty"`
	LastSuccess   int       `json:"lastSuccess"`
	LastTotal     int       `json:"lastTotal"`
}

// Entry is a subscription row for the admin API.
type Entry struct {
	URL           string `json:"url"`
	LastSuccess   int    `json:"lastSuccess"`
	LastTotal     int    `json:"lastTotal"`
	LastCheckAt   string `json:"lastCheckAt,omitempty"`
	LastSuccessAt string `json:"lastSuccessAt,omitempty"`
	LastRecheckAt string `json:"lastRecheckAt,omitempty"`
	Status        string `json:"status"`
}

// Snapshot is returned by the admin API.
type Snapshot struct {
	DeadSubDays        int     `json:"deadSubDays"`
	DeadSubRecheckDays int     `json:"deadSubRecheckDays"`
	Entries            []Entry `json:"entries"`
}

type statsStore map[string]Stat

const (
	statusActive       = "active"
	statusDead         = "dead"
	statusRecheckDue   = "recheck_due"
	statusNeverChecked = "never_checked"
)

// IsDead reports whether url should be skipped this run.
func IsDead(url string) bool {
	entry, ok := loadEntry(url)
	if !ok {
		return false
	}
	return shouldSkip(entry, time.Now())
}

// IsRecheckDue reports whether a dead subscription is being spot-checked this run.
func IsRecheckDue(url string) bool {
	entry, ok := loadEntry(url)
	if !ok {
		return false
	}
	now := time.Now()
	deadDays := config.GlobalConfig.DeadSubDays
	if deadDays <= 0 || !isDeadSub(entry, now, deadDays) {
		return false
	}
	return !shouldSkip(entry, now)
}

// Track updates persistent stats and refreshes dead-subs.txt.
func Track(checkStats map[string]CheckStat) {
	if len(checkStats) == 0 {
		return
	}

	outputPath, ok := getOutputPath()
	if !ok {
		return
	}

	store, err := loadStats(outputPath)
	if err != nil {
		slog.Warn(fmt.Sprintf("读取订阅统计失败: %v", err))
		store = make(statsStore)
	}

	now := time.Now()
	deadDays := config.GlobalConfig.DeadSubDays
	withSuccess := 0
	rechecked := 0
	revived := 0

	for url, stats := range checkStats {
		entry := store[url]
		wasDead := deadDays > 0 && isDeadSub(entry, now, deadDays)
		if entry.FirstCheckAt.IsZero() {
			entry.FirstCheckAt = now
		}
		entry.LastCheckAt = now
		entry.LastTotal = stats.Total
		entry.LastSuccess = stats.Success
		if wasDead {
			entry.LastRecheckAt = now
			rechecked++
		}
		if stats.Success > 0 {
			entry.LastSuccessAt = now
			withSuccess++
			if wasDead {
				revived++
			}
		}
		store[url] = entry

		slog.Debug(fmt.Sprintf("订阅统计: %s", url),
			"成功", stats.Success,
			"总数", stats.Total,
		)
	}

	if err := saveStats(outputPath, store); err != nil {
		slog.Warn(fmt.Sprintf("保存订阅统计失败: %v", err))
	}

	slog.Info(fmt.Sprintf(
		"订阅检测汇总: %d 个链接, %d 个有可用节点, %d 个无可用节点",
		len(checkStats), withSuccess, len(checkStats)-withSuccess,
	))
	if rechecked > 0 {
		slog.Info(fmt.Sprintf("失效订阅抽检: %d 个", rechecked),
			"恢复可用", revived,
		)
	}

	if deadDays <= 0 {
		return
	}

	dead := collectDeadSubs(store, checkStats, now, deadDays)
	if err := writeDeadSubsFile(outputPath, dead, store, now, deadDays); err != nil {
		slog.Warn(fmt.Sprintf("写入失效订阅列表失败: %v", err))
		return
	}
	if len(dead) > 0 {
		slog.Warn(fmt.Sprintf(
			"%d 个订阅连续 %d 天无可用节点，已跳过检测并记录到 %s",
			len(dead), deadDays, filepath.Join(outputPath, deadFile),
		))
	}
}

// LoadSnapshot builds the admin/API view for all known subscription URLs.
func LoadSnapshot(urls []string) Snapshot {
	snap := Snapshot{
		DeadSubDays:        config.GlobalConfig.DeadSubDays,
		DeadSubRecheckDays: config.GlobalConfig.DeadSubRecheckDays,
		Entries:            make([]Entry, 0),
	}

	outputPath, ok := getOutputPath()
	if !ok {
		return snap
	}

	store, err := loadStats(outputPath)
	if err != nil {
		slog.Warn(fmt.Sprintf("读取订阅统计失败: %v", err))
		store = make(statsStore)
	}

	now := time.Now()
	seen := make(map[string]struct{}, len(urls))
	for _, raw := range urls {
		url := strings.TrimSpace(raw)
		if url == "" {
			continue
		}
		seen[url] = struct{}{}
		snap.Entries = append(snap.Entries, buildEntry(url, store[url], now))
	}

	for url, stat := range store {
		if _, ok := seen[url]; ok {
			continue
		}
		snap.Entries = append(snap.Entries, buildEntry(url, stat, now))
	}

	sort.Slice(snap.Entries, func(i, j int) bool {
		if snap.Entries[i].Status != snap.Entries[j].Status {
			return snap.Entries[i].Status < snap.Entries[j].Status
		}
		return snap.Entries[i].URL < snap.Entries[j].URL
	})
	return snap
}

func buildEntry(url string, stat Stat, now time.Time) Entry {
	entry := Entry{
		URL:         url,
		LastSuccess: stat.LastSuccess,
		LastTotal:   stat.LastTotal,
		Status:      statusNeverChecked,
	}
	if stat.LastCheckAt.IsZero() {
		return entry
	}
	entry.LastCheckAt = stat.LastCheckAt.Format(time.RFC3339)
	if !stat.LastSuccessAt.IsZero() {
		entry.LastSuccessAt = stat.LastSuccessAt.Format(time.RFC3339)
	}
	if !stat.LastRecheckAt.IsZero() {
		entry.LastRecheckAt = stat.LastRecheckAt.Format(time.RFC3339)
	}
	entry.Status = entryStatus(stat, now)
	return entry
}

func entryStatus(stat Stat, now time.Time) string {
	if stat.LastCheckAt.IsZero() {
		return statusNeverChecked
	}
	deadDays := config.GlobalConfig.DeadSubDays
	if deadDays <= 0 || !isDeadSub(stat, now, deadDays) {
		return statusActive
	}
	if shouldSkip(stat, now) {
		return statusDead
	}
	return statusRecheckDue
}

func shouldSkip(stat Stat, now time.Time) bool {
	deadDays := config.GlobalConfig.DeadSubDays
	if deadDays <= 0 {
		return false
	}
	if !isDeadSub(stat, now, deadDays) {
		return false
	}
	recheckDays := config.GlobalConfig.DeadSubRecheckDays
	if recheckDays <= 0 {
		return true
	}
	if stat.LastRecheckAt.IsZero() {
		return false
	}
	return now.Sub(stat.LastRecheckAt) < time.Duration(recheckDays)*24*time.Hour
}

func loadEntry(url string) (Stat, bool) {
	if url == "" {
		return Stat{}, false
	}
	outputPath, ok := getOutputPath()
	if !ok {
		return Stat{}, false
	}
	store, err := loadStats(outputPath)
	if err != nil {
		return Stat{}, false
	}
	entry, exists := store[url]
	return entry, exists
}

func isDeadSub(entry Stat, now time.Time, days int) bool {
	if days <= 0 {
		return false
	}
	ref := entry.LastSuccessAt
	if ref.IsZero() {
		ref = entry.FirstCheckAt
	}
	if ref.IsZero() {
		return false
	}
	return now.Sub(ref) >= time.Duration(days)*24*time.Hour
}

func collectDeadSubs(store statsStore, checked map[string]CheckStat, now time.Time, days int) []string {
	dead := make([]string, 0)
	for url := range checked {
		entry, ok := store[url]
		if !ok {
			continue
		}
		if isDeadSub(entry, now, days) && shouldSkip(entry, now) {
			dead = append(dead, url)
		}
	}
	sort.Strings(dead)
	return dead
}

func loadStats(outputPath string) (statsStore, error) {
	path := filepath.Join(outputPath, statsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(statsStore), nil
		}
		return nil, err
	}
	var store statsStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, err
	}
	if store == nil {
		store = make(statsStore)
	}
	return store, nil
}

func saveStats(outputPath string, store statsStore) error {
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputPath, statsFile), data, 0644)
}

func writeDeadSubsFile(outputPath string, dead []string, store statsStore, now time.Time, days int) error {
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return err
	}

	recheckDays := config.GlobalConfig.DeadSubRecheckDays
	var b strings.Builder
	fmt.Fprintf(&b, "# 以下订阅连续 %d 天无可用节点，后续检测将自动跳过\n", days)
	if recheckDays > 0 {
		fmt.Fprintf(&b, "# 每 %d 天抽检一次，恢复可用后自动重新启用\n", recheckDays)
	}
	fmt.Fprintf(&b, "# 更新时间: %s\n", now.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "# 恢复检测: 删除 output/%s 中对应条目，或等待抽检恢复\n", statsFile)
	fmt.Fprintf(&b, "# 格式: URL | 末次成功 | 本次成功/总数\n")
	for _, url := range dead {
		entry := store[url]
		lastSuccess := "从未成功"
		if !entry.LastSuccessAt.IsZero() {
			lastSuccess = entry.LastSuccessAt.Format("2006-01-02 15:04:05")
		}
		fmt.Fprintf(&b, "%s | 末次成功: %s | 本次: %d/%d\n", url, lastSuccess, entry.LastSuccess, entry.LastTotal)
	}
	return os.WriteFile(filepath.Join(outputPath, deadFile), []byte(b.String()), 0644)
}

func getOutputPath() (string, bool) {
	saver, err := method.NewLocalSaver()
	if err != nil {
		slog.Warn(fmt.Sprintf("获取输出目录失败: %v", err))
		return "", false
	}
	return saver.OutputPath, true
}
