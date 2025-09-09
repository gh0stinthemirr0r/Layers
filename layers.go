// Package layers provides OSI layer testing functionality
package layers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"ghostshell/app/layers/common"
	"ghostshell/app/layers/layer1"
	"ghostshell/app/layers/layer2"
	"ghostshell/app/layers/layer3"
	"ghostshell/app/layers/layer4"
	"ghostshell/app/layers/layer5"
	"ghostshell/app/layers/layer6"
	"ghostshell/app/layers/layer7"
)

// TestSession represents a complete testing session
type TestSession struct {
	Config          *Config
	Logger          *zap.Logger
	Results         map[int][]common.TestResult
	ProgressCallback common.TestProgressCallback
	StartTime       time.Time
	EndTime         time.Time
	RunID           string
}

// NewTestSession creates a new test session with the given configuration
func NewTestSession(config *Config) (*TestSession, error) {
	// Create logger
	logger, err := initializeLogger(config.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Create run ID based on timestamp
	runID := time.Now().Format("20060102_150405")

	// Return new session
	return &TestSession{
		Config:     config,
		Logger:     logger,
		Results:    make(map[int][]common.TestResult),
		StartTime:  time.Now(),
		RunID:      runID,
	}, nil
}

// SetProgressCallback sets a callback function for progress updates
func (ts *TestSession) SetProgressCallback(callback common.TestProgressCallback) {
	ts.ProgressCallback = callback
}

// RunAllTests runs tests for all enabled layers
func (ts *TestSession) RunAllTests() ([]common.TestResult, error) {
	// Get enabled layers in priority order
	enabledLayers := ts.Config.GetEnabledLayers()
	if len(enabledLayers) == 0 {
		return nil, fmt.Errorf("no layers enabled in configuration")
	}

	// Log start of testing
	ts.Logger.Info("Starting layer tests",
		zap.Ints("layers", enabledLayers),
		zap.String("run_id", ts.RunID),
	)

	// Create base context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), ts.Config.GlobalTimeout)
	defer cancel()

	// Initialize layer runners
	runners, err := ts.initializeRunners(enabledLayers)
	if err != nil {
		return nil, err
	}

	// Run tests
	var results []common.TestResult
	ts.StartTime = time.Now()

	if ts.Config.ConcurrentMode {
		// Run tests concurrently
		results, err = ts.runConcurrentTests(ctx, runners)
	} else {
		// Run tests sequentially
		results, err = ts.runSequentialTests(ctx, runners)
	}

	ts.EndTime = time.Now()

	// Generate reports
	if err := ts.generateReports(results); err != nil {
		ts.Logger.Error("Failed to generate reports", zap.Error(err))
	}

	// Save results to history if enabled
	if ts.Config.SaveHistoricalData {
		if err := ts.saveHistoricalData(results); err != nil {
			ts.Logger.Error("Failed to save historical data", zap.Error(err))
		}
	}

	return results, err
}

// RunSelectedLayers runs tests for selected layers
func (ts *TestSession) RunSelectedLayers(layers []int) ([]common.TestResult, error) {
	// Filter the selected layers by what's enabled in the config
	enabledLayers := ts.Config.GetEnabledLayers()
	enabledMap := make(map[int]bool)
	for _, layer := range enabledLayers {
		enabledMap[layer] = true
	}

	var selectedLayers []int
	for _, layer := range layers {
		if enabledMap[layer] {
			selectedLayers = append(selectedLayers, layer)
		}
	}

	// Sort layers by priority
	sort.Ints(selectedLayers)

	if len(selectedLayers) == 0 {
		return nil, fmt.Errorf("no valid layers selected or enabled")
	}

	// Log start of testing
	ts.Logger.Info("Starting selected layer tests",
		zap.Ints("layers", selectedLayers),
		zap.String("run_id", ts.RunID),
	)

	// Create base context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), ts.Config.GlobalTimeout)
	defer cancel()

	// Initialize layer runners
	runners, err := ts.initializeRunners(selectedLayers)
	if err != nil {
		return nil, err
	}

	// Run tests
	var results []common.TestResult
	ts.StartTime = time.Now()

	if ts.Config.ConcurrentMode {
		// Run tests concurrently
		results, err = ts.runConcurrentTests(ctx, runners)
	} else {
		// Run tests sequentially
		results, err = ts.runSequentialTests(ctx, runners)
	}

	ts.EndTime = time.Now()

	// Generate reports
	if err := ts.generateReports(results); err != nil {
		ts.Logger.Error("Failed to generate reports", zap.Error(err))
	}

	return results, err
}

// runSequentialTests runs tests one after another
func (ts *TestSession) runSequentialTests(ctx context.Context, runners map[int]common.LayerRunner) ([]common.TestResult, error) {
	var allResults []common.TestResult
	layers := make([]int, 0, len(runners))

	// Sort layers by priority
	for layer := range runners {
		layers = append(layers, layer)
	}
	sort.Ints(layers)

	for _, layer := range layers {
		runner := runners[layer]
		
		// Get layer specific timeout
		layerConfig, err := ts.Config.GetLayerConfig(layer)
		if err != nil {
			ts.Logger.Error("Failed to get layer config", zap.Int("layer", layer), zap.Error(err))
			continue
		}

		// Create layer-specific context with timeout
		layerCtx, layerCancel := context.WithTimeout(ctx, layerConfig.Timeout)
		
		// Progress update - starting
		if ts.ProgressCallback != nil {
			ts.ProgressCallback(layer, 0, 1, "Running")
		}

		// Run tests for this layer
		results, err := ts.runLayerTestsWithRetry(layerCtx, layer, runner)
		layerCancel()

		// Progress update - complete
		if ts.ProgressCallback != nil {
			ts.ProgressCallback(layer, 1, 1, "Complete")
		}

		if err != nil {
			ts.Logger.Error("Layer test failed",
				zap.Int("layer", layer),
				zap.Error(err),
			)
			
			// Store results even if failed
			if results != nil && len(results) > 0 {
				allResults = append(allResults, results...)
				ts.Results[layer] = results
			}
			
			// Check if we should stop on failure
			if ts.Config.StopOnFailure {
				ts.Logger.Warn("Stopping tests due to layer failure",
					zap.Int("layer", layer),
				)
				break
			}
		} else {
			// Add results
			allResults = append(allResults, results...)
			ts.Results[layer] = results
		}
	}

	return allResults, nil
}

// runConcurrentTests runs tests concurrently with controlled concurrency
func (ts *TestSession) runConcurrentTests(ctx context.Context, runners map[int]common.LayerRunner) ([]common.TestResult, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var allResults []common.TestResult
	
	// Create channel for concurrency control
	semaphore := make(chan struct{}, ts.Config.MaxConcurrent)
	
	layers := make([]int, 0, len(runners))
	
	// Sort layers by priority
	for layer := range runners {
		layers = append(layers, layer)
	}
	sort.Ints(layers)

	// Track errors
	errChan := make(chan error, len(runners))
	
	// Run each layer test in its own goroutine
	for _, layer := range layers {
		wg.Add(1)
		
		// Get layer config for timeout
		layerConfig, err := ts.Config.GetLayerConfig(layer)
		if err != nil {
			ts.Logger.Error("Failed to get layer config", zap.Int("layer", layer), zap.Error(err))
			wg.Done()
			continue
		}
		
		// Acquire semaphore slot
		semaphore <- struct{}{}
		
		// Run test in goroutine
		go func(l int, r common.LayerRunner, lc LayerConfig) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore when done
			
			// Progress update - starting
			if ts.ProgressCallback != nil {
				ts.ProgressCallback(l, 0, 1, "Running")
			}
			
			// Create layer-specific context with timeout
			layerCtx, layerCancel := context.WithTimeout(ctx, lc.Timeout)
			defer layerCancel()
			
			// Run tests for this layer
			results, err := ts.runLayerTestsWithRetry(layerCtx, l, r)
			
			// Progress update - complete
			if ts.ProgressCallback != nil {
				ts.ProgressCallback(l, 1, 1, "Complete")
			}
			
			if err != nil {
				ts.Logger.Error("Layer test failed",
					zap.Int("layer", l),
					zap.Error(err),
				)
				errChan <- err
			}
			
			// Store results
			if results != nil && len(results) > 0 {
				mu.Lock()
				allResults = append(allResults, results...)
				ts.Results[l] = results
				mu.Unlock()
			}
		}(layer, runners[layer], layerConfig)
	}
	
	// Wait for all tests to complete
	wg.Wait()
	close(errChan)
	
	// Check for errors
	var lastError error
	for err := range errChan {
		lastError = err
		if ts.Config.StopOnFailure {
			break
		}
	}
	
	return allResults, lastError
}

