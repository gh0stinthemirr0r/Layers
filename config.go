package layers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// LayerConfig represents configuration for a specific OSI layer
type LayerConfig struct {
	Enabled  bool           `json:"enabled" yaml:"enabled"`       // Whether to test this layer
	Timeout  time.Duration  `json:"timeout" yaml:"timeout"`       // Layer-specific timeout
	Targets  []string       `json:"targets" yaml:"targets"`       // Target hosts/addresses to test
	Options  map[string]any `json:"options" yaml:"options"`       // Layer-specific options
	Retry    RetryConfig    `json:"retry,omitempty" yaml:"retry"` // Retry configuration
	Priority int            `json:"priority" yaml:"priority"`     // Execution priority (lower runs first)
	Tags     []string       `json:"tags,omitempty" yaml:"tags"`   // Tags for grouping tests
}

// RetryConfig controls retry behavior for failed tests
type RetryConfig struct {
	Enabled       bool          `json:"enabled" yaml:"enabled"`               // Whether retries are enabled
	Count         int           `json:"count" yaml:"count"`                   // Number of retry attempts
	Interval      time.Duration `json:"interval" yaml:"interval"`             // Time to wait between retries
	BackoffFactor float64       `json:"backoff_factor" yaml:"backoff_factor"` // Multiplier for increasing wait time
}

// Config represents the structure for application configuration
type Config struct {
	// General settings
	OutputFormat  string        `json:"output_format" yaml:"output_format"`   // Output format: "csv", "pdf", "json", etc.
	OutputPath    string        `json:"output_path" yaml:"output_path"`       // Path for saving the output
	LogLevel      string        `json:"log_level" yaml:"log_level"`           // Log level: "info", "debug", or "error"
	GlobalTimeout time.Duration `json:"global_timeout" yaml:"global_timeout"` // Global timeout for all tests

	// Advanced settings
	ConcurrentMode     bool   `json:"concurrent_mode" yaml:"concurrent_mode"`           // Run tests concurrently
	MaxConcurrent      int    `json:"max_concurrent" yaml:"max_concurrent"`             // Maximum concurrent tests
	StopOnFailure      bool   `json:"stop_on_failure" yaml:"stop_on_failure"`           // Stop testing on first failure
	DependencyMode     string `json:"dependency_mode" yaml:"dependency_mode"`           // How to handle dependencies: "strict", "warn", "ignore"
	ProgressReporting  bool   `json:"progress_reporting" yaml:"progress_reporting"`     // Enable real-time progress reporting
	DetailedMetrics    bool   `json:"detailed_metrics" yaml:"detailed_metrics"`         // Collect detailed performance metrics
	SaveHistoricalData bool   `json:"save_historical_data" yaml:"save_historical_data"` // Save test results for historical comparison
	HistoryRetention   int    `json:"history_retention" yaml:"history_retention"`       // Number of historical results to keep

	// Global retry configuration (can be overridden per layer)
	GlobalRetry RetryConfig `json:"global_retry" yaml:"global_retry"` // Global retry settings

	// Layer-specific configurations
	Layer1 LayerConfig `json:"layer1" yaml:"layer1"` // Physical Layer
	Layer2 LayerConfig `json:"layer2" yaml:"layer2"` // Data Link Layer
	Layer3 LayerConfig `json:"layer3" yaml:"layer3"` // Network Layer
	Layer4 LayerConfig `json:"layer4" yaml:"layer4"` // Transport Layer
	Layer5 LayerConfig `json:"layer5" yaml:"layer5"` // Session Layer
	Layer6 LayerConfig `json:"layer6" yaml:"layer6"` // Presentation Layer
	Layer7 LayerConfig `json:"layer7" yaml:"layer7"` // Application Layer

	// Alert thresholds
	AlertThresholds AlertThresholds `json:"alert_thresholds" yaml:"alert_thresholds"` // Thresholds for alerts
}

