package puppetfilecheck

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

func ParseFile(path string) (ParseResult, error) {
	content, err := load(path)
	if err != nil {
		return ParseResult{}, err
	}

	return ParseContent(string(content)), nil
}

func ParseContent(content string) ParseResult {
	result := ParseResult{
		Modules: make(Modules),
	}

	lines := strings.Split(content, "\n")

	for i, rawLine := range lines {
		line := strings.TrimSpace(stripComment(rawLine))
		if line == "" || line[0] == '#' {
			continue
		}

		if !strings.HasPrefix(line, "mod ") && !strings.HasPrefix(line, "mod\t") {
			continue
		}

		body := strings.TrimSpace(line[3:])

		name, argsBody, ok := scanQuoted(body)
		if !ok {
			continue
		}

		argsBody = strings.TrimLeft(argsBody, ",")
		argsBody = strings.TrimSpace(argsBody)
		lineNum := i + 1

		// Forge module: second quoted string = version
		if ver, _, ok := scanQuoted(argsBody); ok {
			owner, modName := splitModuleName(name)
			result.Modules[name] = ModuleInfo{
				Owner:   owner,
				Name:    modName,
				Version: ver,
				Type:    ModuleTypeForge,
			}
			continue
		}

		// :latest
		if argsBody == ":latest" {
			result.IncompleteModules = append(result.IncompleteModules, fmt.Sprintf("%s@line:%d", name, lineNum))
			continue
		}

		// No comma after module name -> single-line mod with nothing else
		if !strings.Contains(rawLine, ",") {
			result.IncompleteModules = append(result.IncompleteModules, fmt.Sprintf("%s@line:%d", name, lineNum))
			continue
		}

		// Multi-line source module (git, path, etc.)
		info, status := parseModSource(lines, i, argsBody)
		if status == moduleParseOK {
			info.Owner, info.Name = splitModuleName(name)
			result.Modules[name] = info
			continue
		}

		entry := fmt.Sprintf("%s@line:%d", name, lineNum)
		if status == moduleParseIncomplete {
			result.IncompleteModules = append(result.IncompleteModules, entry)
			continue
		}
		result.UnsupportedModules = append(result.UnsupportedModules, entry)
	}

	sort.Strings(result.UnsupportedModules)
	sort.Strings(result.IncompleteModules)
	return result
}

// stripComment removes Ruby-style # comments outside quoted strings.
func stripComment(line string) string {
	inSingle := false
	inDouble := false
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				return line[:i]
			}
		}
	}
	return line
}

// scanQuoted extracts the first single-quoted or double-quoted string from s.
func scanQuoted(s string) (value string, rest string, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", "", false
	}

	var quote byte
	switch s[0] {
	case '\'':
		quote = '\''
	case '"':
		quote = '"'
	default:
		return "", "", false
	}

	for i := 1; i < len(s); i++ {
		if s[i] == quote {
			return s[1:i], strings.TrimSpace(s[i+1:]), true
		}
	}
	return "", "", false
}

// parseModSource parses a multi-line source module (git, path, etc.)
// starting from a mod declaration at lines[modIndex].
func parseModSource(lines []string, modIndex int, argsBody string) (ModuleInfo, moduleParseStatus) {
	var repo, tag, branch, ref, commit string
	var sawKnown bool

	repo, tag, branch, ref, commit, sawKnown = mergeSourceOptions(repo, tag, branch, ref, commit, sawKnown, argsBody)
	if ver, modType, ok := resolvePinnedVersion(tag, branch, ref, commit); ok && repo != "" {
		return ModuleInfo{Version: ver, Type: modType}, moduleParseOK
	}

	for j := modIndex + 1; j < len(lines); j++ {
		nextLine := strings.TrimSpace(lines[j])
		if nextLine == "" || nextLine[0] == '#' {
			continue
		}

		if strings.HasPrefix(nextLine, "mod ") || strings.HasPrefix(nextLine, "mod\t") {
			break
		}

		repo, tag, branch, ref, commit, sawKnown = mergeSourceOptions(repo, tag, branch, ref, commit, sawKnown, nextLine)
		if ver, modType, ok := resolvePinnedVersion(tag, branch, ref, commit); ok && repo != "" {
			return ModuleInfo{Version: ver, Type: modType}, moduleParseOK
		}
	}

	if repo != "" {
		return ModuleInfo{}, moduleParseIncomplete
	}
	if sawKnown {
		return ModuleInfo{}, moduleParseIncomplete
	}
	return ModuleInfo{}, moduleParseUnsupported
}

