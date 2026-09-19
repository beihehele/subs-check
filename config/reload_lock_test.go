package config

import (
	"sync"
	"testing"
	"time"
)

// A reload that is waiting for an active run must not queue a writer on the
// RWMutex: once a writer is queued, Go blocks every later RLock until that
// writer acquires the lock. A pending reload during a long check would
// therefore stall unrelated readers (status/subscription-stats handlers, or a
// test that wants its own run guard) for the whole run.
func TestPendingReloadDoesNotBlockReaders(t *testing.T) {
	original := clone(*GlobalConfig)
	t.Cleanup(func() {
		unlock := AcquireReload()
		*GlobalConfig = original
		unlock()
	})

	releaseRun := AcquireRun()
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(releaseRun) }

	applied := make(chan struct{})
	go func() {
		next := Defaults()
		next.PrintProgress = !original.PrintProgress
		Replace(next)
		close(applied)
	}()

	// Let the reload goroutine reach the waiting state.
	time.Sleep(100 * time.Millisecond)
	select {
	case <-applied:
		release()
		t.Fatal("reload applied while a run was active")
	default:
	}

	readerDone := make(chan struct{})
	go func() {
		unlock := AcquireRun()
		unlock()
		close(readerDone)
	}()
	select {
	case <-readerDone:
	case <-time.After(2 * time.Second):
		release()
		t.Fatal("reader blocked behind a pending reload")
	}

	release()
	select {
	case <-applied:
	case <-time.After(2 * time.Second):
		t.Fatal("reload did not apply after the run finished")
	}
}
