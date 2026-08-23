package puppetfile

import (
	"context"
	"time"

	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"

	"github.com/zxzharmlesszxz/prometheus-puppetfile-exporter/internal/puppetfilecheck"
)

func NewDefaultSnapshotEngine() featurekit.SnapshotEngine[Snapshot] {
	return newSnapshotEngine(NewDefaultConfig())
}

func NewSnapshotEngine(ctx featurekit.CollectorContext[Config]) (featurekit.SnapshotEngine[Snapshot], error) {
	return newSnapshotEngine(ctx.Config), nil
}

func FeatureSnapshotStatus(snapshot Snapshot) framework.SnapshotStatus {
	return framework.SnapshotStatus{
		AttemptTime: snapshot.puppetfile.AttemptTime,
		Success:     snapshot.puppetfile.Success,
	}
}

func newSnapshotEngine(config Config) featurekit.SnapshotEngine[Snapshot] {
	checker := puppetfilecheck.NewChecker(config.PuppetfilePaths...)
	return featurekit.SnapshotEngineFunc[Snapshot](func(ctx context.Context, now time.Time) Snapshot {
		return Snapshot{
			puppetfile: checker.Snapshot(ctx, now),
		}
	})
}