// mergeSourceOptions extracts known source options from a line and merges them.
func mergeSourceOptions(repo, tag, branch, ref, commit string, sawKnown bool, line string) (string, string, string, string, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return repo, tag, branch, ref, commit, sawKnown
	}

	if v := extractValue(line, "git"); v != "" {
		repo = v
		sawKnown = true
	}
	if v := extractValue(line, "tag"); v != "" {
		tag = v
		sawKnown = true
	}
	if v := extractValue(line, "branch"); v != "" {
		branch = v
		sawKnown = true
	}
	if v := extractValue(line, "ref"); v != "" {
		ref = v
		sawKnown = true
	}
	if v := extractValue(line, "commit"); v != "" {
		commit = v
		sawKnown = true
	}

	if containsLineOption(line, "install_path") ||
		containsLineOption(line, "default_branch") ||
		strings.Contains(line, "exclude_spec") {
		sawKnown = true
	}

	return repo, tag, branch, ref, commit, sawKnown
}

// extractValue extracts the Puppetfile option value for key from line.
// Supports both :key => 'val' and key: 'val' (Ruby 1.9 hash) syntax.
func extractValue(line, key string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}

	// :key => 'value' or :key => :symbol
	prefix := ":" + key + " => "
	if idx := strings.Index(line, prefix); idx >= 0 {
		rest := strings.TrimSpace(line[idx+len(prefix):])
		if val, _, ok := scanQuoted(rest); ok {
			return val
		}
		if rest != "" && rest[0] == ':' {
			end := strings.IndexAny(rest, ", \t")
			if end < 0 {
				end = len(rest)
			}
			return rest[1:end]
		}
	}

	// key: 'value' (Ruby 1.9)
	prefix = key + ": "
	if idx := strings.Index(line, prefix); idx >= 0 {
		rest := strings.TrimSpace(line[idx+len(prefix):])
		if val, _, ok := scanQuoted(rest); ok {
			return val
		}
		// Symbol value
		if rest != "" && rest[0] == ':' {
			end := strings.IndexAny(rest, ", \t")
			if end < 0 {
				end = len(rest)
			}
			return rest[1:end]
		}
	}

	return ""
}

// containsLineOption checks if a line contains a Puppetfile option key.
func containsLineOption(line, key string) bool {
	line = strings.TrimSpace(line)
	return strings.Contains(line, ":"+key+" => ") ||
		strings.Contains(line, key+": ")
}

func resolvePinnedVersion(tag string, branch string, ref string, commit string) (string, ModuleType, bool) {
	switch {
	case tag != "":
		return tag, ModuleTypeGitTag, true
	case branch != "":
		return branch, ModuleTypeGitBranch, true
	case ref != "":
		return ref, ModuleTypeGitRef, true
	case commit != "":
		return commit, ModuleTypeGitCommit, true
	default:
		return "", "", false
	}
}

func splitModuleName(module string) (owner string, name string) {
	owner, name, found := strings.Cut(module, "/")
	if found {
		return owner, name
	}

	owner, name, found = strings.Cut(module, "-")
	if found {
		return owner, name
	}

	return "", module
}

type moduleParseStatus uint8

const (
	moduleParseUnsupported moduleParseStatus = iota
	moduleParseIncomplete
	moduleParseOK
)

func load(path string) ([]byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read Puppetfile: %w", err)
	}
	return content, nil
}
