// Package layers provides OSI layer testing functionality
package layers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"ghostshell/app/layers/common"
)

// API represents the REST API for the Layers testing system
type API struct {
	Router       *mux.Router
	Config       *Config
	Logger       *zap.Logger
	ActiveTests  map[string]*TestSession
	ResultsCache map[string][]common.TestResult
}

// NewAPI creates a new API instance
func NewAPI(config *Config) (*API, error) {
	// Create logger
	logger, err := initializeLogger(config.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize API logger: %w", err)
	}

	// Create API
	api := &API{
		Router:       mux.NewRouter(),
		Config:       config,
		Logger:       logger,
		ActiveTests:  make(map[string]*TestSession),
		ResultsCache: make(map[string][]common.TestResult),
	}

	// Register routes
	api.registerRoutes()

	return api, nil
}

// registerRoutes sets up the API routes
func (api *API) registerRoutes() {
	// API version prefix
	v1 := api.Router.PathPrefix("/api/v1").Subrouter()

	// Layer testing endpoints
	v1.HandleFunc("/tests", api.handleGetAllTests).Methods("GET")
	v1.HandleFunc("/tests", api.handleCreateTest).Methods("POST")
	v1.HandleFunc("/tests/{id}", api.handleGetTest).Methods("GET")
	v1.HandleFunc("/tests/{id}/cancel", api.handleCancelTest).Methods("POST")
	v1.HandleFunc("/tests/{id}/results", api.handleGetTestResults).Methods("GET")

	// Configuration endpoints
	v1.HandleFunc("/config", api.handleGetConfig).Methods("GET")
	v1.HandleFunc("/config", api.handleUpdateConfig).Methods("PUT")
	v1.HandleFunc("/config/reset", api.handleResetConfig).Methods("POST")

	// Layer-specific endpoints
	v1.HandleFunc("/layers", api.handleGetLayers).Methods("GET")
	v1.HandleFunc("/layers/{layer}", api.handleGetLayerInfo).Methods("GET")
	v1.HandleFunc("/layers/{layer}/config", api.handleGetLayerConfig).Methods("GET")
	v1.HandleFunc("/layers/{layer}/config", api.handleUpdateLayerConfig).Methods("PUT")

	// History endpoints
	v1.HandleFunc("/history", api.handleGetHistory).Methods("GET")
	v1.HandleFunc("/history/{id}", api.handleGetHistoryItem).Methods("GET")
	v1.HandleFunc("/history/compare", api.handleCompareHistory).Methods("POST")

	// Report endpoints
	v1.HandleFunc("/reports", api.handleGetReports).Methods("GET")
	v1.HandleFunc("/reports/generate", api.handleGenerateReport).Methods("POST")
}

// Run starts the API server
func (api *API) Run(addr string) error {
	api.Logger.Info("Starting API server", zap.String("address", addr))
	return http.ListenAndServe(addr, api.Router)
}

// Test Management API Handlers

// handleGetAllTests returns all tests (active and completed)
func (api *API) handleGetAllTests(w http.ResponseWriter, r *http.Request) {
	// Create response struct
	type TestInfo struct {
		ID        string    `json:"id"`
		Status    string    `json:"status"`
		StartTime time.Time `json:"start_time"`
		EndTime   time.Time `json:"end_time,omitempty"`
		Layers    []int     `json:"layers"`
	}

	// Collect active tests
	tests := make([]TestInfo, 0, len(api.ActiveTests))
	for id, session := range api.ActiveTests {
		tests = append(tests, TestInfo{
			ID:        id,
			Status:    "running",
			StartTime: session.StartTime,
			Layers:    api.Config.GetEnabledLayers(),
		})
	}

	// TODO: Add completed tests from history

	api.respondWithJSON(w, http.StatusOK, tests)
}

