package puppetfilecheck

import (
	"context"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
)

type Checker struct {
	Paths            []string
	readErrorsTotal  *atomic.Uint64
	parseErrorsTotal *atomic.Uint64
}

func NewChecker(paths ...string) Checker {
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
	return Checker{
		Paths:            cleaned,
		readErrorsTotal:  &atomic.Uint64{},
		parseErrorsTotal: &atomic.Uint64{},
	}
}

func (c Checker) Path() string {
	if len(c.Paths) == 0 {
		return ""
	}
	return c.Paths[0]
}

func (c Checker) Snapshot(ctx context.Context, now time.Time) Snapshot {
	var files []FileSnapshot
	var allModules = make(Modules)
	var allUnsupported []string
	var allIncomplete []string
	var allSuccess = true
	var scrapeDuration float64

	for _, path := range c.Paths {
		fs := c.scrapeFile(ctx, now, path)
		files = append(files, fs)
		if !fs.Success {
			allSuccess = false
		}
		for k, v := range fs.Modules {
			allModules[k] = v
		}
		allUnsupported = append(allUnsupported, fs.UnsupportedModules...)
		allIncomplete = append(allIncomplete, fs.IncompleteModules...)
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
		ReadErrorsTotal:       c.readErrorsTotal.Load(),
		ParseErrorsTotal:      c.parseErrorsTotal.Load(),
		ScrapeDurationSeconds: scrapeDuration,
		Modules:               allModules,
		UnsupportedModules:    allUnsupported,
		IncompleteModules:     allIncomplete,
		Files:                 files,
	}
}

func (c Checker) scrapeFile(ctx context.Context, now time.Time, path string) FileSnapshot {
	var result ParseResult
	scrape := framework.FileScraper{
		ReadErrorsTotal: c.readErrorsTotal,
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
		ParseErrorsTotal:      c.parseErrorsTotal.Load(),
		ScrapeDurationSeconds: scrape.ScrapeDurationSeconds,
	}

	if scrape.Up {
		fs.Modules = result.Modules
		fs.UnsupportedModules = result.UnsupportedModules
		fs.IncompleteModules = result.IncompleteModules
		fs.Valid = len(result.UnsupportedModules) == 0 && len(result.IncompleteModules) == 0
		if !fs.Valid {
			c.parseErrorsTotal.Add(1)
			fs.ParseErrorsTotal = c.parseErrorsTotal.Load()
		}
	}

	return fs
}
