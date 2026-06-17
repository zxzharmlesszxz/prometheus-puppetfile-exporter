package puppetfile

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/exportertest"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/exportertest/featuretest"

	"github.com/zxzharmlesszxz/prometheus-puppetfile-exporter/internal/puppetfilecheck"
)

func TestFeatureContract(t *testing.T) {
	suite := NewFeatureTestSuite(NewFeatureTestSpec())
	RegisterFeatureTests(suite)
	suite.RunTests(t)
}

func NewFeatureTestSpec() FeatureTestSpec {
	puppetfilePath := testPuppetfilePath()
	return FeatureTestSpec{
		SuccessfulSnapshot: func(at time.Time) Snapshot {
			return Snapshot{
				puppetfile: puppetfilecheck.Snapshot{
					AttemptTime: at,
					Success:     true,
					Path:        puppetfilePath,
					Modules: puppetfilecheck.Modules{
						"puppetlabs/apt": {
							Owner: "puppetlabs",
							Name:  "apt",
							Type:  puppetfilecheck.ModuleTypeForge,
						},
					},
					Files: []puppetfilecheck.FileSnapshot{
						{
							Path:    puppetfilePath,
							Success: true,
							Modules: puppetfilecheck.Modules{
								"puppetlabs/apt": {
									Owner: "puppetlabs",
									Name:  "apt",
									Type:  puppetfilecheck.ModuleTypeForge,
								},
							},
						},
					},
				},
			}
		},
		FailedSnapshot: func(at time.Time, err error) Snapshot {
			return Snapshot{
				puppetfile: puppetfilecheck.Snapshot{
					AttemptTime: at,
					Success:     false,
					Err:         err,
					Path:        puppetfilePath,
					Files: []puppetfilecheck.FileSnapshot{
						{
							Path:    puppetfilePath,
							Success: false,
							Err:     err,
						},
					},
				},
			}
		},
		ContractFlagArgs: []string{
			"--" + testFeatureName + ".puppetfile-path=" + puppetfilePath,
		},
		ContractRuntimeConfig: map[string]any{
			"puppetfile_path": puppetfilePath,
		},

		CollectorFlagArgs: []string{
			"--" + testFeatureName + ".puppetfile-path=" + puppetfilePath,
		},
		DefaultRuntimeConfig: map[string]any{
			"puppetfile_path": DefaultPuppetfilePath,
		},
	}
}

func RegisterFeatureTests(suite *FeatureTestSuite) {
	suite.Register("collector_exports_puppetfile_metrics", func(t *testing.T) { testCollectorExportsPuppetfileMetrics(t, suite) })
	suite.Register("collector_reads_puppetfile_fixture", func(t *testing.T) { testCollectorReadsPuppetfileFixture(t, suite) })
	suite.Register("collector_reports_failed_snapshot", func(t *testing.T) { testCollectorReportsFailedSnapshot(t, suite) })
	suite.Register("collector_logs_snapshot_diagnostics", func(t *testing.T) { testCollectorLogsSnapshotDiagnostics(t, suite) })
	suite.Register("snapshot_engine_tracks_read_errors", func(t *testing.T) { testSnapshotEngineTracksReadErrors(t, suite) })
	suite.Register("exporter_registers_collector_with_fixture", func(t *testing.T) { testExporterRegistersCollectorWithFixture(t, suite) })
	suite.Register("exporter_runtime_config_normalizes_values", func(t *testing.T) { testExporterRuntimeConfigNormalizesValues(t, suite) })
	suite.Register("smoke_spec_includes_fixture_args", func(t *testing.T) { testSmokeSpecIncludesFixtureArgs(t, suite) })
}