// AlertThresholds defines thresholds for various metrics that trigger alerts
type AlertThresholds struct {
	LatencyWarningMs      int     `json:"latency_warning_ms" yaml:"latency_warning_ms"`           // Latency warning threshold in ms
	LatencyErrorMs        int     `json:"latency_error_ms" yaml:"latency_error_ms"`               // Latency error threshold in ms
	PacketLossWarningPct  float64 `json:"packet_loss_warning_pct" yaml:"packet_loss_warning_pct"` // Packet loss warning threshold
	PacketLossErrorPct    float64 `json:"packet_loss_error_pct" yaml:"packet_loss_error_pct"`     // Packet loss error threshold
	SignalStrengthWarning int     `json:"signal_strength_warning" yaml:"signal_strength_warning"` // Signal strength warning threshold
	SignalStrengthError   int     `json:"signal_strength_error" yaml:"signal_strength_error"`     // Signal strength error threshold
	JitterWarningMs       int     `json:"jitter_warning_ms" yaml:"jitter_warning_ms"`             // Jitter warning threshold in ms
	JitterErrorMs         int     `json:"jitter_error_ms" yaml:"jitter_error_ms"`                 // Jitter error threshold in ms
}

// LoadConfig reads the configuration from a file
func LoadConfig(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config

	// Determine file format by extension
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported config format: %s", ext)
	}

	// Validate config and set defaults
	if err := validateConfig(&config); err != nil {
		return nil, err
	}

	setConfigDefaults(&config)
	return &config, nil
}

// SaveConfig saves the configuration to a file
func SaveConfig(config *Config, filePath string) error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	var data []byte
	var err error

	// Format based on file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".json":
		data, err = json.MarshalIndent(config, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal config to JSON: %w", err)
		}
	case ".yaml", ".yml":
		data, err = yaml.Marshal(config)
		if err != nil {
			return fmt.Errorf("failed to marshal config to YAML: %w", err)
		}
	default:
		return fmt.Errorf("unsupported config format: %s", ext)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// validateConfig ensures that the configuration values are valid
func validateConfig(config *Config) error {
	// Validate general settings
	validOutputFormats := map[string]struct{}{
		"csv":  {},
		"pdf":  {},
		"json": {},
		"yaml": {},
		"html": {},
		"md":   {},
		"xml":  {},
	}

	if _, valid := validOutputFormats[config.OutputFormat]; !valid {
		return fmt.Errorf("invalid output format: %s. Allowed formats: csv, pdf, json, yaml, html, md, xml", config.OutputFormat)
	}

	validLogLevels := map[string]struct{}{
		"info":  {},
		"debug": {},
		"error": {},
		"warn":  {},
	}

	if _, valid := validLogLevels[config.LogLevel]; !valid {
		return fmt.Errorf("invalid log level: %s. Allowed levels: info, debug, error, warn", config.LogLevel)
	}

	// Validate dependency mode
	validDependencyModes := map[string]struct{}{
		"strict": {},
		"warn":   {},
		"ignore": {},
	}

	if _, valid := validDependencyModes[config.DependencyMode]; !valid {
		return fmt.Errorf("invalid dependency mode: %s. Allowed modes: strict, warn, ignore", config.DependencyMode)
	}

	// Validate global retry settings
	if config.GlobalRetry.Enabled {
		if config.GlobalRetry.Count <= 0 {
			return fmt.Errorf("global retry count must be greater than 0 when retry is enabled")
		}
		if config.GlobalRetry.Interval <= 0 {
			return fmt.Errorf("global retry interval must be greater than 0 when retry is enabled")
		}
	}

	// Validate layer configurations
	layers := []struct {
		name   string
		config LayerConfig
	}{
		{"Layer1", config.Layer1},
		{"Layer2", config.Layer2},
		{"Layer3", config.Layer3},
		{"Layer4", config.Layer4},
		{"Layer5", config.Layer5},
		{"Layer6", config.Layer6},
		{"Layer7", config.Layer7},
	}

	for _, layer := range layers {
		if layer.config.Enabled {
			if layer.config.Timeout < 0 {
				return fmt.Errorf("%s: timeout cannot be negative", layer.name)
			}

			// Don't require targets for all layers as some might be capability tests
			// rather than target-specific tests

			// Validate layer-specific retry settings
			if layer.config.Retry.Enabled {
				if layer.config.Retry.Count <= 0 {
					return fmt.Errorf("%s: retry count must be greater than 0 when retry is enabled", layer.name)
				}
				if layer.config.Retry.Interval <= 0 {
					return fmt.Errorf("%s: retry interval must be greater than 0 when retry is enabled", layer.name)
				}
			}
		}
	}

	// Validate alert thresholds
	if config.AlertThresholds.LatencyWarningMs >= config.AlertThresholds.LatencyErrorMs {
		return fmt.Errorf("latency warning threshold must be less than error threshold")
	}

	if config.AlertThresholds.PacketLossWarningPct >= config.AlertThresholds.PacketLossErrorPct {
		return fmt.Errorf("packet loss warning threshold must be less than error threshold")
	}

	if config.AlertThresholds.JitterWarningMs >= config.AlertThresholds.JitterErrorMs {
		return fmt.Errorf("jitter warning threshold must be less than error threshold")
	}

	return nil
}

