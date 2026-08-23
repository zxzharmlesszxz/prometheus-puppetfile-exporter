package puppetfile

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestHasGlobMeta(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"/plain/path/Puppetfile", false},
		{"/path/*/Puppetfile", true},
		{"/path/?/Puppetfile", true},
		{"/path/[abc]/Puppetfile", true},
		{"/path/star/Puppetfile", false},
		{"", false},
	}
	for _, tc := range tests {
		got := hasGlobMeta(tc.s)
		if got != tc.want {
			t.Errorf("hasGlobMeta(%q) = %v, want %v", tc.s, got, tc.want)
		}
	}
}

func TestExpandPaths(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{
			name: "single path",
			raw:  "/path/to/Puppetfile",
			want: []string{"/path/to/Puppetfile"},
		},
		{
			name: "comma separated",
			raw:  "/a/Puppetfile,/b/Puppetfile",
			want: []string{"/a/Puppetfile", "/b/Puppetfile"},
		},
		{
			name: "duplicate explicit paths removed",
			raw:  "/a/Puppetfile,/b/Puppetfile,/a/Puppetfile",
			want: []string{"/a/Puppetfile", "/b/Puppetfile"},
		},
		{
			name: "spaces around comma",
			raw:  " /a/Puppetfile , /b/Puppetfile ",
			want: []string{"/a/Puppetfile", "/b/Puppetfile"},
		},
		{
			name: "empty segments removed",
			raw:  " /a/Puppetfile, ,/b/Puppetfile,",
			want: []string{"/a/Puppetfile", "/b/Puppetfile"},
		},
		{
			name: "all empty falls to default",
			raw:  ", ,",
			want: []string{DefaultPuppetfilePath},
		},
		{
			name: "empty string falls to default",
			raw:  "",
			want: []string{DefaultPuppetfilePath},
		},
		{
			name: "mixed with trailing comma",
			raw:  "/a/Puppetfile,",
			want: []string{"/a/Puppetfile"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := expandPaths(tc.raw)
			if len(got) != len(tc.want) {
				t.Fatalf("expandPaths(%q) = %v (len %d), want %v (len %d)", tc.raw, got, len(got), tc.want, len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("expandPaths(%q)[%d] = %q, want %q", tc.raw, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestExpandPathsGlob(t *testing.T) {
	tmpDir := t.TempDir()

	dirs := []string{"production", "staging", "testing"}
	var expected []string
	for _, env := range dirs {
		dir := filepath.Join(tmpDir, "environments", env)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "Puppetfile")
		if err := os.WriteFile(path, []byte("# "+env+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		expected = append(expected, path)
	}

	pattern := filepath.Join(tmpDir, "environments", "*", "Puppetfile")
	got := expandPaths(pattern)

	sort.Strings(got)
	sort.Strings(expected)

	if len(got) != len(expected) {
		t.Fatalf("expandPaths(%q) = %v (len %d), want %v (len %d)", pattern, got, len(got), expected, len(expected))
	}
	for i := range got {
		if got[i] != expected[i] {
			t.Fatalf("expandPaths(%q)[%d] = %q, want %q", pattern, i, got[i], expected[i])
		}
	}
}

func TestExpandPathsGlobNoMatch(t *testing.T) {
	pattern := filepath.Join(t.TempDir(), "environments", "*", "Puppetfile")
	got := expandPaths(pattern)
	if len(got) != 1 || got[0] != pattern {
		t.Fatalf("expandPaths(%q) = %v, want [%q]", pattern, got, pattern)
	}
}

func TestExpandPathsGlobMixedLiteralAndGlob(t *testing.T) {
	tmpDir := t.TempDir()

	literal := filepath.Join(tmpDir, "literal", "Puppetfile")
	if err := os.MkdirAll(filepath.Dir(literal), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(literal, []byte("# literal\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	matchDir := filepath.Join(tmpDir, "env", "prod")
	matchFile := filepath.Join(matchDir, "Puppetfile")
	if err := os.MkdirAll(matchDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(matchFile, []byte("# prod\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	raw := literal + "," + filepath.Join(tmpDir, "env", "*", "Puppetfile")
	got := expandPaths(raw)

	if len(got) != 2 {
		t.Fatalf("expandPaths(%q) = %v, want 2 paths", raw, got)
	}
	if got[0] != literal {
		t.Fatalf("expandPaths(...)[0] = %q, want %q", got[0], literal)
	}
	if got[1] != matchFile {
		t.Fatalf("expandPaths(...)[1] = %q, want %q", got[1], matchFile)
	}
}

func TestExpandPathsDeduplicatesGlobAndExplicitOverlap(t *testing.T) {
	tmpDir := t.TempDir()
	var expected []string
	for _, env := range []string{"production", "testing"} {
		path := filepath.Join(tmpDir, "environments", env, "Puppetfile")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("# "+env+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		expected = append(expected, path)
	}

	pattern := filepath.Join(tmpDir, "environments", "*", "Puppetfile")
	raw := pattern + "," + expected[0] + "," + pattern
	got := expandPaths(raw)

	if len(got) != len(expected) {
		t.Fatalf("expandPaths(%q) = %v, want %v", raw, got, expected)
	}
	for i := range expected {
		if got[i] != expected[i] {
			t.Fatalf("expandPaths(%q)[%d] = %q, want %q", raw, i, got[i], expected[i])
		}
	}
}

func TestExpandPathsMalformedGlob(t *testing.T) {
	raw := "[/bad/glob/Puppetfile"
	got := expandPaths(raw)
	if len(got) != 1 || got[0] != raw {
		t.Fatalf("expandPaths(%q) = %v, want [%q]", raw, got, raw)
	}
}
