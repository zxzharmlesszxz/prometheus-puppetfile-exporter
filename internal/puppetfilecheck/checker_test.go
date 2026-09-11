package puppetfilecheck

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckerSnapshotsPuppetfile(t *testing.T) {
	t.Parallel()

	path := writeTempPuppetfile(t, "mod 'puppetlabs/apt', '11.2.0'\n")
	now := time.Unix(1_700_000_000, 0)
	snapshot := NewChecker(path).Snapshot(context.Background(), now)

	if !snapshot.Success {
		t.Fatalf("Snapshot().Success = false, want true: %v", snapshot.Err)
	}
	if !snapshot.AttemptTime.Equal(now) {
		t.Fatalf("Snapshot().AttemptTime = %v, want %v", snapshot.AttemptTime, now)
	}
	if snapshot.Path != path {
		t.Fatalf("Snapshot().Path = %q, want %q", snapshot.Path, path)
	}
	if snapshot.ReadErrorsTotal != 0 {
		t.Fatalf("Snapshot().ReadErrorsTotal = %d, want 0", snapshot.ReadErrorsTotal)
	}
	if got := snapshot.Modules["puppetlabs/apt"]; got.Version != "11.2.0" || got.Type != ModuleTypeForge {
		t.Fatalf("unexpected checker module: %#v", got)
	}
	if !snapshot.Files[0].LastKnownGood.Available || snapshot.Files[0].LastKnownGood.ConsecutiveFailures != 0 {
		t.Fatalf("LastKnownGood = %#v, want available without failures", snapshot.Files[0].LastKnownGood)
	}
}

func TestCheckerReportsReadErrors(t *testing.T) {
	t.Parallel()

	checker := NewChecker(filepath.Join(t.TempDir(), "missing-Puppetfile"))
	first := checker.Snapshot(context.Background(), time.Unix(1_700_000_000, 0))
	second := checker.Snapshot(context.Background(), time.Unix(1_700_000_001, 0))

	if first.Success || first.Err == nil {
		t.Fatalf("first Snapshot() success/error = %v/%v, want failed read", first.Success, first.Err)
	}
	if second.Success || second.Err == nil {
		t.Fatalf("second Snapshot() success/error = %v/%v, want failed read", second.Success, second.Err)
	}
	if first.ReadErrorsTotal != 1 || second.ReadErrorsTotal != 2 {
		t.Fatalf("Snapshot().ReadErrorsTotal = %d/%d, want 1/2", first.ReadErrorsTotal, second.ReadErrorsTotal)
	}
	if first.Files[0].LastKnownGood.Available || first.Files[0].LastKnownGood.ConsecutiveFailures != 1 {
		t.Fatalf("first LastKnownGood = %#v, want unavailable with one failure", first.Files[0].LastKnownGood)
	}
	if second.Files[0].LastKnownGood.Available || second.Files[0].LastKnownGood.ConsecutiveFailures != 2 {
		t.Fatalf("second LastKnownGood = %#v, want unavailable with two failures", second.Files[0].LastKnownGood)
	}
}

func TestCheckerRetainsAndStalesLastKnownGood(t *testing.T) {
	t.Parallel()

	path := writeTempPuppetfile(t, "mod 'puppetlabs/apt', '11.2.0'\n")
	checker := NewChecker(path)
	now := time.Unix(1_700_000_000, 0)
	first := checker.Snapshot(context.Background(), now)

	if err := os.WriteFile(path, []byte("mod 'puppetlabs/apt'\n"), 0o644); err != nil {
		t.Fatalf("write invalid Puppetfile: %v", err)
	}
	second := checker.Snapshot(context.Background(), now.Add(time.Minute))
	if second.Files[0].Valid {
		t.Fatal("second FileSnapshot.Valid = true, want false")
	}
	if got := second.Files[0].LastKnownGood; !got.Available || got.Stale || got.ConsecutiveFailures != 1 || got.Value.Modules["puppetlabs/apt"].Version != "11.2.0" {
		t.Fatalf("second LastKnownGood = %#v, want retained fresh module after one failure", got)
	}

	if err := os.Remove(path); err != nil {
		t.Fatalf("remove Puppetfile: %v", err)
	}
	third := checker.Snapshot(context.Background(), now.Add(SourceDataStaleAfter))
	if got := third.Files[0].LastKnownGood; !got.Available || !got.Stale || got.ConsecutiveFailures != 2 || !got.LastSuccess.Equal(now) {
		t.Fatalf("third LastKnownGood = %#v, want retained stale data after two failures", got)
	}
	if !first.Files[0].LastKnownGood.LastSuccess.Equal(now) {
		t.Fatalf("first LastKnownGood.LastSuccess = %v, want %v", first.Files[0].LastKnownGood.LastSuccess, now)
	}
}