// handleCreateTest starts a new test session
func (api *API) handleCreateTest(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	type TestRequest struct {
		Layers []int                  `json:"layers"`
		Config map[string]interface{} `json:"config,omitempty"`
	}

	var req TestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Create test session with default config
	config := api.Config
	if req.Config != nil {
		// Apply any config overrides
		// In a real implementation, this would merge req.Config into api.Config
	}

	session, err := NewTestSession(config)
	if err != nil {
		api.respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create test session: %v", err))
		return
	}

	// Store session
	api.ActiveTests[session.RunID] = session

	// Run tests in a goroutine
	go func() {
		var results []common.TestResult
		var err error

		if len(req.Layers) > 0 {
			results, err = session.RunSelectedLayers(req.Layers)
		} else {
			results, err = session.RunAllTests()
		}

		// Store results
		api.ResultsCache[session.RunID] = results

		// Remove from active tests
		delete(api.ActiveTests, session.RunID)

		// Log any errors
		if err != nil {
			api.Logger.Error("Test session failed", zap.String("id", session.RunID), zap.Error(err))
		}
	}()

	// Return session ID
	api.respondWithJSON(w, http.StatusCreated, map[string]string{
		"id":      session.RunID,
		"status":  "running",
		"message": "Test session started successfully",
	})
}

// handleGetTest returns information about a specific test
func (api *API) handleGetTest(w http.ResponseWriter, r *http.Request) {
	// Get test ID from URL
	vars := mux.Vars(r)
	id := vars["id"]

	// Check if test is active
	if session, ok := api.ActiveTests[id]; ok {
		// Test is active
		api.respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"id":         id,
			"status":     "running",
			"start_time": session.StartTime,
			"layers":     api.Config.GetEnabledLayers(),
		})
		return
	}

	// Check if test results are in cache
	if _, ok := api.ResultsCache[id]; ok {
		// Test is completed
		api.respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"id":      id,
			"status":  "completed",
			"message": "Test completed. Use /tests/{id}/results to get results.",
		})
		return
	}

	// TODO: Check if test is in history

	// Test not found
	api.respondWithError(w, http.StatusNotFound, "Test not found")
}

// handleCancelTest cancels an active test
func (api *API) handleCancelTest(w http.ResponseWriter, r *http.Request) {
	// Get test ID from URL
	vars := mux.Vars(r)
	id := vars["id"]

	// Check if test is active
	if _, ok := api.ActiveTests[id]; ok {
		// TODO: Implement cancellation mechanism
		// This would typically involve using a cancellation context

		api.respondWithJSON(w, http.StatusOK, map[string]string{
			"message": "Test cancellation requested",
		})
		return
	}

	// Test not active
	api.respondWithError(w, http.StatusNotFound, "No active test with that ID")
}

// handleGetTestResults returns the results of a test
func (api *API) handleGetTestResults(w http.ResponseWriter, r *http.Request) {
	// Get test ID from URL
	vars := mux.Vars(r)
	id := vars["id"]

	// Check if test is active
	if _, ok := api.ActiveTests[id]; ok {
		api.respondWithJSON(w, http.StatusAccepted, map[string]string{
			"message": "Test is still running",
		})
		return
	}

	// Check if test results are in cache
	if results, ok := api.ResultsCache[id]; ok {
		api.respondWithJSON(w, http.StatusOK, results)
		return
	}

	// TODO: Try to load results from history

	// Results not found
	api.respondWithError(w, http.StatusNotFound, "Test results not found")
}

// Configuration API Handlers

// handleGetConfig returns the current configuration
func (api *API) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	api.respondWithJSON(w, http.StatusOK, api.Config)
}

// handleUpdateConfig updates the configuration
func (api *API) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var newConfig Config
	if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
		api.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Validate config
	if err := newConfig.ValidateConfig(); err != nil {
		api.respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Invalid configuration: %v", err))
		return
	}

	// Update config
	api.Config = &newConfig

	// Save config to file
	configPath := "config.json"
	if err := SaveConfig(api.Config, configPath); err != nil {
		api.Logger.Error("Failed to save config", zap.Error(err))
		// Continue anyway, just log the error
	}

	api.respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Configuration updated successfully",
	})
}

