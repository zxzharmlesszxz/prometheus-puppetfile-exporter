package puppetfile

import "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"

func FeatureSmoke(ctx featurekit.SmokeContext[Config]) featurekit.SmokeSpec {
	puppetfilePathLabel := `{path="testdata/Puppetfile"}`

	return featurekit.SmokeSpec{
		ServerArgs: []string{
			"--" + ctx.FeatureName + ".config-file=../examples/" + DefaultFeatureConfigFileName,
			"--" + ctx.FeatureName + ".puppetfile-path=testdata/Puppetfile",
		},
		WantMetrics: []string{
			featurekit.FeatureMetricName(ctx.FeatureName, "", metricUp, featureMetricSpecs) + puppetfilePathLabel + " 1",
			featurekit.FeatureMetricName(ctx.FeatureName, "", metricValid, featureMetricSpecs) + puppetfilePathLabel + " 0",
			featurekit.FeatureMetricName(ctx.FeatureName, "", metricParseErrors, featureMetricSpecs),
			featurekit.FeatureMetricName(ctx.FeatureName, "", metricFilesMonitored, featureMetricSpecs) + " 1",
			featurekit.FeatureMetricName(ctx.FeatureName, "", metricModulesDiscovered, featureMetricSpecs),
			featurekit.FeatureMetricName(ctx.FeatureName, "", metricDeclaredModulesTotal, featureMetricSpecs),
			featurekit.FeatureMetricName(ctx.FeatureName, "", metricModuleInfo, featureMetricSpecs),
			featurekit.FeatureMetricName(ctx.FeatureName, "", metricModuleStatus, featureMetricSpecs),
		},
		RejectMetrics: []string{
			featurekit.FeatureMetricName(ctx.FeatureName, "", metricUp, featureMetricSpecs) + puppetfilePathLabel + " 0",
		},
	}
}