func TestZeroValueCheckerSnapshotsPuppetfile(t *testing.T) {
	t.Parallel()

	path := writeTempPuppetfile(t, "mod 'puppetlabs/stdlib', '9.6.0'\n")
	snapshot := NewChecker(path).Snapshot(context.Background(), time.Unix(1_700_000_000, 0))

	if !snapshot.Success {
		t.Fatalf("Snapshot().Success = false, want true: %v", snapshot.Err)
	}
	if got := snapshot.Modules["puppetlabs/stdlib"]; got.Version != "9.6.0" || got.Type != ModuleTypeForge {
		t.Fatalf("unexpected zero-value checker module: %#v", got)
	}
}

func TestCheckerSnapshotsMultiplePuppetfiles(t *testing.T) {
	t.Parallel()

	pathA := writeTempPuppetfile(t, "mod 'puppetlabs/apt', '11.2.0'\n")
	pathB := writeTempPuppetfile(t, "mod 'puppetlabs/stdlib', '9.6.0'\n")
	now := time.Unix(1_700_000_000, 0)
	snapshot := NewChecker(pathA, pathB).Snapshot(context.Background(), now)

	if !snapshot.Success {
		t.Fatalf("Snapshot().Success = false, want true: %v", snapshot.Err)
	}
	if len(snapshot.Files) != 2 {
		t.Fatalf("len(Files) = %d, want 2", len(snapshot.Files))
	}
	if snapshot.Path != pathA {
		t.Fatalf("Snapshot().Path = %q, want %q", snapshot.Path, pathA)
	}
	if snapshot.Files[0].Path != pathA {
		t.Fatalf("Files[0].Path = %q, want %q", snapshot.Files[0].Path, pathA)
	}
	if snapshot.Files[1].Path != pathB {
		t.Fatalf("Files[1].Path = %q, want %q", snapshot.Files[1].Path, pathB)
	}
	if !snapshot.Files[0].Success || !snapshot.Files[1].Success {
		t.Fatalf("Files[0].Success = %v, Files[1].Success = %v, want true, true", snapshot.Files[0].Success, snapshot.Files[1].Success)
	}
	if snapshot.Files[0].ScrapeDurationSeconds <= 0 {
		t.Fatalf("Files[0].ScrapeDurationSeconds = %v, want > 0", snapshot.Files[0].ScrapeDurationSeconds)
	}
	if snapshot.Files[1].ScrapeDurationSeconds <= 0 {
		t.Fatalf("Files[1].ScrapeDurationSeconds = %v, want > 0", snapshot.Files[1].ScrapeDurationSeconds)
	}
	if snapshot.ReadErrorsTotal != 0 {
		t.Fatalf("ReadErrorsTotal = %d, want 0", snapshot.ReadErrorsTotal)
	}
	if len(snapshot.Modules) != 2 {
		t.Fatalf("len(Modules) = %d, want 2", len(snapshot.Modules))
	}
	if got := snapshot.Modules["puppetlabs/apt"]; got.Version != "11.2.0" || got.Type != ModuleTypeForge {
		t.Fatalf("unexpected module puppetlabs/apt: %#v", got)
	}
	if got := snapshot.Modules["puppetlabs/stdlib"]; got.Version != "9.6.0" || got.Type != ModuleTypeForge {
		t.Fatalf("unexpected module puppetlabs/stdlib: %#v", got)
	}
}