// setConfigDefaults sets default values for optional configuration settings
func setConfigDefaults(config *Config) {
	// Set general defaults
	if config.GlobalTimeout <= 0 {
		config.GlobalTimeout = 30 * time.Second
	}

	if config.LogLevel == "" {
		config.LogLevel = "info"
	}

	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 5
	}

	if config.DependencyMode == "" {
		config.DependencyMode = "warn"
	}

	if config.HistoryRetention <= 0 {
		config.HistoryRetention = 30
	}

	// Set global retry defaults
	if config.GlobalRetry.Enabled && config.GlobalRetry.Count <= 0 {
		config.GlobalRetry.Count = 3
	}

	if config.GlobalRetry.Enabled && config.GlobalRetry.Interval <= 0 {
		config.GlobalRetry.Interval = 500 * time.Millisecond
	}

	if config.GlobalRetry.Enabled && config.GlobalRetry.BackoffFactor <= 0 {
		config.GlobalRetry.BackoffFactor = 1.5
	}

	// Set layer-specific defaults
	layers := []*LayerConfig{
		&config.Layer1,
		&config.Layer2,
		&config.Layer3,
		&config.Layer4,
		&config.Layer5,
		&config.Layer6,
		&config.Layer7,
	}

	for i, layer := range layers {
		if layer.Timeout <= 0 {
			layer.Timeout = config.GlobalTimeout
		}

		if layer.Options == nil {
			layer.Options = make(map[string]any)
		}

		// Set default priorities if not specified
		if layer.Priority <= 0 {
			layer.Priority = i + 1 // Default priority is layer number
		}

		// Apply global retry settings if not set
		if !layer.Retry.Enabled && config.GlobalRetry.Enabled {
			layer.Retry = config.GlobalRetry
		}
	}

	// Set default alert thresholds
	if config.AlertThresholds.LatencyWarningMs <= 0 {
		config.AlertThresholds.LatencyWarningMs = 100
	}

	if config.AlertThresholds.LatencyErrorMs <= 0 {
		config.AlertThresholds.LatencyErrorMs = 500
	}

	if config.AlertThresholds.PacketLossWarningPct <= 0 {
		config.AlertThresholds.PacketLossWarningPct = 1
	}

	if config.AlertThresholds.PacketLossErrorPct <= 0 {
		config.AlertThresholds.PacketLossErrorPct = 5
	}

	if config.AlertThresholds.SignalStrengthWarning <= 0 {
		config.AlertThresholds.SignalStrengthWarning = 50
	}

	if config.AlertThresholds.SignalStrengthError <= 0 {
		config.AlertThresholds.SignalStrengthError = 25
	}

	if config.AlertThresholds.JitterWarningMs <= 0 {
		config.AlertThresholds.JitterWarningMs = 20
	}

	if config.AlertThresholds.JitterErrorMs <= 0 {
		config.AlertThresholds.JitterErrorMs = 50
	}
}

// GetLayerConfig returns the configuration for a specific layer
func (c *Config) GetLayerConfig(layer int) (LayerConfig, error) {
	switch layer {
	case 1:
		return c.Layer1, nil
	case 2:
		return c.Layer2, nil
	case 3:
		return c.Layer3, nil
	case 4:
		return c.Layer4, nil
	case 5:
		return c.Layer5, nil
	case 6:
		return c.Layer6, nil
	case 7:
		return c.Layer7, nil
	default:
		return LayerConfig{}, fmt.Errorf("invalid layer: %d", layer)
	}
}