// handleResetConfig resets the configuration to defaults
func (api *API) handleResetConfig(w http.ResponseWriter, r *http.Request) {
	// Create default config
	api.Config = &Config{
		OutputFormat:  "pdf",
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
	}

	// Apply default layer configs
	// This is simplified - in a real implementation, set all layer configs
	api.Config.Layer1 = LayerConfig{
		Enabled:  true,
		Timeout:  5 * time.Second,
		Priority: 1,
		Options: map[string]any{
			"attempt_count": 3,
		},
	}

	api.respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Configuration reset to defaults",
	})
}

// Layer API Handlers

// handleGetLayers returns information about all layers
func (api *API) handleGetLayers(w http.ResponseWriter, r *http.Request) {
	// Create test session to get layer information
	session, err := NewTestSession(api.Config)
	if err != nil {
		api.respondWithError(w, http.StatusInternalServerError, "Failed to create session")
		return
	}

	// Initialize runners for all layers
	allLayers := []int{1, 2, 3, 4, 5, 6, 7}
	runners, err := session.initializeRunners(allLayers)
	if err != nil {
		api.respondWithError(w, http.StatusInternalServerError, "Failed to initialize runners")
		return
	}

	// Collect layer information
	type LayerInfo struct {
		ID           int      `json:"id"`
		Name         string   `json:"name"`
		Description  string   `json:"description"`
		Enabled      bool     `json:"enabled"`
		Dependencies []int    `json:"dependencies"`
		Priority     int      `json:"priority"`
		Tags         []string `json:"tags,omitempty"`
	}

	// Build layer info
	layerInfos := make([]LayerInfo, 0, len(runners))
	for layer, runner := range runners {
		config, err := api.Config.GetLayerConfig(layer)
		if err != nil {
			continue
		}

		layerInfos = append(layerInfos, LayerInfo{
			ID:           layer,
			Name:         runner.GetName(),
			Description:  runner.GetDescription(),
			Enabled:      config.Enabled,
			Dependencies: runner.GetDependencies(),
			Priority:     config.Priority,
			Tags:         config.Tags,
		})
	}

	api.respondWithJSON(w, http.StatusOK, layerInfos)
}

// handleGetLayerInfo returns information about a specific layer
func (api *API) handleGetLayerInfo(w http.ResponseWriter, r *http.Request) {
	// Get layer ID from URL
	vars := mux.Vars(r)
	layerStr := vars["layer"]
	layer, err := strconv.Atoi(layerStr)
	if err != nil {
		api.respondWithError(w, http.StatusBadRequest, "Invalid layer ID")
		return
	}

	// Validate layer
	if layer < 1 || layer > 7 {
		api.respondWithError(w, http.StatusBadRequest, "Layer ID must be between 1 and 7")
		return
	}

	// Create test session to get layer information
	session, err := NewTestSession(api.Config)
	if err != nil {
		api.respondWithError(w, http.StatusInternalServerError, "Failed to create session")
		return
	}

	// Initialize runner for this layer
	runners, err := session.initializeRunners([]int{layer})
	if err != nil {
		api.respondWithError(w, http.StatusInternalServerError, "Failed to initialize runner")
		return
	}

	runner, ok := runners[layer]
	if !ok {
		api.respondWithError(w, http.StatusNotFound, "Layer not found or disabled")
		return
	}

	// Get layer config
	config, err := api.Config.GetLayerConfig(layer)
	if err != nil {
		api.respondWithError(w, http.StatusInternalServerError, "Failed to get layer config")
		return
	}

	// Build response
	api.respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"id":           layer,
		"name":         runner.GetName(),
		"description":  runner.GetDescription(),
		"enabled":      config.Enabled,
		"dependencies": runner.GetDependencies(),
		"priority":     config.Priority,
		"timeout":      config.Timeout.String(),
		"tags":         config.Tags,
		"options":      config.Options,
	})
}