func testCollectorExportsPuppetfileMetrics(t *testing.T, suite *FeatureTestSuite) {
	now := time.Unix(1700000000, 0)
	snapshotter := suite.NewFakeSnapshotter(Snapshot{})
	snapshotter.Set(Snapshot{
		puppetfile: puppetfilecheck.Snapshot{
			AttemptTime:           now,
			Success:               true,
			Path:                  testPuppetfilePath(),
			MTimeSeconds:          1000,
			ReadErrorsTotal:       2,
			ScrapeDurationSeconds: 0.25,
			Modules: puppetfilecheck.Modules{
				"puppetlabs/apt": {
					Owner:   "puppetlabs",
					Name:    "apt",
					Version: "11.2.0",
					Type:    puppetfilecheck.ModuleTypeForge,
				},
			},
			UnsupportedModules: []string{"example/local@line:1"},
			IncompleteModules:  []string{"broken-module@line:3"},
			Files: []puppetfilecheck.FileSnapshot{
				{
					Path:                  testPuppetfilePath(),
					Success:               true,
					Valid:                 false,
					MTimeSeconds:          1000,
					ParseErrorsTotal:      1,
					ScrapeDurationSeconds: 0.25,
					Modules: puppetfilecheck.Modules{
						"puppetlabs/apt": {
							Owner:   "puppetlabs",
							Name:    "apt",
							Version: "11.2.0",
							Type:    puppetfilecheck.ModuleTypeForge,
						},
					},
					UnsupportedModules: []string{"example/local@line:1"},
					IncompleteModules:  []string{"broken-module@line:3"},
				},
			},
		},
	})
	collector := suite.NewCollectorWithNow(
		testFeatureName,
		testMetricNamespace,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		snapshotter,
		testRefreshInterval,
		func() time.Time { return now },
	)

	families := exportertest.RegisterAndGather(t, collector)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricUp), nil, 1)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricValid), nil, 0)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricMTime), nil, 1000)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricReadErrors), nil, 2)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricParseErrors), nil, 1)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricScrapeDuration), nil, 0.25)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricModulesDiscovered), nil, 1)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricUnsupportedModules), nil, 1)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricIncompleteModules), nil, 1)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricFilesMonitored), nil, 1)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricDeclaredModulesTotal), nil, 3)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricModuleStatus), map[string]string{"module": "puppetlabs/apt"}, 1)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricModuleStatus), map[string]string{"module": "example/local"}, 0)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricModuleStatus), map[string]string{"module": "broken-module"}, 0)
	exportertest.AssertMetricValue(t, families, testLastSuccess, nil, 1)
	exportertest.AssertMetricValue(t, families, testLastTimestamp, nil, float64(now.Unix()))
	exportertest.AssertMetricValue(t, families, testLastSuccessfulTS, nil, float64(now.Unix()))

	metric := singleMetric(t, families, suite.MetricName(testFeatureName, "", metricModuleInfo))
	labels := metricLabels(metric)
	if labels["module"] != "puppetlabs/apt" || labels["owner"] != "puppetlabs" || labels["name"] != "apt" {
		t.Fatalf("unexpected module labels: %#v", labels)
	}
	if labels["current_version"] != "11.2.0" || labels["type"] != "forge" {
		t.Fatalf("unexpected version labels: %#v", labels)
	}
}

func testCollectorReadsPuppetfileFixture(t *testing.T, suite *FeatureTestSuite) {
	checker := puppetfilecheck.NewChecker(testPuppetfilePath())
	snapshot := checker.Snapshot(t.Context(), time.Now())
	collector := suite.NewCollector(
		testFeatureName,
		testMetricNamespace,
		suite.NewFakeSnapshotter(Snapshot{puppetfile: snapshot}),
		testRefreshInterval,
	)

	families := exportertest.RegisterAndGather(t, collector)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricUp), nil, 1)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricValid), nil, 0)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricFilesMonitored), nil, 1)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricParseErrors), nil, 1)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricModulesDiscovered), nil, 7)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricUnsupportedModules), nil, 1)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricIncompleteModules), nil, 3)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricDeclaredModulesTotal), nil, 11)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricModuleStatus), map[string]string{"module": "puppetlabs/apt"}, 1)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricModuleStatus), map[string]string{"module": "example/local"}, 0)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricModuleStatus), map[string]string{"module": "broken-module"}, 0)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricModuleStatus), map[string]string{"module": "puppetlabs/apache"}, 0)
}

func testCollectorReportsFailedSnapshot(t *testing.T, suite *FeatureTestSuite) {
	now := time.Unix(1700000000, 0)
	collector := suite.NewCollectorWithNow(
		testFeatureName,
		testMetricNamespace,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		suite.NewFakeSnapshotter(Snapshot{
			puppetfile: puppetfilecheck.Snapshot{
				AttemptTime:           now,
				Success:               false,
				Err:                   errors.New("read failed"),
				Path:                  testPuppetfilePath(),
				ReadErrorsTotal:       1,
				ScrapeDurationSeconds: 0.01,
				Files: []puppetfilecheck.FileSnapshot{
					{
						Path:                  testPuppetfilePath(),
						Success:               false,
						Err:                   errors.New("read failed"),
						ScrapeDurationSeconds: 0.01,
					},
				},
			},
		}),
		testRefreshInterval,
		func() time.Time { return now },
	)

	families := exportertest.RegisterAndGather(t, collector)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricUp), nil, 0)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricValid), nil, 0)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricFilesMonitored), nil, 1)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricReadErrors), nil, 1)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricParseErrors), nil, 0)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricModulesDiscovered), nil, 0)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricUnsupportedModules), nil, 0)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricIncompleteModules), nil, 0)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricDeclaredModulesTotal), nil, 0)
	exportertest.AssertMetricValue(t, families, testLastSuccess, nil, 0)
}

func testCollectorLogsSnapshotDiagnostics(t *testing.T, suite *FeatureTestSuite) {
	logger, logs := newBufferedLogger(slog.LevelDebug)
	collector := suite.NewCollectorWithNow(
		testFeatureName,
		testMetricNamespace,
		logger,
		suite.NewFakeSnapshotter(Snapshot{
			puppetfile: puppetfilecheck.Snapshot{
				AttemptTime:        time.Unix(1700000000, 0),
				Success:            true,
				Path:               testPuppetfilePath(),
				UnsupportedModules: []string{"example/local@line:1"},
				IncompleteModules:  []string{"broken-module@line:3"},
				Files: []puppetfilecheck.FileSnapshot{
					{
						Path:               testPuppetfilePath(),
						Success:            true,
						UnsupportedModules: []string{"example/local@line:1"},
						IncompleteModules:  []string{"broken-module@line:3"},
					},
				},
			},
		}),
		testRefreshInterval,
		func() time.Time { return time.Unix(1700000000, 0) },
	)

	_ = exportertest.RegisterAndGather(t, collector)
	output := logs.String()
	for _, want := range []string{
		"skipping unsupported Puppetfile modules",
		"skipping incomplete Puppetfile modules",
		"no modules discovered in Puppetfile",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("logs missing %q:\n%s", want, output)
		}
	}
}

