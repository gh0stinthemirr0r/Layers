// Package layer6 implements presentation layer testing functionality
package layer6

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"ghostshell/app/layers/common"
)

// Runner implements presentation layer tests
type Runner struct {
	*common.Layer6Runner
}

// New creates a new Layer6Runner
func New(dataSets []map[string]string) *Runner {
	return &Runner{
		Layer6Runner: &common.Layer6Runner{
			DataSets: dataSets,
		},
	}
}

// RunTests implements the LayerRunner interface
func (r *Runner) RunTests(ctx context.Context, logger *zap.Logger) ([]common.TestResult, error) {
	logger.Info("Starting Layer 6 (Presentation Layer) tests...")

	startTime := time.Now()

	// Create parent result
	parentResult := common.TestResult{
		Layer:      6,
		Name:       "Presentation Layer Tests",
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

		// Test data encoding/decoding for each dataset
		for i, data := range r.DataSets {
			// JSON transformation test
			jsonResult := common.TestResult{
				Layer:     6,
				Name:      fmt.Sprintf("JSON Transformation Test (Dataset %d)", i+1),
				StartTime: time.Now(),
			}

			success, msg, jsonDetails := testJSONTransformation(data)
			if !success {
				jsonResult.Status = common.StatusFailed
				jsonResult.Message = msg
				failedTests = append(failedTests, msg)
			} else {
				jsonResult.Status = common.StatusPassed
				jsonResult.Message = msg
			}

			jsonResult.Diagnostics = jsonDetails
			jsonResult.EndTime = time.Now()
			jsonResult.Metrics.Duration = jsonResult.EndTime.Sub(jsonResult.StartTime)
			parentResult.SubResults = append(parentResult.SubResults, jsonResult)

			// Base64 transformation test
			base64Result := common.TestResult{
				Layer:     6,
				Name:      fmt.Sprintf("Base64 Transformation Test (Dataset %d)", i+1),
				StartTime: time.Now(),
			}

			success, msg, base64Details := testBase64Transformation(data)
			if !success {
				base64Result.Status = common.StatusFailed
				base64Result.Message = msg
				failedTests = append(failedTests, msg)
			} else {
				base64Result.Status = common.StatusPassed
				base64Result.Message = msg
			}

			base64Result.Diagnostics = base64Details
			base64Result.EndTime = time.Now()
			base64Result.Metrics.Duration = base64Result.EndTime.Sub(base64Result.StartTime)
			parentResult.SubResults = append(parentResult.SubResults, base64Result)
		}

		// Set overall test status and message
		if len(failedTests) > 0 {
			parentResult.Status = common.StatusFailed
			parentResult.Message = fmt.Sprintf("Layer 6 tests failed with %d failures:\n\n%s",
				len(failedTests), strings.Join(failedTests, "\n\n"))
			logger.Error(parentResult.Message)
		} else {
			parentResult.Status = common.StatusPassed
			parentResult.Message = fmt.Sprintf("All Layer 6 tests passed successfully:\n"+
				"- Datasets tested: %d\n"+
				"- Total transformations: %d",
				len(r.DataSets), len(r.DataSets)*2)
			logger.Info(parentResult.Message)
		}

		parentResult.EndTime = time.Now()
		parentResult.Metrics.Duration = parentResult.EndTime.Sub(parentResult.StartTime)

		if len(failedTests) > 0 {
			return []common.TestResult{parentResult}, fmt.Errorf("layer 6 tests failed")
		}
		return []common.TestResult{parentResult}, nil
	}
}

// testJSONTransformation tests JSON encoding and decoding
func testJSONTransformation(data map[string]string) (bool, string, map[string]interface{}) {
	diagnostics := make(map[string]interface{})
	diagnostics["data_size"] = len(data)

	// Try to marshal to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		diagnostics["error"] = err.Error()
		diagnostics["stage"] = "encoding"
		return false, fmt.Sprintf("JSON encoding failed: %v", err), diagnostics
	}
	diagnostics["encoded_size"] = len(jsonData)

	// Try to unmarshal back
	var decoded map[string]string
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		diagnostics["error"] = err.Error()
		diagnostics["stage"] = "decoding"
		return false, fmt.Sprintf("JSON decoding failed: %v", err), diagnostics
	}

	// Verify data integrity
	if len(decoded) != len(data) {
		diagnostics["error"] = "Data size mismatch"
		diagnostics["original_size"] = len(data)
		diagnostics["decoded_size"] = len(decoded)
		return false, "JSON transformation failed: data size mismatch", diagnostics
	}

	for k, v := range data {
		if decoded[k] != v {
			diagnostics["error"] = "Data content mismatch"
			diagnostics["mismatched_key"] = k
			return false, "JSON transformation failed: data content mismatch", diagnostics
		}
	}

	diagnostics["stage"] = "complete"
	diagnostics["success"] = true
	return true, "JSON transformation successful", diagnostics
}

// testBase64Transformation tests Base64 encoding and decoding
func testBase64Transformation(data map[string]string) (bool, string, map[string]interface{}) {
	diagnostics := make(map[string]interface{})
	diagnostics["data_size"] = len(data)

	// Convert map to JSON first
	jsonData, err := json.Marshal(data)
	if err != nil {
		diagnostics["error"] = err.Error()
		diagnostics["stage"] = "json_encoding"
		return false, fmt.Sprintf("Base64 pre-processing failed: %v", err), diagnostics
	}

	// Encode to Base64
	encoded := base64.StdEncoding.EncodeToString(jsonData)
	diagnostics["encoded_size"] = len(encoded)

	// Decode from Base64
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		diagnostics["error"] = err.Error()
		diagnostics["stage"] = "base64_decoding"
		return false, fmt.Sprintf("Base64 decoding failed: %v", err), diagnostics
	}

	// Verify data integrity
	if len(decoded) != len(jsonData) {
		diagnostics["error"] = "Data size mismatch"
		diagnostics["original_size"] = len(jsonData)
		diagnostics["decoded_size"] = len(decoded)
		return false, "Base64 transformation failed: data size mismatch", diagnostics
	}

	// Try to unmarshal back to verify data
	var finalData map[string]string
	if err := json.Unmarshal(decoded, &finalData); err != nil {
		diagnostics["error"] = err.Error()
		diagnostics["stage"] = "json_decoding"
		return false, fmt.Sprintf("Base64 post-processing failed: %v", err), diagnostics
	}

	// Verify content
	for k, v := range data {
		if finalData[k] != v {
			diagnostics["error"] = "Data content mismatch"
			diagnostics["mismatched_key"] = k
			return false, "Base64 transformation failed: data content mismatch", diagnostics
		}
	}

	diagnostics["stage"] = "complete"
	diagnostics["success"] = true
	return true, "Base64 transformation successful", diagnostics
}

// GetDependencies returns the layer numbers this layer depends on
func (r *Runner) GetDependencies() []int {
	return []int{1, 2, 3, 4, 5} // Layer 6 depends on Layers 1-5
}

// ValidateConfig validates the configuration for this layer
func (r *Runner) ValidateConfig() error {
	if len(r.DataSets) == 0 {
		return fmt.Errorf("at least one data set must be specified")
	}
	for i, dataset := range r.DataSets {
		if len(dataset) == 0 {
			return fmt.Errorf("data set %d is empty", i+1)
		}
	}
	return nil
}

// GetDescription returns a description of this layer's functionality
func (r *Runner) GetDescription() string {
	return "Tests presentation layer functionality including data encoding and encryption"
}

// GetName returns the name of this layer
func (r *Runner) GetName() string {
	return "Presentation Layer"
}
