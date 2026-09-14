package puppetfile

import "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"

const (
	metricUp                   = "up"
	metricValid                = "valid"
	metricMTime                = "mtime"
	metricModulesDiscovered    = "modules_discovered"
	metricUnsupportedModules   = "unsupported_modules"
	metricIncompleteModules    = "incomplete_modules"
	metricReadErrors           = "read_errors"
	metricParseErrors          = "parse_errors"
	metricScrapeDuration       = "scrape_duration"
	metricModuleInfo           = "module_info"
	metricFilesMonitored       = "files_monitored"
	metricDeclaredModulesTotal = "declared_modules_total"
	metricModuleStatus         = "module_status"
)

var sourceLastKnownGoodMetricIDs = featurekit.LastKnownGoodMetricIDsFor("source")

var featureMetricSpecs = append([]featurekit.FeatureMetricSpec{
	{
		ID:     metricUp,
		Scope:  featurekit.MetricScopeFeature,
		Name:   "_up",
		Help:   "1 if Puppetfile was read successfully.",
		Labels: []string{"path"},
	},
	{
		ID:     metricValid,
		Scope:  featurekit.MetricScopeFeature,
		Name:   "_valid",
		Help:   "1 if Puppetfile was readable and had no unsupported or incomplete module declarations.",
		Labels: []string{"path"},
	},
	{
		ID:     metricMTime,
		Scope:  featurekit.MetricScopeFeature,
		Name:   "_mtime_seconds",
		Help:   "Unix timestamp of the Puppetfile modification time, or 0 if unavailable.",
		Labels: []string{"path"},
	},
	{
		ID:     metricModulesDiscovered,
		Scope:  featurekit.MetricScopeFeature,
		Name:   "_modules_discovered",
		Help:   "Number of supported pinned modules discovered in Puppetfile during the last refresh.",
		Labels: []string{"path"},
	},
	{
		ID:     metricUnsupportedModules,
		Scope:  featurekit.MetricScopeFeature,
		Name:   "_unsupported_modules_count",
		Help:   "Number of unsupported Puppetfile module declarations seen during the last refresh.",
		Labels: []string{"path"},
	},
	{
		ID:     metricIncompleteModules,
		Scope:  featurekit.MetricScopeFeature,
		Name:   "_incomplete_modules_count",
		Help:   "Number of incomplete Puppetfile module declarations seen during the last refresh.",
		Labels: []string{"path"},
	},
	{
		ID:     metricReadErrors,
		Scope:  featurekit.MetricScopeFeature,
		Name:   "_read_errors_total",
		Help:   "Total number of Puppetfile read errors.",
		Labels: []string{"path"},
	},
	{
		ID:     metricParseErrors,
		Scope:  featurekit.MetricScopeFeature,
		Name:   "_parse_errors_total",
		Help:   "Total number of Puppetfile refreshes with unsupported or incomplete module declarations.",
		Labels: []string{"path"},
	},
	{
		ID:     metricScrapeDuration,
		Scope:  featurekit.MetricScopeFeature,
		Name:   "_scrape_duration_seconds",
		Help:   "Time spent refreshing metrics from Puppetfile.",
		Labels: []string{"path"},
	},
	{
		ID:     metricModuleInfo,
		Scope:  featurekit.MetricScopeFeature,
		Name:   "_module_info",
		Help:   "Pinned Puppet modules declared in Puppetfile.",
		Labels: []string{"path", "module", "owner", "name", "current_version", "type"},
	},
	{
		ID:     metricFilesMonitored,
		Scope:  featurekit.MetricScopeFeature,
		Name:   "_files_monitored",
		Help:   "Number of Puppetfile paths configured and monitored.",
		Labels: []string{},
	},
	{
		ID:     metricDeclaredModulesTotal,
		Scope:  featurekit.MetricScopeFeature,
		Name:   "_declared_modules_total",
		Help:   "Total number of declared modules (supported + unsupported + incomplete) in Puppetfile.",
		Labels: []string{"path"},
	},
	{
		ID:     metricModuleStatus,
		Scope:  featurekit.MetricScopeFeature,
		Name:   "_module_status",
		Help:   "1 if the module declaration is supported and well-formed, 0 otherwise.",
		Labels: []string{"path", "module"},
	},
}, featurekit.LastKnownGoodMetricSpecs("source", []string{"path"})...)
