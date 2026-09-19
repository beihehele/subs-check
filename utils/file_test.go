package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSetCacheDir_FollowsConfigFile(t *testing.T) {
	old := cacheDir
	t.Cleanup(func() { cacheDir = old })

	dir := t.TempDir()
	SetCacheDir(filepath.Join(dir, "config.yaml"))
	if got, want := CacheDir(), filepath.Join(dir, "cache", "config"); got != want {
		t.Fatalf("CacheDir() = %q, want %q", got, want)
	}

	// Config files sharing a dir get separate caches.
	SetCacheDir(filepath.Join(dir, "other.yml"))
	if got, want := CacheDir(), filepath.Join(dir, "cache", "other"); got != want {
		t.Fatalf("CacheDir() = %q, want %q", got, want)
	}
}

func TestWriteFileAtomic_CreatesDirAndReplaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "data.json")

	if err := WriteFileAtomic(path, []byte("one")); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := WriteFileAtomic(path, []byte("two")); err != nil {
		t.Fatalf("second write: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil || string(got) != "two" {
		t.Fatalf("read = %q, %v; want two", got, err)
	}

	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("leftover temp files: %v", entries)
	}

	if runtime.GOOS != "windows" {
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := fi.Mode().Perm(); perm != 0o600 {
			t.Errorf("perm = %o, want 600", perm)
		}
	}
}

func TestWriteFileAtomicMode_UsesRequestedMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix permission bits")
	}
	path := filepath.Join(t.TempDir(), "data.txt")
	if err := WriteFileAtomicMode(path, []byte("data"), 0o640); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o640 {
		t.Fatalf("perm = %o, want 640", got)
	}
}
