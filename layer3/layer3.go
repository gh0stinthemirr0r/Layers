// Package layer3 implements network layer testing functionality
package layer3

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"go.uber.org/zap"

	"ghostshell/app/layers/common"
)

// Runner implements network layer tests
type Runner struct {
	*common.Layer3Runner
}

// New creates a new Layer3Runner
func New(hostname string, pingAddr string, pingCount int) *Runner {
	return &Runner{
		Layer3Runner: &common.Layer3Runner{
			Hostname:  hostname,
			PingAddr:  pingAddr,
			PingCount: pingCount,
		},
	}
}

// RunTests implements the LayerRunner interface
func (r *Runner) RunTests(ctx context.Context, logger *zap.Logger) ([]common.TestResult, error) {
	logger.Info("Starting Layer 3 (Network Layer) tests...",
		zap.String("hostname", r.Hostname),
		zap.String("ping_addr", r.PingAddr),
		zap.Int("ping_count", r.PingCount))

	startTime := time.Now()

	// Create parent result
	parentResult := common.TestResult{
		Layer:      3,
		Name:       "Network Layer Tests",
		StartTime:  startTime,
		SubResults: []common.TestResult{},
	}

	select {
	case <-ctx.Done():
		parentResult.Status = common.StatusFailed
		parentResult.Message = "Test cancelled"
		parentResult.EndTime = time.Now()
		return []common.TestResult{parentResult}, ctx.Err()
	default:
		var failedTests []string

		// Run ping test
		pingResult := common.TestResult{
			Layer:     3,
			Name:      fmt.Sprintf("Ping Test (%s)", r.PingAddr),
			StartTime: time.Now(),
		}

		output, err := runPing(r.PingAddr, r.PingCount)
		if err != nil {
			pingResult.Status = common.StatusFailed
			pingResult.Message = fmt.Sprintf("Ping test failed: %v\nOutput: %s", err, output)
			failedTests = append(failedTests, pingResult.Message)
		} else {
			pingResult.Status = common.StatusPassed
			pingResult.Message = fmt.Sprintf("Ping test successful:\n%s", output)
		}
		pingResult.EndTime = time.Now()
		parentResult.SubResults = append(parentResult.SubResults, pingResult)

		// DNS resolution test
		dnsResult := common.TestResult{
			Layer:     3,
			Name:      fmt.Sprintf("DNS Resolution Test (%s)", r.Hostname),
			StartTime: time.Now(),
		}

		addrs, err := net.LookupHost(r.Hostname)
		if err != nil {
			dnsResult.Status = common.StatusFailed
			dnsResult.Message = fmt.Sprintf("DNS resolution failed for %s: %v", r.Hostname, err)
			failedTests = append(failedTests, dnsResult.Message)
		} else {
			dnsResult.Status = common.StatusPassed
			dnsResult.Message = fmt.Sprintf("DNS resolution successful for %s:\n- Resolved addresses: %v",
				r.Hostname, addrs)
		}
		dnsResult.EndTime = time.Now()
		parentResult.SubResults = append(parentResult.SubResults, dnsResult)

		// Set overall test status and message
		if len(failedTests) > 0 {
			parentResult.Status = common.StatusFailed
			parentResult.Message = fmt.Sprintf("Layer 3 tests failed with %d failures:\n\n%s",
				len(failedTests), strings.Join(failedTests, "\n\n"))
			logger.Error(parentResult.Message)
			parentResult.EndTime = time.Now()
			return []common.TestResult{parentResult}, fmt.Errorf("layer 3 tests failed")
		}

		parentResult.Status = common.StatusPassed
		parentResult.Message = fmt.Sprintf("All Layer 3 tests passed successfully:\n"+
			"- Ping test to %s completed successfully\n"+
			"- DNS resolution for %s successful",
			r.PingAddr, r.Hostname)
		logger.Info(parentResult.Message)
		parentResult.EndTime = time.Now()
		return []common.TestResult{parentResult}, nil
	}
}

// runPing executes the ping command appropriate for the OS
func runPing(ip string, count int) (string, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("ping", "-n", fmt.Sprintf("%d", count), ip)
	} else {
		cmd = exec.Command("ping", "-c", fmt.Sprintf("%d", count), ip)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ping failed: %v - %s", err, string(output))
	}

	// Extract relevant parts of the ping output
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	var relevantLines []string
	for _, line := range lines {
		if strings.Contains(line, "time=") || strings.Contains(line, "statistics") {
			relevantLines = append(relevantLines, strings.TrimSpace(line))
		}
	}

	return strings.Join(relevantLines, "\n"), nil
}

// GetDependencies returns the layer numbers this layer depends on
func (r *Runner) GetDependencies() []int {
	return []int{1, 2} // Layer 3 depends on Layers 1 and 2
}

// ValidateConfig validates the configuration for this layer
func (r *Runner) ValidateConfig() error {
	if r.Hostname == "" {
		return fmt.Errorf("hostname must be specified")
	}
	if r.PingAddr == "" {
		return fmt.Errorf("ping address must be specified")
	}
	if r.PingCount <= 0 {
		return fmt.Errorf("ping count must be greater than 0")
	}
	return nil
}

// GetDescription returns a description of this layer's functionality
func (r *Runner) GetDescription() string {
	return "Tests network layer functionality including IP addressing and routing"
}

// GetName returns the name of this layer
func (r *Runner) GetName() string {
	return "Network Layer"
}