// GetEnabledLayers returns a list of enabled layer numbers in priority order
func (c *Config) GetEnabledLayers() []int {
	type layerInfo struct {
		layer    int
		priority int
	}

	var layers []layerInfo

	// Collect enabled layers with their priorities
	if c.Layer1.Enabled {
		layers = append(layers, layerInfo{1, c.Layer1.Priority})
	}
	if c.Layer2.Enabled {
		layers = append(layers, layerInfo{2, c.Layer2.Priority})
	}
	if c.Layer3.Enabled {
		layers = append(layers, layerInfo{3, c.Layer3.Priority})
	}
	if c.Layer4.Enabled {
		layers = append(layers, layerInfo{4, c.Layer4.Priority})
	}
	if c.Layer5.Enabled {
		layers = append(layers, layerInfo{5, c.Layer5.Priority})
	}
	if c.Layer6.Enabled {
		layers = append(layers, layerInfo{6, c.Layer6.Priority})
	}
	if c.Layer7.Enabled {
		layers = append(layers, layerInfo{7, c.Layer7.Priority})
	}

	// Sort by priority
	sort.Slice(layers, func(i, j int) bool {
		return layers[i].priority < layers[j].priority
	})

	// Extract layer numbers
	result := make([]int, len(layers))
	for i, l := range layers {
		result[i] = l.layer
	}

	return result
}

// PrintConfig displays the configuration values
func PrintConfig(config *Config) {
	fmt.Println("Configuration:")
	fmt.Printf("  Output Format: %s\n", config.OutputFormat)
	fmt.Printf("  Output Path: %s\n", config.OutputPath)
	fmt.Printf("  Log Level: %s\n", config.LogLevel)
	fmt.Printf("  Global Timeout: %s\n", config.GlobalTimeout)
	fmt.Printf("  Concurrent Mode: %v\n", config.ConcurrentMode)
	fmt.Printf("  Max Concurrent: %d\n", config.MaxConcurrent)
	fmt.Printf("  Stop On Failure: %v\n", config.StopOnFailure)
	fmt.Printf("  Dependency Mode: %s\n", config.DependencyMode)
	fmt.Printf("  Progress Reporting: %v\n", config.ProgressReporting)
	fmt.Printf("  Save Historical Data: %v\n", config.SaveHistoricalData)
	fmt.Printf("  History Retention: %d days\n", config.HistoryRetention)

	fmt.Println("\nGlobal Retry Configuration:")
	fmt.Printf("  Enabled: %v\n", config.GlobalRetry.Enabled)
	if config.GlobalRetry.Enabled {
		fmt.Printf("  Count: %d\n", config.GlobalRetry.Count)
		fmt.Printf("  Interval: %s\n", config.GlobalRetry.Interval)
		fmt.Printf("  Backoff Factor: %.2f\n", config.GlobalRetry.BackoffFactor)
	}

	fmt.Println("\nAlert Thresholds:")
	fmt.Printf("  Latency Warning: %d ms\n", config.AlertThresholds.LatencyWarningMs)
	fmt.Printf("  Latency Error: %d ms\n", config.AlertThresholds.LatencyErrorMs)
	fmt.Printf("  Packet Loss Warning: %.2f%%\n", config.AlertThresholds.PacketLossWarningPct)
	fmt.Printf("  Packet Loss Error: %.2f%%\n", config.AlertThresholds.PacketLossErrorPct)
	fmt.Printf("  Signal Strength Warning: %d%%\n", config.AlertThresholds.SignalStrengthWarning)
	fmt.Printf("  Signal Strength Error: %d%%\n", config.AlertThresholds.SignalStrengthError)
	fmt.Printf("  Jitter Warning: %d ms\n", config.AlertThresholds.JitterWarningMs)
	fmt.Printf("  Jitter Error: %d ms\n", config.AlertThresholds.JitterErrorMs)

	layers := []struct {
		name   string
		config LayerConfig
	}{
		{"Layer1 (Physical)", config.Layer1},
		{"Layer2 (Data Link)", config.Layer2},
		{"Layer3 (Network)", config.Layer3},
		{"Layer4 (Transport)", config.Layer4},
		{"Layer5 (Session)", config.Layer5},
		{"Layer6 (Presentation)", config.Layer6},
		{"Layer7 (Application)", config.Layer7},
	}

	fmt.Println("\nLayer Configurations:")
	for _, layer := range layers {
		if layer.config.Enabled {
			fmt.Printf("  %s:\n", layer.name)
			fmt.Printf("    Timeout: %s\n", layer.config.Timeout)
			fmt.Printf("    Priority: %d\n", layer.config.Priority)
			fmt.Printf("    Targets: %v\n", layer.config.Targets)

			if len(layer.config.Tags) > 0 {
				fmt.Printf("    Tags: %v\n", layer.config.Tags)
			}

			if layer.config.Retry.Enabled {
				fmt.Printf("    Retry: enabled (count=%d, interval=%s, backoff=%.2f)\n",
					layer.config.Retry.Count,
					layer.config.Retry.Interval,
					layer.config.Retry.BackoffFactor)
			}

			if len(layer.config.Options) > 0 {
				fmt.Printf("    Options: %v\n", layer.config.Options)
			}
		}
	}
}

