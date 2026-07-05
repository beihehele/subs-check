package substats

import (
	"testing"
	"time"
)

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

func TestIsDeadSub_RecentSuccess(t *testing.T) {
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.Local)
	entry := Stat{
		FirstCheckAt:  now.Add(-72 * time.Hour),
		LastSuccessAt: now.Add(-24 * time.Hour),
	}
	if isDeadSub(entry, now, 2) {
		t.Fatal("expected not dead when last success was 1 day ago")
	}
}

func TestCollectDeadSubs_OnlyCheckedURLs(t *testing.T) {
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.Local)
	store := statsStore{
		"https://a.example/sub": {FirstCheckAt: now.Add(-72 * time.Hour)},
		"https://b.example/sub": {FirstCheckAt: now.Add(-72 * time.Hour)},
	}
	checked := map[string]CheckStat{
		"https://a.example/sub": {Total: 10, Success: 0},
	}
	dead := collectDeadSubs(store, checked, now, 2)
	if len(dead) != 1 || dead[0] != "https://a.example/sub" {
		t.Fatalf("unexpected dead list: %v", dead)
	}
}
