package layers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"ghostshell/app/layers/common"

	"github.com/jung-kurt/gofpdf"
	"go.uber.org/zap" // Importing the Zap logger
)

// WriteOutput writes the test results to the specified format and file.
func WriteOutput(results []common.TestResult, format string, outputPath string) error {
	logger, _ := zap.NewProduction() // Create a new logger instance
	defer logger.Sync()              // Flushes buffer, if any

	switch format {
	case "csv":
		if err := writeCSV(results, outputPath, logger); err != nil {
			logger.Error("Failed to write CSV", zap.String("outputPath", outputPath), zap.Error(err))
			return err
		}
	case "pdf":
		if err := writePDF(results, outputPath, logger); err != nil {
			logger.Error("Failed to write PDF", zap.String("outputPath", outputPath), zap.Error(err))
			return err
		}
	case "json":
		if err := writeJSON(results, outputPath, logger); err != nil {
			logger.Error("Failed to write JSON", zap.String("outputPath", outputPath), zap.Error(err))
			return err
		}
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}

	logger.Info("Report written successfully", zap.String("format", format), zap.String("outputPath", outputPath))
	return nil
}

// writeCSV generates a CSV report of the test results.
func writeCSV(results []common.TestResult, outputPath string, logger *zap.Logger) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("error creating CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write CSV header
	if err := writer.Write([]string{"Layer", "Status", "Message"}); err != nil {
		return fmt.Errorf("error writing CSV header: %w", err)
	}

	// Write test results
	for _, result := range results {
		// Add test result to the appropriate row
		row := []string{
			fmt.Sprintf("%d", result.Layer),
			result.Name,
			string(result.Status),
			result.Message,
			result.StartTime.Format(time.RFC3339),
			result.EndTime.Format(time.RFC3339),
			fmt.Sprintf("%.2f", result.Metrics.Duration.Milliseconds()),
			fmt.Sprintf("%.2f", result.Metrics.TransferRate),
			fmt.Sprintf("%.2f", result.Metrics.Latency.Milliseconds()),
			fmt.Sprintf("%.2f", result.Metrics.PacketLoss),
			fmt.Sprintf("%.2f", result.Metrics.ResponseTime.Milliseconds()),
		}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("error writing CSV row: %w", err)
		}
	}

	logger.Info("CSV report written", zap.String("outputPath", outputPath))
	return nil
}

// writePDF generates a PDF report of the test results.
func writePDF(results []common.TestResult, outputPath string, logger *zap.Logger) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)

	// Add title
	pdf.Cell(40, 10, "OSI Layer Test Report")
	pdf.Ln(12)

	// Table header
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(30, 10, "Layer")
	pdf.Cell(40, 10, "Status")
	pdf.Cell(120, 10, "Message")
	pdf.Ln(10)

	// Table rows
	pdf.SetFont("Arial", "", 12)
	for _, result := range results {
		pdf.Cell(30, 10, fmt.Sprintf("%d", result.Layer))
		pdf.Cell(40, 10, string(result.Status))
		pdf.MultiCell(120, 10, result.Message, "", "", false)
	}

	// Save PDF file
	if err := pdf.OutputFileAndClose(outputPath); err != nil {
		return fmt.Errorf("error writing PDF file: %w", err)
	}

	logger.Info("PDF report written", zap.String("outputPath", outputPath))
	return nil
}

// writeJSON generates a JSON report of the test results.
func writeJSON(results []common.TestResult, outputPath string, logger *zap.Logger) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("error creating JSON file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(results); err != nil {
		return fmt.Errorf("error writing JSON file: %w", err)
	}

	logger.Info("JSON report written", zap.String("outputPath", outputPath))
	return nil
}
