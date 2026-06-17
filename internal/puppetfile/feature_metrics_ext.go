package puppetfile

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"

	"github.com/zxzharmlesszxz/prometheus-puppetfile-exporter/internal/puppetfilecheck"
)

type featureMetricState struct {
	lastModulesCount     *atomic.Int64
	lastUnsupportedState *atomic.Value
	lastIncompleteState  *atomic.Value
}

func NewFeatureMetricHandlers() featurekit.FeatureMetricHandlers[Snapshot] {
	state := &featureMetricState{
		lastModulesCount:     &atomic.Int64{},
		lastUnsupportedState: &atomic.Value{},
		lastIncompleteState:  &atomic.Value{},
	}
	state.lastModulesCount.Store(-1)
	state.lastUnsupportedState.Store("")
	state.lastIncompleteState.Store("")
	return featurekit.FeatureMetricHandlers[Snapshot]{
		Collect:  state.CollectFeatureMetrics,
		LogError: state.LogFeatureSnapshotError,
	}
}

func (state *featureMetricState) CollectFeatureMetrics(ctx featurekit.FeatureMetricsContext[Snapshot], ch chan<- prometheus.Metric, snapshot Snapshot, _ time.Time) {
	ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(metricReadErrors), prometheus.CounterValue, float64(snapshot.puppetfile.ReadErrorsTotal), snapshot.puppetfile.Path)
	ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(metricFilesMonitored), prometheus.GaugeValue, float64(len(snapshot.puppetfile.Files)))

	for _, file := range snapshot.puppetfile.Files {
		ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(metricUp), prometheus.GaugeValue, framework.BoolFloat(file.Success), file.Path)
		ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(metricValid), prometheus.GaugeValue, framework.BoolFloat(file.Valid), file.Path)
		ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(metricMTime), prometheus.GaugeValue, file.MTimeSeconds, file.Path)
		ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(metricParseErrors), prometheus.CounterValue, float64(file.ParseErrorsTotal), file.Path)
		ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(metricScrapeDuration), prometheus.GaugeValue, file.ScrapeDurationSeconds, file.Path)

		if !file.Success {
			ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(metricModulesDiscovered), prometheus.GaugeValue, 0, file.Path)
			ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(metricUnsupportedModules), prometheus.GaugeValue, 0, file.Path)
			ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(metricIncompleteModules), prometheus.GaugeValue, 0, file.Path)
			ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(metricDeclaredModulesTotal), prometheus.GaugeValue, 0, file.Path)
			continue
		}

		ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(metricModulesDiscovered), prometheus.GaugeValue, float64(len(file.Modules)), file.Path)
		ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(metricUnsupportedModules), prometheus.GaugeValue, float64(len(file.UnsupportedModules)), file.Path)
		ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(metricIncompleteModules), prometheus.GaugeValue, float64(len(file.IncompleteModules)), file.Path)
		ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(metricDeclaredModulesTotal), prometheus.GaugeValue, float64(len(file.Modules)+len(file.UnsupportedModules)+len(file.IncompleteModules)), file.Path)

		for _, module := range sortedModuleNames(file.Modules) {
			info := file.Modules[module]
			ch <- prometheus.MustNewConstMetric(
				ctx.Descriptors.Get(metricModuleInfo),
				prometheus.GaugeValue,
				1,
				file.Path,
				module,
				info.Owner,
				info.Name,
				info.Version,
				string(info.Type),
			)
			ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(metricModuleStatus), prometheus.GaugeValue, 1, file.Path, module)
		}

		for _, raw := range file.UnsupportedModules {
			module, _, _ := strings.Cut(raw, "@")
			ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(metricModuleStatus), prometheus.GaugeValue, 0, file.Path, module)
		}
		for _, raw := range file.IncompleteModules {
			module, _, _ := strings.Cut(raw, "@")
			ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(metricModuleStatus), prometheus.GaugeValue, 0, file.Path, module)
		}
	}
}

func (state *featureMetricState) LogFeatureSnapshotError(_ featurekit.FeatureMetricsContext[Snapshot], logger *slog.Logger, snapshot Snapshot) {
	for _, file := range snapshot.puppetfile.Files {
		if !file.Success && file.Err != nil {
			logger.Error("read Puppetfile failed", "path", file.Path, "err", file.Err)
		}
	}

	// Use aggregate lists for dedup state (shared across files) so
	// per-file state is not clobbered by files without the condition.
	if len(snapshot.puppetfile.UnsupportedModules) > 0 {
		fingerprint := moduleStateFingerprint(snapshot.puppetfile.UnsupportedModules)
		if swapStringValue(state.lastUnsupportedState, fingerprint) != fingerprint {
			logger.Warn("skipping unsupported Puppetfile modules",
				"modules", snapshot.puppetfile.UnsupportedModules,
			)
		}
	} else {
		state.lastUnsupportedState.Store("")
	}

	if len(snapshot.puppetfile.IncompleteModules) > 0 {
		fingerprint := moduleStateFingerprint(snapshot.puppetfile.IncompleteModules)
		if swapStringValue(state.lastIncompleteState, fingerprint) != fingerprint {
			logger.Warn("skipping incomplete Puppetfile modules",
				"modules", snapshot.puppetfile.IncompleteModules,
			)
		}
	} else {
		state.lastIncompleteState.Store("")
	}

	if len(snapshot.puppetfile.Modules) == 0 {
		if state.lastModulesCount.Swap(0) != 0 {
			logger.Warn("no modules discovered in Puppetfile")
		}
		return
	}
	state.lastModulesCount.Store(int64(len(snapshot.puppetfile.Modules)))
	logger.Debug("collected Puppetfile module metrics",
		"modules_count", len(snapshot.puppetfile.Modules),
	)
}

func sortedModuleNames(modules puppetfilecheck.Modules) []string {
	names := make([]string, 0, len(modules))
	for module := range modules {
		names = append(names, module)
	}
	sort.Strings(names)
	return names
}

func swapStringValue(value *atomic.Value, next string) string {
	current, _ := value.Swap(next).(string)
	return current
}

func moduleStateFingerprint(modules []string) string {
	if len(modules) == 0 {
		return ""
	}
	return fmt.Sprintf("%d:%s", len(modules), strings.Join(modules, "|"))
}