// handleGetLayerConfig returns the configuration for a specific layer
func (api *API) handleGetLayerConfig(w http.ResponseWriter, r *http.Request) {
	// Get layer ID from URL
	vars := mux.Vars(r)
	layerStr := vars["layer"]
	layer, err := strconv.Atoi(layerStr)
	if err != nil {
		api.respondWithError(w, http.StatusBadRequest, "Invalid layer ID")
		return
	}

	// Validate layer
	if layer < 1 || layer > 7 {
		api.respondWithError(w, http.StatusBadRequest, "Layer ID must be between 1 and 7")
		return
	}

	// Get layer config
	config, err := api.Config.GetLayerConfig(layer)
	if err != nil {
		api.respondWithError(w, http.StatusInternalServerError, "Failed to get layer config")
		return
	}

	api.respondWithJSON(w, http.StatusOK, config)
}

// handleUpdateLayerConfig updates the configuration for a specific layer
func (api *API) handleUpdateLayerConfig(w http.ResponseWriter, r *http.Request) {
	// Get layer ID from URL
	vars := mux.Vars(r)
	layerStr := vars["layer"]
	layer, err := strconv.Atoi(layerStr)
	if err != nil {
		api.respondWithError(w, http.StatusBadRequest, "Invalid layer ID")
		return
	}

	// Validate layer
	if layer < 1 || layer > 7 {
		api.respondWithError(w, http.StatusBadRequest, "Layer ID must be between 1 and 7")
		return
	}

	// Parse request body
	var newConfig LayerConfig
	if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
		api.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Update layer config
	switch layer {
	case 1:
		api.Config.Layer1 = newConfig
	case 2:
		api.Config.Layer2 = newConfig
	case 3:
		api.Config.Layer3 = newConfig
	case 4:
		api.Config.Layer4 = newConfig
	case 5:
		api.Config.Layer5 = newConfig
	case 6:
		api.Config.Layer6 = newConfig
	case 7:
		api.Config.Layer7 = newConfig
	}

	// Save config to file
	configPath := "config.json"
	if err := SaveConfig(api.Config, configPath); err != nil {
		api.Logger.Error("Failed to save config", zap.Error(err))
		// Continue anyway, just log the error
	}

	api.respondWithJSON(w, http.StatusOK, map[string]string{
		"message": fmt.Sprintf("Layer %d configuration updated successfully", layer),
	})
}

// History API Handlers

// handleGetHistory returns test history
func (api *API) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	limit := 10 // Default
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// List history directory
	historyDir := filepath.Join(common.MetricsDir, "history")
	files, err := os.ReadDir(historyDir)
	if err != nil {
		if os.IsNotExist(err) {
			// No history yet
			api.respondWithJSON(w, http.StatusOK, []interface{}{})
			return
		}
		api.respondWithError(w, http.StatusInternalServerError, "Failed to read history directory")
		return
	}

	// Process files
	type HistoryItem struct {
		ID        string    `json:"id"`
		Timestamp time.Time `json:"timestamp"`
		FilePath  string    `json:"file_path"`
	}

	var historyItems []HistoryItem
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		// Parse timestamp from filename
		name := file.Name()
		name = strings.TrimSuffix(name, ".json")
		parts := strings.Split(name, "_")
		if len(parts) < 3 {
			continue
		}

		// Extract timestamp from the last part
		timestampStr := parts[len(parts)-1]
		timestamp, err := time.Parse("20060102_150405", timestampStr)
		if err != nil {
			continue
		}

		historyItems = append(historyItems, HistoryItem{
			ID:        timestampStr,
			Timestamp: timestamp,
			FilePath:  filepath.Join(historyDir, file.Name()),
		})

		// Limit number of items
		if len(historyItems) >= limit {
			break
		}
	}

	api.respondWithJSON(w, http.StatusOK, historyItems)
}