func testSnapshotEngineTracksReadErrors(t *testing.T, suite *FeatureTestSuite) {
	engine := puppetfilecheck.NewChecker(filepath.Join(t.TempDir(), "missing-Puppetfile"))
	first := engine.Snapshot(t.Context(), time.Unix(1700000000, 0))
	second := engine.Snapshot(t.Context(), time.Unix(1700000001, 0))

	if first.Success || second.Success {
		t.Fatalf("Snapshot() success = %v, %v; want false, false", first.Success, second.Success)
	}
	if first.ReadErrorsTotal != 1 || second.ReadErrorsTotal != 2 {
		t.Fatalf("ReadErrorsTotal = %d, %d; want 1, 2", first.ReadErrorsTotal, second.ReadErrorsTotal)
	}
}

func testExporterRegistersCollectorWithFixture(t *testing.T, suite *FeatureTestSuite) {
	exporter := suite.NewNamedFeature()
	suite.ParseFeatureFlags(t, exporter, []string{"--" + testFeatureName + ".config-file=" + suite.WriteConfig(t, `
puppetfile_path: `+testPuppetfilePath()+`
`)})

	registry := suite.RegisterFeatureCollectors(t, exporter)
	exportertest.WaitForMetricValue(t, registry, testLastSuccess, nil, 1)
	families := exportertest.Gather(t, registry)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricModulesDiscovered), nil, 7)
}

func testExporterRuntimeConfigNormalizesValues(t *testing.T, suite *FeatureTestSuite) {
	exporter := suite.NewNamedFeature()
	suite.ParseFeatureFlags(t, exporter, []string{
		"--" + testFeatureName + ".refresh-interval=0s",
		"--" + testFeatureName + ".puppetfile-path=",
	})

	config := exporter.RuntimeConfig()
	if got := exportertest.RuntimeConfigValue(t, config, "refresh_interval"); got != DefaultRefreshInterval {
		t.Fatalf("refresh_interval = %v, want %v", got, DefaultRefreshInterval)
	}
	if got := exportertest.RuntimeConfigValue(t, config, "puppetfile_path"); got != DefaultPuppetfilePath {
		t.Fatalf("puppetfile_path = %q, want %q", got, DefaultPuppetfilePath)
	}
}

func testSmokeSpecIncludesFixtureArgs(t *testing.T, suite *FeatureTestSuite) {
	spec := suite.NewNamedFeature().SmokeSpec()
	for _, want := range []string{
		"--" + testFeatureName + ".config-file=../examples/" + DefaultFeatureConfigFileName,
		"--" + testFeatureName + ".puppetfile-path=testdata/Puppetfile",
	} {
		if !featuretest.HasString(spec.ServerArgs, want) {
			t.Fatalf("SmokeSpec().ServerArgs = %v, want %q", spec.ServerArgs, want)
		}
	}
	if !featuretest.HasString(spec.WantMetrics, suite.MetricName(testFeatureName, "", metricUp)+`{path="testdata/Puppetfile"} 1`) {
		t.Fatalf("SmokeSpec().WantMetrics = %v, want up metric", spec.WantMetrics)
	}
	if !featuretest.HasString(spec.WantMetrics, suite.MetricName(testFeatureName, "", metricValid)+`{path="testdata/Puppetfile"} 0`) {
		t.Fatalf("SmokeSpec().WantMetrics = %v, want valid metric", spec.WantMetrics)
	}
	if !featuretest.HasString(spec.RejectMetrics, suite.MetricName(testFeatureName, "", metricUp)+`{path="testdata/Puppetfile"} 0`) {
		t.Fatalf("SmokeSpec().RejectMetrics = %v, want up rejection", spec.RejectMetrics)
	}
}

func singleMetric(t *testing.T, families []*dto.MetricFamily, name string) *dto.Metric {
	t.Helper()

	family := exportertest.MetricFamily(t, families, name)
	if len(family.Metric) != 1 {
		t.Fatalf("%s metric count = %d, want 1", name, len(family.Metric))
	}
	return family.Metric[0]
}

func metricLabels(metric *dto.Metric) map[string]string {
	labels := make(map[string]string, len(metric.Label))
	for _, label := range metric.Label {
		labels[label.GetName()] = label.GetValue()
	}
	return labels
}

func newBufferedLogger(level slog.Level) (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: level})
	return slog.New(handler), &buf
}

func testPuppetfilePath() string {
	return filepath.Join("..", "puppetfilecheck", "testdata", "Puppetfile")
}
