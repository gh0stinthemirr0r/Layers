// Package layer5 implements session layer testing functionality
package layer5

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"go.uber.org/zap"

	"ghostshell/app/layers/common"
)

// Runner implements session layer tests
type Runner struct {
	*common.Layer5Runner
}

// New creates a new Layer5Runner
func New(targets []string, timeout time.Duration) *Runner {
	return &Runner{
		Layer5Runner: &common.Layer5Runner{
			Targets: targets,
			Timeout: timeout,
		},
	}
}

// RunTests implements the LayerRunner interface
func (r *Runner) RunTests(ctx context.Context, logger *zap.Logger) ([]common.TestResult, error) {
	logger.Info("Starting Layer 5 (Session Layer) tests...",
		zap.Strings("targets", r.Targets),
		zap.Duration("timeout", r.Timeout))

	startTime := time.Now()

	// Create parent result
	parentResult := common.TestResult{
		Layer:      5,
		Name:       "Session Layer Tests",
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

		// Test session establishment with each target
		for _, target := range r.Targets {
			sessionResult := common.TestResult{
				Layer:     5,
				Name:      fmt.Sprintf("Session Establishment Test (%s)", target),
				StartTime: time.Now(),
			}

			success, msg, details := testSessionEstablishment(target, r.Timeout)
			if !success {
				sessionResult.Status = common.StatusFailed
				sessionResult.Message = msg
				failedTests = append(failedTests, msg)
			} else {
				sessionResult.Status = common.StatusPassed
				sessionResult.Message = msg
			}

			// Add detailed diagnostics
			sessionResult.Diagnostics = details
			sessionResult.EndTime = time.Now()
			sessionResult.Metrics.Duration = sessionResult.EndTime.Sub(sessionResult.StartTime)
			parentResult.SubResults = append(parentResult.SubResults, sessionResult)
		}

		// Set overall test status and message
		if len(failedTests) > 0 {
			parentResult.Status = common.StatusFailed
			parentResult.Message = fmt.Sprintf("Layer 5 tests failed with %d failures:\n\n%s",
				len(failedTests), strings.Join(failedTests, "\n\n"))
			logger.Error(parentResult.Message)
		} else {
			parentResult.Status = common.StatusPassed
			parentResult.Message = fmt.Sprintf("All Layer 5 tests passed successfully:\n"+
				"- Session establishments tested: %d\n"+
				"- Tested targets: %s",
				len(r.Targets), strings.Join(r.Targets, ", "))
			logger.Info(parentResult.Message)
		}

		parentResult.EndTime = time.Now()
		parentResult.Metrics.Duration = parentResult.EndTime.Sub(parentResult.StartTime)

		if len(failedTests) > 0 {
			return []common.TestResult{parentResult}, fmt.Errorf("layer 5 tests failed")
		}
		return []common.TestResult{parentResult}, nil
	}
}

// testSessionEstablishment attempts to establish a session with the target
func testSessionEstablishment(target string, timeout time.Duration) (bool, string, map[string]interface{}) {
	// Create diagnostics map
	diagnostics := make(map[string]interface{})
	diagnostics["target"] = target
	diagnostics["timeout"] = timeout.String()

	// Try to establish TCP connection first (as base for session)
	conn, err := net.DialTimeout("tcp", target, timeout)
	if err != nil {
		diagnostics["error"] = err.Error()
		diagnostics["connection_state"] = "failed"
		return false, fmt.Sprintf("Failed to establish session with %s: %v", target, err), diagnostics
	}
	defer conn.Close()

	// Get connection details
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		diagnostics["local_addr"] = tcpConn.LocalAddr().String()
		diagnostics["remote_addr"] = tcpConn.RemoteAddr().String()

		// Try to get more TCP-specific info
		if err := tcpConn.SetKeepAlive(true); err == nil {
			diagnostics["keepalive_enabled"] = true
		}
	}

	// Try basic session handshake
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		diagnostics["error"] = "Failed to set connection deadline"
		diagnostics["connection_state"] = "unstable"
		return false, fmt.Sprintf("Session with %s is unstable: failed to set timeout", target), diagnostics
	}

	diagnostics["connection_state"] = "established"
	return true, fmt.Sprintf("Successfully established session with %s", target), diagnostics
}

// GetDependencies returns the layer numbers this layer depends on
func (r *Runner) GetDependencies() []int {
	return []int{1, 2, 3, 4} // Layer 5 depends on Layers 1-4
}

// GetDescription returns a description of this layer's functionality
func (r *Runner) GetDescription() string {
	return "Tests session establishment and management"
}

// GetName returns the name of this layer
func (r *Runner) GetName() string {
	return "Session Layer"
}

// ValidateConfig validates the configuration for this layer
func (r *Runner) ValidateConfig() error {
	if len(r.Targets) == 0 {
		return fmt.Errorf("at least one target must be specified")
	}
	if r.Timeout <= 0 {
		return fmt.Errorf("timeout must be greater than 0")
	}
	return nil
}