// CreateDefaultConfig creates a default configuration file
func CreateDefaultConfig(filePath string) error {
	// Create a new config with default values
	config := &Config{
		OutputFormat:  "pdf",
		OutputPath:    "",
		LogLevel:      "info",
		GlobalTimeout: 30 * time.Second,

		ConcurrentMode:     true,
		MaxConcurrent:      5,
		StopOnFailure:      false,
		DependencyMode:     "warn",
		ProgressReporting:  true,
		DetailedMetrics:    true,
		SaveHistoricalData: true,
		HistoryRetention:   30,

		GlobalRetry: RetryConfig{
			Enabled:       true,
			Count:         3,
			Interval:      500 * time.Millisecond,
			BackoffFactor: 1.5,
		},

		AlertThresholds: AlertThresholds{
			LatencyWarningMs:      100,
			LatencyErrorMs:        500,
			PacketLossWarningPct:  1.0,
			PacketLossErrorPct:    5.0,
			SignalStrengthWarning: 50,
			SignalStrengthError:   25,
			JitterWarningMs:       20,
			JitterErrorMs:         50,
		},

		// Default Layer 1 config
		Layer1: LayerConfig{
			Enabled:  true,
			Timeout:  5 * time.Second,
			Priority: 1,
			Targets:  []string{"eth0", "wlan0"},
			Options: map[string]any{
				"attempt_count":       3,
				"min_signal_strength": 50,
			},
			Tags: []string{"physical", "hardware"},
		},

		// Default Layer 2 config
		Layer2: LayerConfig{
			Enabled:  true,
			Timeout:  5 * time.Second,
			Priority: 2,
			Targets:  []string{"eth0", "wlan0"},
			Options: map[string]any{
				"check_mac": true,
				"check_mtu": true,
			},
			Tags: []string{"datalink", "ethernet"},
		},

		// Default Layer 3 config
		Layer3: LayerConfig{
			Enabled:  true,
			Timeout:  10 * time.Second,
			Priority: 3,
			Targets:  []string{"8.8.8.8", "1.1.1.1"},
			Options: map[string]any{
				"hostname":   "example.com",
				"ping_addr":  "8.8.8.8",
				"ping_count": 4,
			},
			Tags: []string{"network", "ip", "ping"},
		},

		// Default Layer 4 config
		Layer4: LayerConfig{
			Enabled:  true,
			Timeout:  10 * time.Second,
			Priority: 4,
			Targets: []string{
				"tcp://example.com:80",
				"tcp://example.com:443",
			},
			Options: map[string]any{
				"udp_addr": "udp://example.com:53",
			},
			Tags: []string{"transport", "tcp", "udp"},
		},

		// Default Layer 5 config
		Layer5: LayerConfig{
			Enabled:  true,
			Timeout:  15 * time.Second,
			Priority: 5,
			Targets: []string{
				"example.com:22",
				"example.com:443",
			},
			Options: map[string]any{
				"check_session_reuse": true,
				"check_keepalive":     true,
			},
			Tags: []string{"session", "connection"},
		},

		// Default Layer 6 config
		Layer6: LayerConfig{
			Enabled:  true,
			Timeout:  10 * time.Second,
			Priority: 6,
			Targets:  []string{},
			Options: map[string]any{
				"compression": true,
				"encryption":  true,
				"encoding":    "utf-8",
			},
			Tags: []string{"presentation", "encoding", "encryption"},
		},

		// Default Layer 7 config
		Layer7: LayerConfig{
			Enabled:  true,
			Timeout:  15 * time.Second,
			Priority: 7,
			Targets: []string{
				"https://api.example.com/health",
				"https://api.example.com/status",
			},
			Options: map[string]any{
				"http_method":      "GET",
				"verify_ssl":       true,
				"follow_redirects": true,
			},
			Tags: []string{"application", "http", "api"},
		},
	}

	// Save the config to file
	return SaveConfig(config, filePath)
}

// ValidateConfig validates the configuration
func (c *Config) ValidateConfig() error {
	if c.GlobalTimeout <= 0 {
		return fmt.Errorf("global timeout must be greater than 0")
	}
	return nil
}
