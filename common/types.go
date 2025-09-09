// Package common provides shared types and interfaces for OSI layer testing
package common

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// TestStatus defines the possible outcomes of a test
type TestStatus string

const (
	StatusPassed  TestStatus = "Passed"
	StatusFailed  TestStatus = "Failed"
	StatusWarning TestStatus = "Warning"
	StatusSkipped TestStatus = "Skipped"
	StatusMixed   TestStatus = "Mixed" // For tests with both passed and failed sub-results
)

// TestResult represents one outcome from a single layer test or sub-test.
type TestResult struct {
	Layer       int          `json:"layer"`
	Name        string       `json:"name"`                  // Test name
	Status      TestStatus   `json:"status"`                // e.g. "Passed", "Failed", "Warning", "Skipped"
	Message     string       `json:"message"`               // Additional details
	StartTime   time.Time    `json:"start_time"`            // When the test started
	EndTime     time.Time    `json:"end_time"`              // When the test completed
	Metrics     TestMetrics  `json:"metrics"`               // Performance metrics
	SubResults  []TestResult `json:"sub_results,omitempty"` // Results of subtests
	Diagnostics interface{}  `json:"diagnostics,omitempty"` // Detailed diagnostic data including network and security info
}

// TestMetrics contains performance and reliability metrics
type TestMetrics struct {
	Duration       time.Duration          `json:"duration"`         // Test duration
	TransferRate   float64                `json:"transfer_rate"`    // In MB/s if applicable
	Latency        time.Duration          `json:"latency"`          // Average latency
	PacketLoss     float64                `json:"packet_loss"`      // Percentage of packet loss (0-100)
	ResponseTime   time.Duration          `json:"response_time"`    // Average response time
	Jitter         time.Duration          `json:"jitter"`           // Jitter measurement
	ReliabilityPct float64                `json:"reliability_pct"`  // Overall reliability percentage (0-100)
	Custom         map[string]interface{} `json:"custom,omitempty"` // Custom metrics
}

// NetworkDetails contains information about network interfaces and their status
type NetworkDetails struct {
	InterfaceName string   `json:"interfaceName"`
	Status        string   `json:"status"`
	IPv4Address   []string `json:"ipv4Address"`
	IPv6Address   []string `json:"ipv6Address"`
	IsPrimary     bool     `json:"isPrimary"`
	IsVPN         bool     `json:"isVPN"`
}

// PortInfo contains information about an open port
type PortInfo struct {
	Port         int    `json:"port"`
	Protocol     string `json:"protocol"`
	Service      string `json:"service"`
	IsVulnerable bool   `json:"isVulnerable"`
}

// SecurityFindings contains the overall security assessment
type SecurityFindings struct {
	NetworkDetails  []NetworkDetails `json:"networkDetails"`
	OpenPorts       []PortInfo       `json:"openPorts"`
	Vulnerabilities []string         `json:"vulnerabilities"`
}

// LayerRunner is the interface each layer implements, returning one or more test results.
type LayerRunner interface {
	RunTests(ctx context.Context, logger *zap.Logger) ([]TestResult, error)
	GetName() string
	GetDescription() string
	GetDependencies() []int
	ValidateConfig() error
}

// TestProgressCallback is a function called to update test progress
type TestProgressCallback func(layer int, completed, total int, status string)

// TestConfig holds common test configuration
type TestConfig struct {
	Enabled       bool                   `json:"enabled"`
	Timeout       time.Duration          `json:"timeout"`
	RetryCount    int                    `json:"retry_count"`
	RetryInterval time.Duration          `json:"retry_interval"`
	Targets       []string               `json:"targets"`
	Options       map[string]interface{} `json:"options"`
}

// Global logger instance
var Logger *zap.Logger

// Constants for visualization
const (
	WindowWidth  = 1280
	WindowHeight = 720
	MaxParticles = 100
)

// Constants for file paths
const (
	LogDir     = "Logging"
	ReportDir  = "Reporting"
	ConfigDir  = "Config"
	MetricsDir = "Metrics"
	CacheDir   = "Cache"
)

// Constants for test results
const (
	MaxConcurrentTests = 10
	DefaultTimeout     = 30 * time.Second
	DefaultRetryCount  = 3
	RetryBackoff       = 500 * time.Millisecond
)
