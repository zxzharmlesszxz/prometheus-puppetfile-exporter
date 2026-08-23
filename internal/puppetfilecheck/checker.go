package puppetfilecheck

import (
	"context"
	"maps"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
)

type Checker struct {
	paths    []string
	counters map[string]*fileCounters
}

type fileCounters struct {
	readErrorsTotal  atomic.Uint64
	parseErrorsTotal atomic.Uint64
}

func NewChecker(paths ...string) *Checker {
	if len(paths) == 0 {
		paths = []string{DefaultPuppetfilePath}
	}
	var cleaned []string
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p != "" {
			cleaned = append(cleaned, p)
		}
	}
	if len(cleaned) == 0 {
		cleaned = []string{DefaultPuppetfilePath}
	}
	counters := make(map[string]*fileCounters, len(cleaned))
	for _, path := range cleaned {
		counters[path] = &fileCounters{}
	}
	return &Checker{
		paths:    cleaned,
		counters: counters,
	}
}

func (c *Checker) Snapshot(ctx context.Context, now time.Time) Snapshot {
	var files []FileSnapshot
	var allModules = make(Modules)
	var allUnsupported []string
	var allIncomplete []string
	var allSuccess = true
	var scrapeDuration float64
	var readErrorsTotal uint64
	var parseErrorsTotal uint64

	for _, path := range c.paths {
		fs := c.scrapeFile(ctx, path)
		files = append(files, fs)
		if !fs.Success {
			allSuccess = false
		}
		maps.Copy(allModules, fs.Modules)
		allUnsupported = append(allUnsupported, fs.UnsupportedModules...)
		allIncomplete = append(allIncomplete, fs.IncompleteModules...)
		readErrorsTotal += fs.ReadErrorsTotal
		parseErrorsTotal += fs.ParseErrorsTotal
	}

	sort.Strings(allUnsupported)
	sort.Strings(allIncomplete)

	var overallErr error
	if !allSuccess {
		for _, fs := range files {
			if !fs.Success && fs.Err != nil {
				overallErr = fs.Err
				break
			}
		}
	}

	for _, fs := range files {
		scrapeDuration += fs.ScrapeDurationSeconds
	}

	var firstPath string
	var firstMTime float64
	if len(files) > 0 {
		firstPath = files[0].Path
		firstMTime = files[0].MTimeSeconds
	}

	return Snapshot{
		AttemptTime:           now,
		Success:               allSuccess,
		Err:                   overallErr,
		Path:                  firstPath,
		MTimeSeconds:          firstMTime,
		ReadErrorsTotal:       readErrorsTotal,
		ParseErrorsTotal:      parseErrorsTotal,
		ScrapeDurationSeconds: scrapeDuration,
		Modules:               allModules,
		UnsupportedModules:    allUnsupported,
		IncompleteModules:     allIncomplete,
		Files:                 files,
	}
}

func (c *Checker) scrapeFile(ctx context.Context, path string) FileSnapshot {
	var result ParseResult
	counters := c.counters[path]
	scrape := framework.FileScraper{
		ReadErrorsTotal: &counters.readErrorsTotal,
		ReadFile: func(p string) ([]byte, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return load(p)
		},
	}.Scrape(path, func(content []byte) error {
		result = ParseContent(string(content))
		return nil
	})

	fs := FileSnapshot{
		Path:                  scrape.Path,
		Success:               scrape.Up,
		Err:                   scrape.Err,
		MTimeSeconds:          scrape.MTimeSeconds,
		ReadErrorsTotal:       counters.readErrorsTotal.Load(),
		ParseErrorsTotal:      counters.parseErrorsTotal.Load(),
		ScrapeDurationSeconds: scrape.ScrapeDurationSeconds,
	}

	if scrape.Up {
		fs.Modules = result.Modules
		fs.UnsupportedModules = result.UnsupportedModules
		fs.IncompleteModules = result.IncompleteModules
		fs.Valid = len(result.UnsupportedModules) == 0 && len(result.IncompleteModules) == 0
		if !fs.Valid {
			counters.parseErrorsTotal.Add(1)
			fs.ParseErrorsTotal = counters.parseErrorsTotal.Load()
		}
	}

	return fs
}
