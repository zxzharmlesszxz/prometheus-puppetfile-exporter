package puppetfile

import "github.com/zxzharmlesszxz/prometheus-puppetfile-exporter/internal/puppetfilecheck"

type Snapshot struct {
	puppetfile puppetfilecheck.Snapshot
}
