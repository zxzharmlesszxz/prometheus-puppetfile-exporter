package puppetfilecheck

import (
	"os"
	"path/filepath"
	"testing"
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

func TestParseContentSupportsEscapedQuotesAndCommentCharacters(t *testing.T) {
	t.Parallel()

	result := ParseContent(
		"mod 'example/single', '1\\'2#stable' # trailing comment\n" +
			"mod \"example/double\", \"feature\\\"#1\" # trailing comment\n",
	)

	if got := result.Modules["example/single"]; got.Version != "1'2#stable" || got.Type != ModuleTypeForge {
		t.Fatalf("unexpected escaped single-quoted module: %#v", got)
	}
	if got := result.Modules["example/double"]; got.Version != `feature"#1` || got.Type != ModuleTypeForge {
		t.Fatalf("unexpected escaped double-quoted module: %#v", got)
	}
	if len(result.UnsupportedModules) != 0 || len(result.IncompleteModules) != 0 {
		t.Fatalf("unexpected parser diagnostics: unsupported=%v incomplete=%v", result.UnsupportedModules, result.IncompleteModules)
	}
}

func TestParseContentIgnoresCommaInBareModuleComment(t *testing.T) {
	t.Parallel()

	result := ParseContent("mod 'puppetlabs/apache' # temporarily unpinned, pending review\n")

	if len(result.UnsupportedModules) != 0 {
		t.Fatalf("unexpected unsupported modules: %v", result.UnsupportedModules)
	}
	if len(result.IncompleteModules) != 1 || result.IncompleteModules[0] != "puppetlabs/apache@line:1" {
		t.Fatalf("unexpected incomplete modules: %v", result.IncompleteModules)
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

func TestParseContentSupportsFlexibleOptionWhitespaceWithoutKeySuffixMatches(t *testing.T) {
	t.Parallel()

	result := ParseContent(
		"mod 'example/hashrocket', :git=>'https://git.example.com/hashrocket.git', :tag   =>   '2.4.1'\n" +
			"mod 'example/ruby19', git:'https://git.example.com/ruby19.git', branch:'main'\n" +
			"mod 'example/default-only', git:'https://git.example.com/default.git', default_branch:'main'\n" +
			"mod 'example/auxiliary-only', exclude_spec:true\n",
	)

	if got := result.Modules["example/hashrocket"]; got.Version != "2.4.1" || got.Type != ModuleTypeGitTag {
		t.Fatalf("unexpected compact hashrocket module: %#v", got)
	}
	if got := result.Modules["example/ruby19"]; got.Version != "main" || got.Type != ModuleTypeGitBranch {
		t.Fatalf("unexpected compact Ruby 1.9 module: %#v", got)
	}
	if _, ok := result.Modules["example/default-only"]; ok {
		t.Fatalf("default_branch was incorrectly treated as branch: %#v", result.Modules["example/default-only"])
	}
	wantIncomplete := []string{"example/auxiliary-only@line:4", "example/default-only@line:3"}
	if len(result.IncompleteModules) != len(wantIncomplete) || result.IncompleteModules[0] != wantIncomplete[0] || result.IncompleteModules[1] != wantIncomplete[1] {
		t.Fatalf("unexpected incomplete modules: %v", result.IncompleteModules)
	}
	if len(result.UnsupportedModules) != 0 {
		t.Fatalf("unexpected unsupported modules: %v", result.UnsupportedModules)
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
