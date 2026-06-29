package replay

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ritikraj2425/agentsandbox/internal/trace"
)

type Run struct {
	ID         string             `json:"id"`
	Events     []trace.TraceEvent `json:"events"`
	Stdout     string             `json:"stdout"`
	Stderr     string             `json:"stderr"`
	Report     interface{}        `json:"report,omitempty"`
	CreatedAt  time.Time          `json:"created_at"`
	DurationMs int64              `json:"duration_ms"`
	ExitCode   int                `json:"exit_code"`
}

type RunSummary struct {
	ID         string    `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	EventCount int       `json:"event_count"`
	DurationMs int64     `json:"duration_ms"`
	ExitCode   *int      `json:"exit_code"`
	Status     string    `json:"status"`
}

func ListRuns(baseDir string) ([]RunSummary, error) {
	runsDir := filepath.Join(baseDir, ".agentsandbox", "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []RunSummary{}, nil
		}
		return nil, err
	}

	var summaries []RunSummary
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		runPath := filepath.Join(runsDir, id)

		info, err := entry.Info()
		var createdAt time.Time
		if err == nil {
			createdAt = info.ModTime()
		} else {
			createdAt = time.Now()
		}

		tracePath := filepath.Join(runPath, "trace.jsonl")
		f, err := os.Open(tracePath)
		var eventCount int
		var durationMs int64
		var exitCode *int
		if err == nil {
			scanner := bufio.NewScanner(f)
			var firstTime, lastTime time.Time
			for scanner.Scan() {
				var ev trace.TraceEvent
				if json.Unmarshal(scanner.Bytes(), &ev) == nil {
					if eventCount == 0 {
						firstTime = ev.Timestamp
					}
					lastTime = ev.Timestamp
					eventCount++
					
					if ev.Type == trace.EventTypeProcessFinished && ev.Data != nil {
						if code, ok := ev.Data["exit_code"].(float64); ok {
							val := int(code)
							exitCode = &val
						}
					}
				}
			}
			f.Close()
			if eventCount > 0 {
				durationMs = lastTime.Sub(firstTime).Milliseconds()
			}
		}

		status := "completed"
		if exitCode != nil && *exitCode != 0 {
			status = "failed"
		}

		summaries = append(summaries, RunSummary{
			ID:         id,
			CreatedAt:  createdAt,
			EventCount: eventCount,
			DurationMs: durationMs,
			ExitCode:   exitCode,
			Status:     status,
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CreatedAt.After(summaries[j].CreatedAt)
	})

	return summaries, nil
}

func LoadRun(baseDir, runID string) (*Run, error) {
	runPath := filepath.Join(baseDir, ".agentsandbox", "runs", runID)

	info, err := os.Stat(runPath)
	if err != nil {
		return nil, fmt.Errorf("run not found: %w", err)
	}

	run := &Run{
		ID:        runID,
		CreatedAt: info.ModTime(),
	}

	// Load traces
	tracePath := filepath.Join(runPath, "trace.jsonl")
	f, err := os.Open(tracePath)
	if err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			var ev trace.TraceEvent
			if err := json.Unmarshal(scanner.Bytes(), &ev); err == nil {
				run.Events = append(run.Events, ev)
				if ev.Type == trace.EventTypeProcessFinished && ev.Data != nil {
					if code, ok := ev.Data["exit_code"].(float64); ok {
						run.ExitCode = int(code)
					}
				}
			}
		}
		if len(run.Events) > 0 {
			first := run.Events[0].Timestamp
			last := run.Events[len(run.Events)-1].Timestamp
			run.DurationMs = last.Sub(first).Milliseconds()
		}
	}

	// Load stdout
	stdoutPath := filepath.Join(runPath, "stdout.log")
	b, err := ioutil.ReadFile(stdoutPath)
	if err == nil {
		run.Stdout = string(b)
	}

	// Load stderr
	stderrPath := filepath.Join(runPath, "stderr.log")
	b, err = ioutil.ReadFile(stderrPath)
	if err == nil {
		run.Stderr = string(b)
	}

	// Load report
	reportPath := filepath.Join(runPath, "report.json")
	b, err = ioutil.ReadFile(reportPath)
	if err == nil {
		var report interface{}
		if json.Unmarshal(b, &report) == nil {
			run.Report = report
		}
	}

	return run, nil
}