// runLayerTestsWithRetry runs tests for a specific layer with retry logic
func (ts *TestSession) runLayerTestsWithRetry(ctx context.Context, layer int, runner common.LayerRunner) ([]common.TestResult, error) {
	layerConfig, err := ts.Config.GetLayerConfig(layer)
	if err != nil {
		return nil, err
	}

	var attempt int
	var lastErr error
	var results []common.TestResult

	// Determine retry settings
	retry := layerConfig.Retry
	if !retry.Enabled {
		retry = ts.Config.GlobalRetry
	}

	// Execute test with retry
	for attempt = 0; attempt <= retry.Count; attempt++ {
		// If not first attempt, wait before retry
		if attempt > 0 {
			// Calculate backoff duration
			waitTime := retry.Interval
			for i := 1; i < attempt; i++ {
				waitTime = time.Duration(float64(waitTime) * retry.BackoffFactor)
			}
			
			ts.Logger.Info("Retrying layer test",
				zap.Int("layer", layer),
				zap.Int("attempt", attempt),
				zap.Duration("wait_time", waitTime),
			)
			
			// Update progress
			if ts.ProgressCallback != nil {
				ts.ProgressCallback(layer, 0, 1, fmt.Sprintf("Retrying (%d/%d)", attempt, retry.Count))
			}
			
			// Wait before retry
			select {
			case <-time.After(waitTime):
				// Continue after waiting
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		// Run the test
		results, lastErr = runner.RunTests(ctx, ts.Logger)
		
		// Check for success or retryable errors
		if lastErr == nil {
			return results, nil
		}
		
		// If we've reached the maximum retry count, return the last error
		if attempt >= retry.Count {
			break
		}
	}

	return results, fmt.Errorf("failed after %d attempts: %w", attempt, lastErr)
}

// generateReports creates reports in the configured format
func (ts *TestSession) generateReports(results []common.TestResult) error {
	// Create report generator
	generator := common.NewReportGenerator(results, "layer_tests")
	generator.CreatedAt = ts.StartTime
	
	// Set output directory if configured
	if ts.Config.OutputPath != "" {
		generator.OutputDir = ts.Config.OutputPath
	}

	// Generate report in configured format
	format := common.ReportFormat(ts.Config.OutputFormat)
	
	path, err := generator.GenerateReport(format)
	if err != nil {
		return fmt.Errorf("failed to generate %s report: %w", format, err)
	}

	ts.Logger.Info("Generated report",
		zap.String("format", string(format)),
		zap.String("path", path),
	)

	return nil
}

// saveHistoricalData saves test results for historical comparison
func (ts *TestSession) saveHistoricalData(results []common.TestResult) error {
	historyDir := filepath.Join(common.MetricsDir, "history")
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	// Create JSON report in history directory
	path := filepath.Join(historyDir, fmt.Sprintf("layer_tests_%s.json", ts.RunID))
	if err := common.WriteJSONReport(results, path); err != nil {
		return fmt.Errorf("failed to save historical data: %w", err)
	}

	ts.Logger.Info("Saved historical data", zap.String("path", path))

	// Perform history retention cleanup (async)
	go ts.cleanupHistoricalData(historyDir)

	return nil
}

// cleanupHistoricalData removes old historical data files
func (ts *TestSession) cleanupHistoricalData(historyDir string) {
	// List all history files
	files, err := os.ReadDir(historyDir)
	if err != nil {
		ts.Logger.Error("Failed to read history directory", zap.Error(err))
		return
	}

	// Sort files by modification time (oldest first)
	type fileInfo struct {
		name  string
		mtime time.Time
	}

	var filesInfo []fileInfo
	for _, file := range files {
		info, err := file.Info()
		if err != nil {
			continue
		}
		filesInfo = append(filesInfo, fileInfo{
			name:  file.Name(),
			mtime: info.ModTime(),
		})
	}

	// Sort by modification time (newest first)
	sort.Slice(filesInfo, func(i, j int) bool {
		return filesInfo[i].mtime.After(filesInfo[j].mtime)
	})

	// Delete old files beyond retention limit
	if len(filesInfo) > ts.Config.HistoryRetention {
		for i := ts.Config.HistoryRetention; i < len(filesInfo); i++ {
			path := filepath.Join(historyDir, filesInfo[i].name)
			if err := os.Remove(path); err != nil {
				ts.Logger.Error("Failed to delete old history file",
					zap.String("file", path),
					zap.Error(err),
				)
			} else {
				ts.Logger.Debug("Deleted old history file",
					zap.String("file", path),
				)
			}
		}
	}
}

// initializeRunners creates runner instances for the specified layers
func (ts *TestSession) initializeRunners(layers []int) (map[int]common.LayerRunner, error) {
	runners := make(map[int]common.LayerRunner)

	for _, l := range layers {
		layerConfig, err := ts.Config.GetLayerConfig(l)
		if err != nil {
			ts.Logger.Error("Invalid layer", zap.Int("layer", l), zap.Error(err))
			continue
		}

		// Skip disabled layers
		if !layerConfig.Enabled {
			continue
		}

		// Create runner based on layer
		var runner common.LayerRunner
		switch l {
		case 1:
			// Get Layer 1 specific options
			attemptCount := 3 // Default
			if val, ok := layerConfig.Options["attempt_count"]; ok {
				if count, ok := val.(float64); ok {
					attemptCount = int(count)
				}
			}
			
			minSignalStrength := 50 // Default
			if val, ok := layerConfig.Options["min_signal_strength"]; ok {
				if strength, ok := val.(float64); ok {
					minSignalStrength = int(strength)
				}
			}
			
			runner = layer1.New(attemptCount, minSignalStrength)
			
		case 2:
			// Layer 2 options
			checkMAC := true // Default
			if val, ok := layerConfig.Options["check_mac"]; ok {
				if b, ok := val.(bool); ok {
					checkMAC = b
				}
			}
			
			checkMTU := true // Default
			if val, ok := layerConfig.Options["check_mtu"]; ok {
				if b, ok := val.(bool); ok {
					checkMTU = b
				}
			}
			
			runner = layer2.New(layerConfig.Targets, checkMAC, checkMTU)
			
		case 3:
			// Layer 3 options
			hostname := "localhost" // Default
			if val, ok := layerConfig.Options["hostname"]; ok {
				if s, ok := val.(string); ok {
					hostname = s
				}
			}
			
			pingAddr := "8.8.8.8" // Default
			if val, ok := layerConfig.Options["ping_addr"]; ok {
				if s, ok := val.(string); ok {
					pingAddr = s
				}
			}
			
			pingCount := 4 // Default
			if val, ok := layerConfig.Options["ping_count"]; ok {
				if count, ok := val.(float64); ok {
					pingCount = int(count)
				}
			}
			
			runner = layer3.New(hostname, pingAddr, pingCount)
			
		case 4:
			// Layer 4 options
			tcpAddresses := []string{"8.8.8.8:53", "1.1.1.1:53"} // Default
			if len(layerConfig.Targets) > 0 {
				tcpAddresses = layerConfig.Targets
			}
			
			udpAddress := "8.8.8.8:53" // Default
			if val, ok := layerConfig.Options["udp_addr"]; ok {
				if s, ok := val.(string); ok {
					udpAddress = s
				}
			}
			
			runner = layer4.New(tcpAddresses, udpAddress, layerConfig.Timeout)
			
		case 5:
			// Layer 5 options
			sessionTargets := []string{"8.8.8.8:53", "1.1.1.1:53"} // Default
			if len(layerConfig.Targets) > 0 {
				sessionTargets = layerConfig.Targets
			}
			
			runner = layer5.New(sessionTargets, layerConfig.Timeout)
			
		case 6:
			// Layer 6 options
			dataSets := []map[string]string{
				{"test": "Hello, World!"},
				{"json": `{"key": "value"}`},
			} // Default
			
			// Check if custom datasets are provided
			if val, ok := layerConfig.Options["data_sets"]; ok {
				if datasets, ok := val.([]map[string]string); ok {
					dataSets = datasets
				}
			}
			
			runner = layer6.New(dataSets)
			
		case 7:
			// Layer 7 options
			endpoints := []string{
				"https://www.google.com",
				"https://www.cloudflare.com",
			} // Default
			
			if len(layerConfig.Targets) > 0 {
				endpoints = layerConfig.Targets
			}
			
			runner = layer7.New(endpoints, layerConfig.Timeout)
			
		default:
			return nil, fmt.Errorf("unknown layer: %d", l)
		}

		// Store runner
		runners[l] = runner
	}

	return runners, nil
}

// CreateDefaultConfig creates a default configuration in the specified path
func CreateDefaultConfigFile(path string) error {
	return CreateDefaultConfig(path)
}

// initializeLogger creates a configured logger
func initializeLogger(level string) (*zap.Logger, error) {
	// Create log directory
	if err := os.MkdirAll(common.LogDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Set log level
	var logLevel zapcore.Level
	switch level {
	case "debug":
		logLevel = zapcore.DebugLevel
	case "info":
		logLevel = zapcore.InfoLevel
	case "warn":
		logLevel = zapcore.WarnLevel
	case "error":
		logLevel = zapcore.ErrorLevel
	default:
		logLevel = zapcore.InfoLevel
	}

	// Configure logger
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(logLevel)
	cfg.OutputPaths = []string{
		filepath.Join(common.LogDir, fmt.Sprintf("layers_%s.log", time.Now().Format("20060102_150405"))),
		"stdout",
	}
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Build logger
	logger, err := cfg.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build logger: %w", err)
	}

	// Set global logger
	zap.ReplaceGlobals(logger)
	common.Logger = logger

	return logger, nil
}

// Legacy support functions

// Options holds configuration for layer testing
type Options struct {
	OutputFormat string // "csv", "pdf", or "json"
}

// RunLayerTests initializes and runs OSI layer tests for selected layers
func RunLayerTests(selectedLayers []int) ([]common.TestResult, error) {
	// Create a default config
	config := &Config{
		OutputFormat:  "pdf",
		LogLevel:      "info",
		GlobalTimeout: 30 * time.Second,
		
		Layer1: LayerConfig{
			Enabled: true,
			Timeout: 5 * time.Second,
			Options: map[string]any{
				"attempt_count": 3,
			},
		},
		Layer2: LayerConfig{
			Enabled: true,
			Timeout: 5 * time.Second,
		},
		Layer3: LayerConfig{
			Enabled: true,
			Timeout: 10 * time.Second,
			Options: map[string]any{
				"hostname":  "localhost",
				"ping_addr": "8.8.8.8",
				"ping_count": 3,
			},
		},
		Layer4: LayerConfig{
			Enabled: true,
			Timeout: 10 * time.Second,
			Targets: []string{"8.8.8.8:53", "1.1.1.1:53"},
			Options: map[string]any{
				"udp_addr": "8.8.8.8:53",
			},
		},
		Layer5: LayerConfig{
			Enabled: true,
			Timeout: 15 * time.Second,
			Targets: []string{"8.8.8.8:53", "1.1.1.1:53"},
		},
		Layer6: LayerConfig{
			Enabled: true,
			Timeout: 10 * time.Second,
		},
		Layer7: LayerConfig{
			Enabled: true,
			Timeout: 15 * time.Second,
			Targets: []string{
				"https://www.google.com",
				"https://www.cloudflare.com",
			},
		},
	}

	// Create test session
	session, err := NewTestSession(config)
	if err != nil {
		return nil, err
	}

	// Run selected layers
	return session.RunSelectedLayers(selectedLayers)
}

// InitializeLogger creates and configures a new logger instance
func InitializeLogger() (*zap.Logger, func(), error) {
	logger, err := initializeLogger("info")
	if err != nil {
		return nil, nil, err
	}
	
	return logger, func() { _ = logger.Sync() }, nil
}

// ExecuteLayers runs tests for all specified layers
func ExecuteLayers(runners []common.LayerRunner, opts Options) []common.TestResult {
	// Create default config
	config := &Config{
		OutputFormat: opts.OutputFormat,
		LogLevel:     "info",
	}
	
	// Create test session
	session, err := NewTestSession(config)
	if err != nil {
		fmt.Printf("Failed to create test session: %v\n", err)
		return nil
	}
	
	// Create context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Run tests sequentially
	var results []common.TestResult
	for i, runner := range runners {
		// Default to layer number based on position + 1
		layer := i + 1
		
		// Run test
		layerResults, err := runner.RunTests(ctx, session.Logger)
		if err != nil {
			session.Logger.Error("Layer test failed",
				zap.Int("layer", layer),
				zap.Error(err),
			)
		}
		
		// Add results
		results = append(results, layerResults...)
	}
	
	// Generate report based on format
	generator := common.NewReportGenerator(results, "layer_tests")
	_, err = generator.GenerateReport(common.ReportFormat(opts.OutputFormat))
	if err != nil {
		session.Logger.Error("Failed to generate report", zap.Error(err))
	}
	
	return results
}