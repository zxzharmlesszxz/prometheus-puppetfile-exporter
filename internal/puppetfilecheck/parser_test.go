package puppetfilecheck

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseFile(t *testing.T) {
	t.Parallel()

	puppetfile := "forge 'https://forge.example.com'\n\n" +
		"mod 'puppetlabs/apt', '11.2.0'\n" +
		"mod 'example-nginx',\n" +
		"  :git => 'https://git.example.com/platform/example-nginx.git',\n" +
		"  :tag => '2.4.1'\n" +
		"mod 'example/redis',\n" +
		"  :git => 'https://git.example.com/platform/example-redis.git',\n" +
		"  :branch => 'main'\n"

	path := writeTempPuppetfile(t, puppetfile)
	result, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile() error = %v, want nil", err)
	}
	modules := result.Modules

	if got := modules["puppetlabs/apt"]; got.Version != "11.2.0" || got.Type != ModuleTypeForge || got.Owner != "puppetlabs" || got.Name != "apt" {
		t.Fatalf("unexpected forge module: %#v", got)
	}
	if got := modules["example-nginx"]; got.Version != "2.4.1" || got.Type != ModuleTypeGitTag || got.Owner != "example" || got.Name != "nginx" {
		t.Fatalf("unexpected git tag module: %#v", got)
	}
	if got := modules["example/redis"]; got.Version != "main" || got.Type != ModuleTypeGitBranch || got.Owner != "example" || got.Name != "redis" {
		t.Fatalf("unexpected git branch module: %#v", got)
	}
}

func TestParseFileSupportsDoubleQuotedDeclarations(t *testing.T) {
	t.Parallel()

	puppetfile := "mod \"puppetlabs/apt\", \"11.2.0\"\n" +
		"mod \"example-nginx\",\n" +
		"  :git => \"https://git.example.com/platform/example-nginx.git\",\n" +
		"  :tag => \"2.4.1\"\n" +
		"mod \"example/redis\",\n" +
		"  :git => \"https://git.example.com/platform/example-redis.git\",\n" +
		"  :branch => \"main\"\n"

	path := writeTempPuppetfile(t, puppetfile)
	result, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile() error = %v, want nil", err)
	}

	if got := result.Modules["puppetlabs/apt"]; got.Version != "11.2.0" || got.Type != ModuleTypeForge {
		t.Fatalf("unexpected double-quoted forge module: %#v", got)
	}
	if got := result.Modules["example-nginx"]; got.Version != "2.4.1" || got.Type != ModuleTypeGitTag {
		t.Fatalf("unexpected double-quoted git tag module: %#v", got)
	}
	if got := result.Modules["example/redis"]; got.Version != "main" || got.Type != ModuleTypeGitBranch {
		t.Fatalf("unexpected double-quoted git branch module: %#v", got)
	}
}

func TestParseFileSupportsRuby19HashSyntax(t *testing.T) {
	t.Parallel()

	puppetfile := "mod 'example-nginx',\n" +
		"  git: 'https://git.example.com/platform/example-nginx.git',\n" +
		"  tag: '2.4.1'\n" +
		"mod 'example/redis',\n" +
		"  git: \"https://git.example.com/platform/example-redis.git\",\n" +
		"  branch: \"main\"\n"

	path := writeTempPuppetfile(t, puppetfile)
	result, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile() error = %v, want nil", err)
	}

	if got := result.Modules["example-nginx"]; got.Version != "2.4.1" || got.Type != ModuleTypeGitTag {
		t.Fatalf("unexpected Ruby 1.9 git tag module: %#v", got)
	}
	if got := result.Modules["example/redis"]; got.Version != "main" || got.Type != ModuleTypeGitBranch {
		t.Fatalf("unexpected Ruby 1.9 git branch module: %#v", got)
	}
}

func TestParseFileSupportsOfficialGitPinnedVariants(t *testing.T) {
	t.Parallel()

	puppetfile := "mod 'example-ref',\n" +
		"  :git => 'https://git.example.com/platform/example-ref.git',\n" +
		"  :ref => 'release-2026'\n" +
		"mod 'example-commit',\n" +
		"  :git => 'https://git.example.com/platform/example-commit.git',\n" +
		"  :commit => '8df51aa'\n" +
		"mod 'example-data',\n" +
		"  :git => 'https://git.example.com/platform/example-data.git',\n" +
		"  :branch => :control_branch,\n" +
		"  :default_branch => 'main',\n" +
		"  :install_path => 'hieradata'\n"

	path := writeTempPuppetfile(t, puppetfile)
	result, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile() error = %v, want nil", err)
	}

	if got := result.Modules["example-ref"]; got.Version != "release-2026" || got.Type != ModuleTypeGitRef {
		t.Fatalf("unexpected git ref module: %#v", got)
	}
	if got := result.Modules["example-commit"]; got.Version != "8df51aa" || got.Type != ModuleTypeGitCommit {
		t.Fatalf("unexpected git commit module: %#v", got)
	}
	if got := result.Modules["example-data"]; got.Version != "control_branch" || got.Type != ModuleTypeGitBranch {
		t.Fatalf("unexpected control branch module: %#v", got)
	}
}

func TestParseFileTracksOfficialButUnpinnedForgeVariantsAsIncomplete(t *testing.T) {
	t.Parallel()

	puppetfile := "forge 'https://forge.example.com'\n" +
		"moduledir 'thirdparty'\n" +
		"mod 'puppetlabs/apache'\n" +
		"mod 'puppetlabs/ntp', :latest\n"

	path := writeTempPuppetfile(t, puppetfile)
	result, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile() error = %v, want nil", err)
	}

	if len(result.Modules) != 0 {
		t.Fatalf("unexpected parsed modules: %#v", result.Modules)
	}
	if len(result.IncompleteModules) != 2 {
		t.Fatalf("unexpected incomplete modules count: %#v", result.IncompleteModules)
	}
	if result.IncompleteModules[0] != "puppetlabs/apache@line:3" || result.IncompleteModules[1] != "puppetlabs/ntp@line:4" {
		t.Fatalf("unexpected incomplete modules: %#v", result.IncompleteModules)
	}
}