// handleGetHistoryItem returns a specific history item
func (api *API) handleGetHistoryItem(w http.ResponseWriter, r *http.Request) {
	// Get history ID from URL
	vars := mux.Vars(r)
	id := vars["id"]

	// Construct file path
	filePath := filepath.Join(common.MetricsDir, "history", fmt.Sprintf("layer_tests_%s.json", id))

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		api.respondWithError(w, http.StatusNotFound, "History item not found")
		return
	}

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		api.respondWithError(w, http.StatusInternalServerError, "Failed to read history file")
		return
	}

	// Parse JSON
	var results []common.TestResult
	if err := json.Unmarshal(data, &results); err != nil {
		api.respondWithError(w, http.StatusInternalServerError, "Failed to parse history file")
		return
	}

	api.respondWithJSON(w, http.StatusOK, results)
}

// handleCompareHistory compares two history items
func (api *API) handleCompareHistory(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	type CompareRequest struct {
		BaseID    string `json:"base_id"`
		CompareID string `json:"compare_id"`
	}

	var req CompareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Load base results
	baseFilePath := filepath.Join(common.MetricsDir, "history", fmt.Sprintf("layer_tests_%s.json", req.BaseID))
	baseData, err := os.ReadFile(baseFilePath)
	if err != nil {
		api.respondWithError(w, http.StatusNotFound, "Base history item not found")
		return
	}

	var baseResults []common.TestResult
	if err := json.Unmarshal(baseData, &baseResults); err != nil {
		api.respondWithError(w, http.StatusInternalServerError, "Failed to parse base history file")
		return
	}

	// Load compare results
	compareFilePath := filepath.Join(common.MetricsDir, "history", fmt.Sprintf("layer_tests_%s.json", req.CompareID))
	compareData, err := os.ReadFile(compareFilePath)
	if err != nil {
		api.respondWithError(w, http.StatusNotFound, "Compare history item not found")
		return
	}

	var compareResults []common.TestResult
	if err := json.Unmarshal(compareData, &compareResults); err != nil {
		api.respondWithError(w, http.StatusInternalServerError, "Failed to parse compare history file")
		return
	}

	// Perform comparison
	// In a real implementation, this would be much more sophisticated
	type ComparisonResult struct {
		Layer            int     `json:"layer"`
		Name             string  `json:"name"`
		BaseStatus       string  `json:"base_status"`
		CompareStatus    string  `json:"compare_status"`
		StatusChanged    bool    `json:"status_changed"`
		LatencyDiff      float64 `json:"latency_diff_ms,omitempty"`
		PacketLossDiff   float64 `json:"packet_loss_diff_pct,omitempty"`
		TransferRateDiff float64 `json:"transfer_rate_diff_mb_s,omitempty"`
	}

	var comparison []ComparisonResult

	// Simple comparison by layer
	for _, baseResult := range baseResults {
		// Find matching result in compare set
		for _, compareResult := range compareResults {
			if baseResult.Layer == compareResult.Layer && baseResult.Name == compareResult.Name {
				comp := ComparisonResult{
					Layer:         baseResult.Layer,
					Name:          baseResult.Name,
					BaseStatus:    string(baseResult.Status),
					CompareStatus: string(compareResult.Status),
					StatusChanged: baseResult.Status != compareResult.Status,
				}

				// Compare metrics
				if baseResult.Metrics.Latency > 0 && compareResult.Metrics.Latency > 0 {
					comp.LatencyDiff = float64(compareResult.Metrics.Latency.Milliseconds() - baseResult.Metrics.Latency.Milliseconds())
				}

				if baseResult.Metrics.PacketLoss > 0 || compareResult.Metrics.PacketLoss > 0 {
					comp.PacketLossDiff = compareResult.Metrics.PacketLoss - baseResult.Metrics.PacketLoss
				}

				if baseResult.Metrics.TransferRate > 0 || compareResult.Metrics.TransferRate > 0 {
					comp.TransferRateDiff = compareResult.Metrics.TransferRate - baseResult.Metrics.TransferRate
				}

				comparison = append(comparison, comp)
				break
			}
		}
	}

	api.respondWithJSON(w, http.StatusOK, comparison)
}

