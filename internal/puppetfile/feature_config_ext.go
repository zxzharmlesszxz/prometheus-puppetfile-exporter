package puppetfile

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"

	"github.com/zxzharmlesszxz/prometheus-puppetfile-exporter/internal/puppetfilecheck"
)

func hasGlobMeta(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

func expandPaths(raw string) []string {
	var paths []string
	seen := make(map[string]struct{})
	appendPath := func(path string) {
		if _, exists := seen[path]; exists {
			return
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}

	for p := range strings.SplitSeq(raw, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if hasGlobMeta(p) {
			matches, err := filepath.Glob(p)
			if err != nil {
				// malformed glob — treat as literal path
				appendPath(p)
				continue
			}
			if len(matches) > 0 {
				for _, match := range matches {
					appendPath(match)
				}
			} else {
				// no files match — keep pattern so read fails with clear error
				appendPath(p)
			}
		} else {
			appendPath(p)
		}
	}
	if len(paths) == 0 {
		paths = []string{puppetfilecheck.DefaultPuppetfilePath}
	}
	return paths
}

type Config struct {
	ConfigFile      string
	PuppetfilePath  string
	PuppetfilePaths []string

	PuppetfilePathConfigured bool
}

const (
	DefaultRefreshInterval = time.Minute
	DefaultPuppetfilePath  = puppetfilecheck.DefaultPuppetfilePath
)

var DefaultFeatureConfigFileName = "prometheus-puppetfile-exporter.yml"

type featureConfigFile struct {
	PuppetfilePath string `yaml:"puppetfile_path"`
}

var featureConfigFlagSpecs = []featurekit.FeatureConfigFlagSpec[Config]{
	{
		Name:        "puppetfile-path",
		Help:        "Path to Puppetfile (comma-separated, supports glob patterns like /etc/puppetlabs/code/environments/*/Puppetfile)",
		Default:     DefaultPuppetfilePath,
		Placeholder: "/etc/puppetlabs/code/environments/production/Puppetfile",
		Bind: func(flag *kingpin.FlagClause, config *Config) {
			flag.StringVar(&config.PuppetfilePath)
			flag.IsSetByUser(&config.PuppetfilePathConfigured)
		},
	},
}

func NewDefaultConfig() Config {
	return Config{
		PuppetfilePath: DefaultPuppetfilePath,
	}
}

func FeatureConfigFile(config *Config) *string {
	return &config.ConfigFile
}

func ValidateFeatureConfig(config Config) error {
	if strings.TrimSpace(config.PuppetfilePath) == "" {
		return fmt.Errorf("puppetfile_path must not be empty")
	}
	return nil
}

func FeatureRuntimeConfigEntries(_ featurekit.RuntimeConfigContext[Config], config Config) []any {
	return []any{
		"puppetfile_path", config.PuppetfilePath,
	}
}

func ResolveFeatureConfig(featureName string, config Config) (Config, string, bool, error) {
	config.PuppetfilePath = strings.TrimSpace(config.PuppetfilePath)

	var fileConfig featureConfigFile
	cfgFile, loaded, err := featurekit.LoadFeatureConfigFile(featureName, config.ConfigFile, &fileConfig)
	if err != nil {
		return config, cfgFile, false, err
	}

	if loaded {
		filePath := strings.TrimSpace(fileConfig.PuppetfilePath)
		if filePath != "" && !config.PuppetfilePathConfigured {
			config.PuppetfilePath = filePath
		}
	}
	if config.PuppetfilePath == "" {
		config.PuppetfilePath = DefaultPuppetfilePath
	}
	config.PuppetfilePaths = expandPaths(config.PuppetfilePath)
	return config, cfgFile, loaded, nil
}
