// Package benchmark provides performance benchmarking for sandbox operations.
package benchmark

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/ritikraj2425/agentsandbox/internal/color"
	"github.com/ritikraj2425/agentsandbox/internal/runtime"
	"github.com/ritikraj2425/agentsandbox/pkg/protocol"
)

// Metric captures the performance of a single sandbox operation.
type Metric struct {
	InitTimeMs int64
	ExecTimeMs int64
	TotalMs    int64
	Error      error
}

// RunBenchmark executes a load test against the specified runtime backend.
func RunBenchmark(backendName string, factory func(string) (runtime.Runtime, error), workDir string, concurrent int) error {
	fmt.Printf("\n%s\n", color.BoldCyan("━━━ AgentSandbox Benchmark Suite ━━━"))
	fmt.Printf("Testing backend: %s\n", color.Bold(backendName))
	fmt.Printf("Concurrency: %d sessions\n\n", concurrent)

	var wg sync.WaitGroup
	metrics := make(chan Metric, concurrent)

	fmt.Printf("Spawning %d concurrent sandbox executions...\n", concurrent)
	
	// Warmup run to pull images (especially for Docker/gVisor)
	fmt.Printf("Running 1 warmup execution to pull images...\n")
	warmupRt, _ := factory(workDir)
	if warmupRt != nil {
		warmupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		warmupRt.Run(warmupCtx, protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{
			"command": "echo warmup",
		}))
		cancel()
	}
	
	startTime := time.Now()

	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			m := Metric{}
			
			tInitStart := time.Now()
			// Init runtime for this session
			rt, err := factory(workDir)
			m.InitTimeMs = time.Since(tInitStart).Milliseconds()
			
			if err != nil {
				m.Error = fmt.Errorf("init failed: %w", err)
				metrics <- m
				return
			}
			
			action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{
				"command": "echo benchmark_ready",
			})
			
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			
			tExecStart := time.Now()
			obs, err := rt.Run(ctx, action)
			m.ExecTimeMs = time.Since(tExecStart).Milliseconds()
			
			if err != nil {
				m.Error = fmt.Errorf("exec failed: %w", err)
			} else if obs.ExitCode != 0 {
				m.Error = fmt.Errorf("non-zero exit: %d, reason: %s", obs.ExitCode, obs.Error)
			}
			
			m.TotalMs = time.Since(tInitStart).Milliseconds()
			metrics <- m
		}(i)
	}

	wg.Wait()
	close(metrics)
	totalWallTime := time.Since(startTime)

	var results []Metric
	var errorList []error
	errors := 0
	for m := range metrics {
		if m.Error != nil {
			errors++
			errorList = append(errorList, m.Error)
		} else {
			results = append(results, m)
		}
	}

	fmt.Printf("\n%s\n", color.BoldCyan("━━━ Results ━━━"))
	fmt.Printf("Total Wall Time: %s\n", totalWallTime)
	fmt.Printf("Successful Runs: %d/%d\n", len(results), concurrent)
	fmt.Printf("Failed Runs: %d\n", errors)
	
	if len(errorList) > 0 {
		fmt.Printf("First Error: %s\n", color.Red(errorList[0].Error()))
	}

	if len(results) > 0 {
		reportPath := filepath.Join(workDir, "benchmark_report.md")
		if err := writeReport(reportPath, backendName, concurrent, results, totalWallTime); err != nil {
			fmt.Printf("Failed to write report: %s\n", err)
		} else {
			fmt.Printf("Report saved to: %s\n", color.Dim(reportPath))
		}
	}

	return nil
}

func writeReport(path string, backend string, concurrent int, results []Metric, totalTime time.Duration) error {
	sort.Slice(results, func(i, j int) bool {
		return results[i].TotalMs < results[j].TotalMs
	})

	var sumInit, sumExec, sumTotal float64
	for _, m := range results {
		sumInit += float64(m.InitTimeMs)
		sumExec += float64(m.ExecTimeMs)
		sumTotal += float64(m.TotalMs)
	}
	
	count := float64(len(results))
	avgInit := sumInit / count
	avgExec := sumExec / count
	avgTotal := sumTotal / count
	
	p50 := results[int(float64(len(results))*0.5)].TotalMs
	p95 := results[int(float64(len(results))*0.95)].TotalMs
	max := results[len(results)-1].TotalMs

	content := fmt.Sprintf("# AgentSandbox Benchmark Report\n\n"+
		"## Configuration\n"+
		"- **Backend**: `%s`\n"+
		"- **Concurrency**: `%d` parallel sessions\n"+
		"- **Wall Clock Time**: `%s`\n"+
		"- **Success Rate**: `%d/%d` (%.1f%%)\n\n"+
		"## Latency Distribution (Total Ms)\n"+
		"| Metric | Average | P50 (Median) | P95 | Max |\n"+
		"|---|---|---|---|---|\n"+
		"| Init Time | %.1fms | - | - | - |\n"+
		"| Exec Time | %.1fms | - | - | - |\n"+
		"| **Total Time** | **%.1fms** | **%dms** | **%dms** | **%dms** |\n\n",
		backend, concurrent, totalTime.String(), len(results), concurrent, (float64(len(results))/float64(concurrent))*100,
		avgInit, avgExec, avgTotal, p50, p95, max,
	)

	return os.WriteFile(path, []byte(content), 0644)
}