// Report API Handlers

// handleGetReports returns available reports
func (api *API) handleGetReports(w http.ResponseWriter, r *http.Request) {
	// List report directory
	files, err := os.ReadDir(common.ReportDir)
	if err != nil {
		if os.IsNotExist(err) {
			// No reports yet
			api.respondWithJSON(w, http.StatusOK, []interface{}{})
			return
		}
		api.respondWithError(w, http.StatusInternalServerError, "Failed to read report directory")
		return
	}

	// Process files
	type ReportItem struct {
		ID        string    `json:"id"`
		Timestamp time.Time `json:"timestamp"`
		Format    string    `json:"format"`
		FilePath  string    `json:"file_path"`
	}

	var reportItems []ReportItem
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		name := file.Name()
		format := filepath.Ext(name)
		if format == "" {
			continue
		}
		format = format[1:] // Remove the dot

		// Try to parse timestamp from filename
		parts := strings.Split(name, "_")
		if len(parts) < 2 {
			continue
		}

		timestampPart := parts[len(parts)-1]
		timestampPart = strings.TrimSuffix(timestampPart, format)
		timestampPart = strings.TrimSuffix(timestampPart, ".")
		timestamp, err := time.Parse("20060102_150405", timestampPart)
		if err != nil {
			continue
		}

		reportItems = append(reportItems, ReportItem{
			ID:        timestampPart,
			Timestamp: timestamp,
			Format:    format,
			FilePath:  filepath.Join(common.ReportDir, name),
		})
	}

	api.respondWithJSON(w, http.StatusOK, reportItems)
}

// handleGenerateReport generates a report from test results
func (api *API) handleGenerateReport(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	type ReportRequest struct {
		TestID  string         `json:"test_id"`
		Format  string         `json:"format"`
		Options map[string]any `json:"options,omitempty"`
	}

	var req ReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Validate format
	validFormats := map[string]bool{
		"csv":  true,
		"pdf":  true,
		"json": true,
		"yaml": true,
		"html": true,
		"md":   true,
		"xml":  true,
	}
	if !validFormats[req.Format] {
		api.respondWithError(w, http.StatusBadRequest, "Invalid format")
		return
	}

	// Get test results
	var results []common.TestResult

	// Check if test is in cache
	if cachedResults, ok := api.ResultsCache[req.TestID]; ok {
		results = cachedResults
	} else {
		// Try to load from history
		historyPath := filepath.Join(common.MetricsDir, "history", fmt.Sprintf("layer_tests_%s.json", req.TestID))
		if _, err := os.Stat(historyPath); os.IsNotExist(err) {
			api.respondWithError(w, http.StatusNotFound, "Test results not found")
			return
		}

		// Read history file
		data, err := os.ReadFile(historyPath)
		if err != nil {
			api.respondWithError(w, http.StatusInternalServerError, "Failed to read history file")
			return
		}

		// Parse JSON
		if err := json.Unmarshal(data, &results); err != nil {
			api.respondWithError(w, http.StatusInternalServerError, "Failed to parse history file")
			return
		}
	}

	// Create report generator
	generator := common.NewReportGenerator(results, "layer_tests")

	// Generate report
	reportPath, err := generator.GenerateReport(common.ReportFormat(req.Format))
	if err != nil {
		api.respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to generate report: %v", err))
		return
	}

	// Return report info
	api.respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Report generated successfully",
		"path":    reportPath,
		"format":  req.Format,
		"test_id": req.TestID,
	})
}

// Helper methods

// respondWithError returns an error response
func (api *API) respondWithError(w http.ResponseWriter, code int, message string) {
	api.respondWithJSON(w, code, map[string]string{"error": message})
}

// respondWithJSON returns a JSON response
func (api *API) respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		api.Logger.Error("Failed to marshal JSON response", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"Internal server error"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
