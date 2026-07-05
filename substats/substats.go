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
	LastSuccess   int       `json:"lastSuccess"`
	LastTotal     int       `json:"lastTotal"`
}

type statsStore map[string]Stat

// IsDead reports whether url has had no successful nodes for dead-sub-days.
func IsDead(url string) bool {
	days := config.GlobalConfig.DeadSubDays
	if days <= 0 || url == "" {
		return false
	}
	outputPath, ok := getOutputPath()
	if !ok {
		return false
	}
	store, err := loadStats(outputPath)
	if err != nil {
		return false
	}
	entry, exists := store[url]
	if !exists {
		return false
	}
	return isDeadSub(entry, time.Now(), days)
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
	withSuccess := 0
	for url, stats := range checkStats {
		entry := store[url]
		if entry.FirstCheckAt.IsZero() {
			entry.FirstCheckAt = now
		}
		entry.LastCheckAt = now
		entry.LastTotal = stats.Total
		entry.LastSuccess = stats.Success
		if stats.Success > 0 {
			entry.LastSuccessAt = now
			withSuccess++
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

	deadDays := config.GlobalConfig.DeadSubDays
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
		if isDeadSub(entry, now, days) {
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

	var b strings.Builder
	fmt.Fprintf(&b, "# 以下订阅连续 %d 天无可用节点，后续检测将自动跳过\n", days)
	fmt.Fprintf(&b, "# 更新时间: %s\n", now.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "# 恢复检测: 从 config 删除链接，或删除 output/%s 中对应条目后重启\n", statsFile)
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
