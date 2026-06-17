package puppetfilecheck

import "time"

type ModuleType string

const (
	ModuleTypeForge       ModuleType = "forge"
	ModuleTypeGitTag      ModuleType = "git+tag"
	ModuleTypeGitBranch   ModuleType = "git+branch"
	ModuleTypeGitRef      ModuleType = "git+ref"
	ModuleTypeGitCommit   ModuleType = "git+commit"
	DefaultPuppetfilePath            = "/etc/puppetlabs/code/environments/production/Puppetfile"
)

type ModuleInfo struct {
	Owner   string
	Name    string
	Version string
	Type    ModuleType
}

type Modules map[string]ModuleInfo

type ParseResult struct {
	Modules            Modules
	UnsupportedModules []string
	IncompleteModules  []string
}

type FileSnapshot struct {
	Path                  string
	Success               bool
	Valid                 bool
	Err                   error
	MTimeSeconds          float64
	ParseErrorsTotal      uint64
	ScrapeDurationSeconds float64
	Modules               Modules
	UnsupportedModules    []string
	IncompleteModules     []string
}

type Snapshot struct {
	AttemptTime           time.Time
	Success               bool
	Err                   error
	Path                  string
	MTimeSeconds          float64
	ReadErrorsTotal       uint64
	ParseErrorsTotal      uint64
	ScrapeDurationSeconds float64
	Modules               Modules
	UnsupportedModules    []string
	IncompleteModules     []string
	Files                 []FileSnapshot
}