func TestParseFileDoesNotBleedIntoNextModule(t *testing.T) {
	t.Parallel()

	puppetfile := "mod 'broken-module',\n" +
		"  :git => 'https://git.example.com/platform/broken-module.git'\n" +
		"mod 'puppetlabs/stdlib', '9.6.0'\n"

	path := writeTempPuppetfile(t, puppetfile)
	result, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile() error = %v, want nil", err)
	}

	if got := result.Modules["puppetlabs/stdlib"]; got.Version != "9.6.0" || got.Type != ModuleTypeForge || got.Owner != "puppetlabs" || got.Name != "stdlib" {
		t.Fatalf("unexpected next module parsing result: %#v", got)
	}
	if len(result.IncompleteModules) != 1 || result.IncompleteModules[0] != "broken-module@line:1" {
		t.Fatalf("unexpected incomplete modules: %#v", result.IncompleteModules)
	}
}

func TestParseFileTracksUnsupportedModuleDeclaration(t *testing.T) {
	t.Parallel()

	puppetfile := "mod 'example/local',\n" +
		"  :path => 'modules/example/local'\n"

	path := writeTempPuppetfile(t, puppetfile)
	result, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile() error = %v, want nil", err)
	}
	if len(result.UnsupportedModules) != 1 || result.UnsupportedModules[0] != "example/local@line:1" {
		t.Fatalf("unexpected unsupported modules: %#v", result.UnsupportedModules)
	}
}

func TestParseFileTracksInvalidEntriesFromFixture(t *testing.T) {
	t.Parallel()

	result, err := ParseFile(filepath.Join("testdata", "Puppetfile"))
	if err != nil {
		t.Fatalf("ParseFile() error = %v, want nil", err)
	}

	if len(result.Modules) != 7 {
		t.Fatalf("unexpected modules count: %d", len(result.Modules))
	}
	if got := result.Modules["puppetlabs/apt"]; got.Version != "11.2.0" || got.Type != ModuleTypeForge {
		t.Fatalf("unexpected fixture forge module: %#v", got)
	}
	if got := result.Modules["puppetlabs/stdlib"]; got.Version != "9.6.0" || got.Type != ModuleTypeForge {
		t.Fatalf("unexpected fixture stdlib module: %#v", got)
	}
	if got := result.Modules["example-nginx"]; got.Version != "2.4.1" || got.Type != ModuleTypeGitTag {
		t.Fatalf("unexpected fixture git tag module: %#v", got)
	}
	if got := result.Modules["example/redis"]; got.Version != "main" || got.Type != ModuleTypeGitBranch {
		t.Fatalf("unexpected fixture git branch module: %#v", got)
	}
	if got := result.Modules["example-ref"]; got.Version != "release-2026" || got.Type != ModuleTypeGitRef {
		t.Fatalf("unexpected fixture git ref module: %#v", got)
	}
	if got := result.Modules["example-commit"]; got.Version != "8df51aa" || got.Type != ModuleTypeGitCommit {
		t.Fatalf("unexpected fixture git commit module: %#v", got)
	}
	if got := result.Modules["example-data"]; got.Version != "control_branch" || got.Type != ModuleTypeGitBranch {
		t.Fatalf("unexpected fixture control branch module: %#v", got)
	}
	if len(result.UnsupportedModules) != 1 || result.UnsupportedModules[0] != "example/local@line:8" {
		t.Fatalf("unexpected fixture unsupported modules: %#v", result.UnsupportedModules)
	}
	if len(result.IncompleteModules) != 3 {
		t.Fatalf("unexpected fixture incomplete modules count: %#v", result.IncompleteModules)
	}
	if result.IncompleteModules[0] != "broken-module@line:10" || result.IncompleteModules[1] != "puppetlabs/apache@line:6" || result.IncompleteModules[2] != "puppetlabs/ntp@line:7" {
		t.Fatalf("unexpected fixture incomplete modules: %#v", result.IncompleteModules)
	}
}

func TestParseFileReturnsErrorForMissingFile(t *testing.T) {
	t.Parallel()

	_, err := ParseFile(filepath.Join(t.TempDir(), "missing-Puppetfile"))
	if err == nil {
		t.Fatal("expected ParseFile() to return error for missing file")
	}
}

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
	// aggregate Modules should only contain the successful file
	if len(snapshot.Modules) != 1 {
		t.Fatalf("len(Modules) = %d, want 1", len(snapshot.Modules))
	}
	if _, ok := snapshot.Modules["puppetlabs/apt"]; !ok {
		t.Fatalf("Modules missing puppetlabs/apt: %#v", snapshot.Modules)
	}
}

func TestSplitModuleName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		module string
		owner  string
		name   string
	}{
		{module: "puppetlabs/apt", owner: "puppetlabs", name: "apt"},
		{module: "example-nginx", owner: "example", name: "nginx"},
		{module: "stdlib", owner: "", name: "stdlib"},
	}

	for _, test := range tests {
		owner, name := splitModuleName(test.module)
		if owner != test.owner || name != test.name {
			t.Fatalf("splitModuleName(%q) = (%q, %q), want (%q, %q)", test.module, owner, name, test.owner, test.name)
		}
	}
}

func writeTempPuppetfile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "Puppetfile")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp Puppetfile: %v", err)
	}
	return path
}