func TestCheckerOneFileFailsAmongMultiple(t *testing.T) {
	t.Parallel()

	pathA := writeTempPuppetfile(t, "mod 'puppetlabs/apt', '11.2.0'\n")
	pathB := filepath.Join(t.TempDir(), "missing-Puppetfile")
	now := time.Unix(1_700_000_000, 0)
	snapshot := NewChecker(pathA, pathB).Snapshot(context.Background(), now)

	if snapshot.Success {
		t.Fatal("Snapshot().Success = true, want false (one file is missing)")
	}
	if len(snapshot.Files) != 2 {
		t.Fatalf("len(Files) = %d, want 2", len(snapshot.Files))
	}
	if !snapshot.Files[0].Success {
		t.Fatalf("Files[0].Success = false, want true")
	}
	if snapshot.Files[1].Success {
		t.Fatalf("Files[1].Success = true, want false (missing file)")
	}
	if snapshot.Files[1].Err == nil {
		t.Fatal("Files[1].Err = nil, want non-nil")
	}
	if snapshot.ReadErrorsTotal != 1 {
		t.Fatalf("ReadErrorsTotal = %d, want 1", snapshot.ReadErrorsTotal)
	}
	if len(snapshot.Modules) != 1 {
		t.Fatalf("len(Modules) = %d, want 1", len(snapshot.Modules))
	}
	if _, ok := snapshot.Modules["puppetlabs/apt"]; !ok {
		t.Fatalf("Modules missing puppetlabs/apt: %#v", snapshot.Modules)
	}
}

func TestCheckerKeepsErrorCountersIsolatedByPath(t *testing.T) {
	t.Parallel()

	invalidPath := writeTempPuppetfile(t, "mod 'puppetlabs/apache'\n")
	validPath := writeTempPuppetfile(t, "mod 'puppetlabs/apt', '11.2.0'\n")
	missingPath := filepath.Join(t.TempDir(), "missing-Puppetfile")
	checker := NewChecker(invalidPath, validPath, missingPath)

	first := checker.Snapshot(context.Background(), time.Unix(1_700_000_000, 0))
	second := checker.Snapshot(context.Background(), time.Unix(1_700_000_001, 0))

	assertFileErrorCounters(t, first.Files[0], invalidPath, 0, 1)
	assertFileErrorCounters(t, first.Files[1], validPath, 0, 0)
	assertFileErrorCounters(t, first.Files[2], missingPath, 1, 0)
	if first.ReadErrorsTotal != 1 || first.ParseErrorsTotal != 1 {
		t.Fatalf("first aggregate error counters = %d/%d, want 1/1", first.ReadErrorsTotal, first.ParseErrorsTotal)
	}

	assertFileErrorCounters(t, second.Files[0], invalidPath, 0, 2)
	assertFileErrorCounters(t, second.Files[1], validPath, 0, 0)
	assertFileErrorCounters(t, second.Files[2], missingPath, 2, 0)
	if second.ReadErrorsTotal != 2 || second.ParseErrorsTotal != 2 {
		t.Fatalf("second aggregate error counters = %d/%d, want 2/2", second.ReadErrorsTotal, second.ParseErrorsTotal)
	}
}

func assertFileErrorCounters(t *testing.T, snapshot FileSnapshot, path string, readErrors uint64, parseErrors uint64) {
	t.Helper()
	if snapshot.Path != path {
		t.Fatalf("FileSnapshot.Path = %q, want %q", snapshot.Path, path)
	}
	if snapshot.ReadErrorsTotal != readErrors || snapshot.ParseErrorsTotal != parseErrors {
		t.Fatalf("%s error counters = %d/%d, want %d/%d", path, snapshot.ReadErrorsTotal, snapshot.ParseErrorsTotal, readErrors, parseErrors)
	}
}
