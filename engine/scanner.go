package engine

import (
	"time"

	"github.com/maharabhossain1/devclean/scanners/dev_caches"
)

// RunScan is the top-level Phase 1 scan: build registry, scan traces, correlate, score.
func RunScan(home string) (*ScanResult, error) {
	dict, err := LoadDictionary()
	if err != nil {
		return nil, err
	}

	apps, err := BuildAppRegistry()
	if err != nil {
		return nil, err
	}

	rawTraces, err := ScanTraces(home)
	if err != nil {
		return nil, err
	}

	correlator := NewCorrelator(apps, dict, home)

	result := &ScanResult{}

	for _, raw := range rawTraces {
		t := correlator.Correlate(raw)
		ScoreTrace(&t)

		switch t.Status {
		case StatusOrphaned:
			result.Orphans = append(result.Orphans, t)
		case StatusDeadAgent:
			result.DeadAgents = append(result.DeadAgents, t)
		case StatusUnknown:
			result.Unknowns = append(result.Unknowns, t)
		// StatusOwned traces are silently dropped from results
		}
	}

	// Dev caches (always regenerable, no correlation needed)
	caches := dev_caches.Scan(home)
	for _, c := range caches {
		result.DevCaches = append(result.DevCaches, Trace{
			Path:     c.Path,
			Category: "dev_cache",
			Status:   StatusOrphaned,
			Risk:     RiskGreen,
		})
	}

	result.UnusedApps = unusedApps(apps, 90)

	return result, nil
}

func unusedApps(apps []AppEntry, thresholdDays int) []AppEntry {
	var unused []AppEntry
	for _, app := range apps {
		if app.Source != SourceApplicationsDir {
			continue
		}
		if app.SizeBytes < 500*1024*1024 {
			continue
		}
		if app.LastLaunch.IsZero() {
			unused = append(unused, app)
			continue
		}
		if int(time.Since(app.LastLaunch).Hours()/24) >= thresholdDays {
			unused = append(unused, app)
		}
	}
	return unused
}
