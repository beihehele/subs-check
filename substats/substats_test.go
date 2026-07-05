package substats

import (
	"testing"
	"time"

	"github.com/beihehele/subs-check/config"
)

func withConfig(t *testing.T, cfg config.Config, fn func()) {
	t.Helper()
	old := config.GlobalConfig
	config.GlobalConfig = &cfg
	t.Cleanup(func() { config.GlobalConfig = old })
	fn()
}

func TestIsDeadSub_NeverSucceeded(t *testing.T) {
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.Local)
	entry := Stat{
		FirstCheckAt: now.Add(-72 * time.Hour),
		LastCheckAt:  now,
	}
	if !isDeadSub(entry, now, 2) {
		t.Fatal("expected dead after 3 days without success")
	}
}

func TestShouldSkip_RecheckDue(t *testing.T) {
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.Local)
	withConfig(t, config.Config{DeadSubDays: 2, DeadSubRecheckDays: 7}, func() {
		entry := Stat{
			FirstCheckAt: now.Add(-96 * time.Hour),
			LastCheckAt:  now.Add(-24 * time.Hour),
		}
		if !isDeadSub(entry, now, 2) {
			t.Fatal("expected dead subscription")
		}
		if shouldSkip(entry, now) {
			t.Fatal("first recheck should not be skipped")
		}
	})
}

func TestShouldSkip_WaitRecheckInterval(t *testing.T) {
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.Local)
	withConfig(t, config.Config{DeadSubDays: 2, DeadSubRecheckDays: 7}, func() {
		entry := Stat{
			FirstCheckAt:  now.Add(-96 * time.Hour),
			LastCheckAt:   now.Add(-24 * time.Hour),
			LastRecheckAt: now.Add(-24 * time.Hour),
		}
		if !shouldSkip(entry, now) {
			t.Fatal("expected skip within recheck interval")
		}
		entry.LastRecheckAt = now.Add(-8 * 24 * time.Hour)
		if shouldSkip(entry, now) {
			t.Fatal("expected recheck after interval elapsed")
		}
	})
}

func TestEntryStatus(t *testing.T) {
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.Local)
	withConfig(t, config.Config{DeadSubDays: 2, DeadSubRecheckDays: 7}, func() {
		if buildEntry("https://a.example", Stat{}, now).Status != statusNeverChecked {
			t.Fatal("expected never_checked")
		}
		active := Stat{
			FirstCheckAt:  now.Add(-24 * time.Hour),
			LastCheckAt:   now,
			LastSuccessAt: now,
			LastSuccess:   1,
		}
		if buildEntry("https://b.example", active, now).Status != statusActive {
			t.Fatal("expected active")
		}
	})
}
