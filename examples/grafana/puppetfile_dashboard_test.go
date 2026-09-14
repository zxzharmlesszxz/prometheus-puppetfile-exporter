package grafana_test

import (
	"os"
	"strings"
	"testing"
)

func TestPuppetfileDashboardCoversLastKnownGoodMetrics(t *testing.T) {
	data, err := os.ReadFile("prometheus-puppetfile-exporter.json")
	if err != nil {
		t.Fatal(err)
	}

	dashboard := string(data)
	for _, metric := range []string{
		"puppetfile_source_last_success_timestamp_seconds",
		"puppetfile_source_consecutive_failures",
		"puppetfile_source_data_available",
		"puppetfile_source_data_stale",
	} {
		if !strings.Contains(dashboard, metric) {
			t.Errorf("dashboard does not reference %s", metric)
		}
	}
}
