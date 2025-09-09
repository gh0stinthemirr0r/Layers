// Package layer4 implements transport layer testing functionality
package layer4

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"ghostshell/app/layers/common"
)

// Runner implements transport layer tests
type Runner struct {
	*common.Layer4Runner
}

// New creates a new Layer4Runner
func New(tcpAddresses []string, udpAddress string, timeout time.Duration) *Runner {
	return &Runner{
		Layer4Runner: &common.Layer4Runner{
			TCPAddresses: tcpAddresses,
			UDPAddress:   udpAddress,
			Timeout:      timeout,
		},
	}
}

// RunTests implements the LayerRunner interface
func (r *Runner) RunTests(ctx context.Context, logger *zap.Logger) ([]common.TestResult, error) {
	logger.Info("Starting Layer 4 (Transport Layer) tests...",
		zap.Strings("tcp_addresses", r.TCPAddresses),
		zap.String("udp_address", r.UDPAddress))

	startTime := time.Now()

	// Create parent result
	parentResult := common.TestResult{
		Layer:      4,
		Name:       "Transport Layer Tests",
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

		// Test TCP connections
		for _, addr := range r.TCPAddresses {
			tcpResult := common.TestResult{
				Layer:     4,
				Name:      fmt.Sprintf("TCP Connection Test (%s)", addr),
				StartTime: time.Now(),
			}

			success, msg := checkTCPConnection(addr, r.Timeout)
			if !success {
				tcpResult.Status = common.StatusFailed
				tcpResult.Message = msg
				failedTests = append(failedTests, msg)
			} else {
				tcpResult.Status = common.StatusPassed
				tcpResult.Message = msg
			}

			tcpResult.EndTime = time.Now()
			tcpResult.Metrics.Duration = tcpResult.EndTime.Sub(tcpResult.StartTime)
			parentResult.SubResults = append(parentResult.SubResults, tcpResult)
		}

		// Test UDP connection
		udpResult := common.TestResult{
			Layer:     4,
			Name:      fmt.Sprintf("UDP Connection Test (%s)", r.UDPAddress),
			StartTime: time.Now(),
		}

		success, msg := checkUDPConnection(r.UDPAddress, r.Timeout)
		if !success {
			udpResult.Status = common.StatusFailed
			udpResult.Message = msg
			failedTests = append(failedTests, msg)
		} else {
			udpResult.Status = common.StatusPassed
			udpResult.Message = msg
		}

		udpResult.EndTime = time.Now()
		udpResult.Metrics.Duration = udpResult.EndTime.Sub(udpResult.StartTime)
		parentResult.SubResults = append(parentResult.SubResults, udpResult)

		// Set overall test status and message
		if len(failedTests) > 0 {
			parentResult.Status = common.StatusFailed
			parentResult.Message = fmt.Sprintf("Layer 4 tests failed with %d failures:\n\n%s",
				len(failedTests), strings.Join(failedTests, "\n\n"))
			logger.Error(parentResult.Message)
		} else {
			parentResult.Status = common.StatusPassed
			parentResult.Message = fmt.Sprintf("All Layer 4 tests passed successfully:\n"+
				"- TCP connections tested: %d\n"+
				"- UDP connection tested: %s",
				len(r.TCPAddresses), r.UDPAddress)
			logger.Info(parentResult.Message)
		}

		parentResult.EndTime = time.Now()
		parentResult.Metrics.Duration = parentResult.EndTime.Sub(parentResult.StartTime)

		// Generate reports
		if err := generateReports([]common.TestResult{parentResult}); err != nil {
			logger.Error("Failed to generate reports", zap.Error(err))
		}

		if len(failedTests) > 0 {
			return []common.TestResult{parentResult}, fmt.Errorf("layer 4 tests failed")
		}
		return []common.TestResult{parentResult}, nil
	}
}

// generateReports generates test reports in various formats
func generateReports(results []common.TestResult) error {
	timestamp := time.Now().Format("20060102_150405")
	basePath := filepath.Join(common.ReportDir, fmt.Sprintf("layer4_tests_%s", timestamp))

	if err := os.MkdirAll(common.ReportDir, 0755); err != nil {
		return fmt.Errorf("failed to create report directory: %v", err)
	}

	// Generate CSV report
	if err := common.WriteCSVReport(results, basePath+".csv"); err != nil {
		return fmt.Errorf("failed to write CSV report: %v", err)
	}

	// Generate PDF report
	if err := common.WritePDFReport(results, basePath+".pdf"); err != nil {
		return fmt.Errorf("failed to write PDF report: %v", err)
	}

	return nil
}

// checkTCPConnection attempts to establish a TCP connection to the given address
func checkTCPConnection(addr string, timeout time.Duration) (bool, string) {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false, fmt.Sprintf("TCP connection to %s failed: %v", addr, err)
	}
	defer conn.Close()
	return true, fmt.Sprintf("TCP connection to %s successful", addr)
}

// checkUDPConnection attempts to establish a UDP connection to the given address
func checkUDPConnection(addr string, timeout time.Duration) (bool, string) {
	conn, err := net.DialTimeout("udp", addr, timeout)
	if err != nil {
		return false, fmt.Sprintf("UDP connection to %s failed: %v", addr, err)
	}
	defer conn.Close()

	// For UDP, we should try to send/receive data to verify the connection
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return false, fmt.Sprintf("Failed to set UDP timeout for %s: %v", addr, err)
	}

	// Send test data
	testData := []byte("UDP test packet")
	_, err = conn.Write(testData)
	if err != nil {
		return false, fmt.Sprintf("Failed to send UDP test packet to %s: %v", addr, err)
	}

	return true, fmt.Sprintf("UDP connection to %s successful", addr)
}

// GetDependencies returns the layer numbers this layer depends on
func (r *Runner) GetDependencies() []int {
	return []int{1, 2, 3} // Layer 4 depends on Layers 1, 2, and 3
}

// GetDescription returns a description of this layer's functionality
func (r *Runner) GetDescription() string {
	return "Tests transport layer protocols including TCP and UDP"
}

// GetName returns the name of this layer
func (r *Runner) GetName() string {
	return "Transport Layer"
}

// ValidateConfig validates the configuration for this layer
func (r *Runner) ValidateConfig() error {
	if len(r.TCPAddresses) == 0 {
		return fmt.Errorf("at least one TCP address must be specified")
	}
	if r.UDPAddress == "" {
		return fmt.Errorf("UDP address must be specified")
	}
	if r.Timeout <= 0 {
		return fmt.Errorf("timeout must be greater than 0")
	}
	return nil
}
