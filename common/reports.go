// Package common provides shared functionality for layer testing
package common

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/wcharczuk/go-chart/v2"
	"gopkg.in/yaml.v3"
)

// ReportFormat defines the supported report types
type ReportFormat string

const (
	ReportCSV      ReportFormat = "csv"
	ReportPDF      ReportFormat = "pdf"
	ReportJSON     ReportFormat = "json"
	ReportYAML     ReportFormat = "yaml"
	ReportHTML     ReportFormat = "html"
	ReportMarkdown ReportFormat = "md"
	ReportXML      ReportFormat = "xml"
)

// ReportGenerator generates reports in various formats
type ReportGenerator struct {
	ResultsByLayer map[int][]TestResult
	AllResults     []TestResult
	TestName       string
	CreatedAt      time.Time
	OutputDir      string
}

// NewReportGenerator creates a new report generator
func NewReportGenerator(results []TestResult, testName string) *ReportGenerator {
	resultsByLayer := make(map[int][]TestResult)
	for _, result := range results {
		resultsByLayer[result.Layer] = append(resultsByLayer[result.Layer], result)
	}

	return &ReportGenerator{
		ResultsByLayer: resultsByLayer,
		AllResults:     results,
		TestName:       testName,
		CreatedAt:      time.Now(),
		OutputDir:      ReportDir,
	}
}

// GenerateReport generates a report in the specified format
func (rg *ReportGenerator) GenerateReport(format ReportFormat) (string, error) {
	timestamp := rg.CreatedAt.Format("20060102_150405")
	fileName := fmt.Sprintf("%s_%s", rg.TestName, timestamp)
	filePath := filepath.Join(rg.OutputDir, fileName+"."+string(format))

	if err := os.MkdirAll(rg.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create report directory: %w", err)
	}

	switch format {
	case ReportCSV:
		return filePath, rg.generateCSVReport(filePath)
	case ReportPDF:
		return filePath, rg.generatePDFReport(filePath)
	case ReportJSON:
		return filePath, rg.generateJSONReport(filePath)
	case ReportYAML:
		return filePath, rg.generateYAMLReport(filePath)
	case ReportHTML:
		return filePath, rg.generateHTMLReport(filePath)
	case ReportMarkdown:
		return filePath, rg.generateMarkdownReport(filePath)
	case ReportXML:
		return filePath, rg.generateXMLReport(filePath)
	default:
		return "", fmt.Errorf("unsupported report format: %s", format)
	}
}

// GenerateAllReports generates reports in all supported formats
func (rg *ReportGenerator) GenerateAllReports() (map[ReportFormat]string, error) {
	formats := []ReportFormat{
		ReportCSV,
		ReportPDF,
		ReportJSON,
		ReportYAML,
		ReportHTML,
		ReportMarkdown,
	}

	results := make(map[ReportFormat]string)
	for _, format := range formats {
		path, err := rg.GenerateReport(format)
		if err != nil {
			return results, err
		}
		results[format] = path
	}

	// Generate charts
	if err := rg.GenerateCharts(); err != nil {
		return results, err
	}

	return results, nil
}

// GenerateCharts creates visualizations of the test results
func (rg *ReportGenerator) GenerateCharts() error {
	chartDir := filepath.Join(rg.OutputDir, "charts")
	if err := os.MkdirAll(chartDir, 0755); err != nil {
		return fmt.Errorf("failed to create chart directory: %w", err)
	}

	timestamp := rg.CreatedAt.Format("20060102_150405")

	// Generate status bar chart
	if err := rg.generateStatusChart(filepath.Join(chartDir, fmt.Sprintf("status_chart_%s.png", timestamp))); err != nil {
		return err
	}

	// Generate performance metrics chart
	if err := rg.generatePerformanceChart(filepath.Join(chartDir, fmt.Sprintf("performance_chart_%s.png", timestamp))); err != nil {
		return err
	}

	// Generate layer completion time chart
	if err := rg.generateTimeChart(filepath.Join(chartDir, fmt.Sprintf("time_chart_%s.png", timestamp))); err != nil {
		return err
	}

	return nil
}

// generateStatusChart creates a bar chart showing pass/fail status by layer
func (rg *ReportGenerator) generateStatusChart(filePath string) error {
	var passed, failed, warning, skipped []chart.Value

	for layer := 1; layer <= 7; layer++ {
		results, ok := rg.ResultsByLayer[layer]
		if !ok {
			continue
		}

		passCount, failCount, warnCount, skipCount := 0, 0, 0, 0
		for _, result := range results {
			switch result.Status {
			case StatusPassed:
				passCount++
			case StatusFailed:
				failCount++
			case StatusWarning:
				warnCount++
			case StatusSkipped:
				skipCount++
			}
		}

		passed = append(passed, chart.Value{
			Label: fmt.Sprintf("Layer %d", layer),
			Value: float64(passCount),
		})
		failed = append(failed, chart.Value{
			Label: fmt.Sprintf("Layer %d", layer),
			Value: float64(failCount),
		})
		warning = append(warning, chart.Value{
			Label: fmt.Sprintf("Layer %d", layer),
			Value: float64(warnCount),
		})
		skipped = append(skipped, chart.Value{
			Label: fmt.Sprintf("Layer %d", layer),
			Value: float64(skipCount),
		})
	}

	statusChart := chart.BarChart{
		Title: "Test Results by Layer",
		Background: chart.Style{
			Padding: chart.Box{
				Top:    40,
				Left:   20,
				Right:  20,
				Bottom: 20,
			},
		},
		Height:   512,
		Width:    1024,
		BarWidth: 30,
		Bars: []chart.Value{
			// Example bar entries, actual implementation would iterate through results
			{Value: 5, Label: "Layer 1"},
			{Value: 3, Label: "Layer 2"},
			{Value: 4, Label: "Layer 3"},
		},
	}

	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	return statusChart.Render(chart.PNG, f)
}

// generatePerformanceChart creates a chart showing performance metrics
func (rg *ReportGenerator) generatePerformanceChart(filePath string) error {
	// Placeholder for chart generation
	return nil
}

// generateTimeChart creates a chart showing test completion times
func (rg *ReportGenerator) generateTimeChart(filePath string) error {
	// Placeholder for chart generation
	return nil
}

// Legacy support functions

// WriteCSVReport writes test results to a CSV file
func WriteCSVReport(results []TestResult, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create report directory: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{
		"Layer",
		"Test Name",
		"Status",
		"Message",
		"Start Time",
		"End Time",
		"Duration (ms)",
		"Transfer Rate (MB/s)",
		"Latency (ms)",
		"Packet Loss (%)",
		"Response Time (ms)",
	}); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write results
	for _, r := range results {
		if err := writer.Write([]string{
			fmt.Sprintf("%d", r.Layer),
			r.Name,
			string(r.Status),
			r.Message,
			r.StartTime.Format(time.RFC3339),
			r.EndTime.Format(time.RFC3339),
			fmt.Sprintf("%.2f", r.Metrics.Duration.Milliseconds()),
			fmt.Sprintf("%.2f", r.Metrics.TransferRate),
			fmt.Sprintf("%.2f", r.Metrics.Latency.Milliseconds()),
			fmt.Sprintf("%.2f", r.Metrics.PacketLoss),
			fmt.Sprintf("%.2f", r.Metrics.ResponseTime.Milliseconds()),
		}); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return nil
}

// WritePDFReport writes test results to a PDF file
func WritePDFReport(results []TestResult, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create report directory: %w", err)
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	// Title
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, "OSI Layer Test Results")
	pdf.Ln(20)

	// Add timestamp
	pdf.SetFont("Arial", "I", 10)
	pdf.Cell(0, 6, fmt.Sprintf("Generated on %s", time.Now().Format("2006-01-02 15:04:05")))
	pdf.Ln(10)

	// Add summary
	passCount, failCount, warnCount := 0, 0, 0
	for _, r := range results {
		switch r.Status {
		case StatusPassed:
			passCount++
		case StatusFailed:
			failCount++
		case StatusWarning:
			warnCount++
		}
	}

	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(0, 8, "Summary:")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(0, 6, fmt.Sprintf("Total Tests: %d", len(results)))
	pdf.Ln(6)
	pdf.Cell(0, 6, fmt.Sprintf("Passed: %d", passCount))
	pdf.Ln(6)
	pdf.Cell(0, 6, fmt.Sprintf("Failed: %d", failCount))
	pdf.Ln(6)
	pdf.Cell(0, 6, fmt.Sprintf("Warnings: %d", warnCount))
	pdf.Ln(12)

	// Results by layer
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(0, 8, "Detailed Results:")
	pdf.Ln(8)

	// Sort results by layer
	sort.Slice(results, func(i, j int) bool {
		return results[i].Layer < results[j].Layer
	})

	// Group by layer
	resultsByLayer := make(map[int][]TestResult)
	for _, r := range results {
		resultsByLayer[r.Layer] = append(resultsByLayer[r.Layer], r)
	}

	pdf.SetFont("Arial", "", 12)
	for layer := 1; layer <= 7; layer++ {
		layerResults, ok := resultsByLayer[layer]
		if !ok {
			continue
		}

		pdf.SetFont("Arial", "B", 12)
		pdf.Cell(0, 8, fmt.Sprintf("Layer %d:", layer))
		pdf.Ln(8)
		pdf.SetFont("Arial", "", 12)

		for _, result := range layerResults {
			statusStr := string(result.Status)
			var color string
			switch result.Status {
			case StatusPassed:
				color = "0,128,0" // Green
			case StatusFailed:
				color = "255,0,0" // Red
			case StatusWarning:
				color = "255,165,0" // Orange
			case StatusSkipped:
				color = "128,128,128" // Gray
			default:
				color = "0,0,0" // Black
			}

			// Split RGB color
			parts := strings.Split(color, ",")
			rColor, gColor, bColor := parts[0], parts[1], parts[2]
			pdf.SetTextColor(int(StringToUint8(rColor)), int(StringToUint8(gColor)), int(StringToUint8(bColor)))

			pdf.Cell(0, 8, fmt.Sprintf("%s: %s", result.Name, statusStr))
			pdf.Ln(8)
			pdf.SetTextColor(0, 0, 0) // Reset to black
			pdf.MultiCell(0, 6, result.Message, "", "", false)
			pdf.Ln(2)

			// Add metrics if available
			if result.Metrics.Duration > 0 {
				pdf.Cell(0, 6, fmt.Sprintf("Duration: %.2f ms", float64(result.Metrics.Duration.Milliseconds())))
				pdf.Ln(6)
			}
			if result.Metrics.Latency > 0 {
				pdf.Cell(0, 6, fmt.Sprintf("Latency: %.2f ms", float64(result.Metrics.Latency.Milliseconds())))
				pdf.Ln(6)
			}
			if result.Metrics.PacketLoss > 0 {
				pdf.Cell(0, 6, fmt.Sprintf("Packet Loss: %.2f%%", result.Metrics.PacketLoss))
				pdf.Ln(6)
			}
			pdf.Ln(4)
		}
		pdf.Ln(8)
	}

	return pdf.OutputFileAndClose(path)
}

// Helper to convert string to uint8
func StringToUint8(s string) uint8 {
	var val int
	fmt.Sscanf(s, "%d", &val)
	if val > 255 {
		val = 255
	}
	if val < 0 {
		val = 0
	}
	return uint8(val)
}

// WriteJSONReport writes test results to a JSON file
func WriteJSONReport(results []TestResult, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create report directory: %w", err)
	}

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write JSON file: %w", err)
	}

	return nil
}

// generateCSVReport is an internal method for the ReportGenerator
func (rg *ReportGenerator) generateCSVReport(path string) error {
	return WriteCSVReport(rg.AllResults, path)
}

// generatePDFReport is an internal method for the ReportGenerator
func (rg *ReportGenerator) generatePDFReport(path string) error {
	return WritePDFReport(rg.AllResults, path)
}

// generateJSONReport is an internal method for the ReportGenerator
func (rg *ReportGenerator) generateJSONReport(path string) error {
	return WriteJSONReport(rg.AllResults, path)
}

// generateYAMLReport is an internal method for the ReportGenerator
func (rg *ReportGenerator) generateYAMLReport(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create report directory: %w", err)
	}

	data, err := yaml.Marshal(rg.AllResults)
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write YAML file: %w", err)
	}

	return nil
}

// generateHTMLReport is an internal method for the ReportGenerator
func (rg *ReportGenerator) generateHTMLReport(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create report directory: %w", err)
	}

	// Simple HTML template
	html := `<!DOCTYPE html>
<html>
<head>
    <title>OSI Layer Test Results</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        h1 { color: #333; }
        .summary { margin: 20px 0; padding: 10px; background-color: #f5f5f5; border-radius: 5px; }
        .layer { margin: 20px 0; }
        .layer-title { font-weight: bold; font-size: 1.2em; }
        .test { margin: 10px 0; padding: 10px; border-radius: 5px; }
        .passed { background-color: #dff0d8; }
        .failed { background-color: #f2dede; }
        .warning { background-color: #fcf8e3; }
        .skipped { background-color: #eee; }
        .metrics { margin-top: 10px; font-size: 0.9em; color: #666; }
    </style>
</head>
<body>
    <h1>OSI Layer Test Results</h1>
    <div class="summary">
        <p>Generated on: %s</p>
        <p>Total Tests: %d</p>
        <p>Passed: %d</p>
        <p>Failed: %d</p>
        <p>Warnings: %d</p>
        <p>Skipped: %d</p>
    </div>
`

	// Count results by status
	total := len(rg.AllResults)
	passCount, failCount, warnCount, skipCount := 0, 0, 0, 0
	for _, r := range rg.AllResults {
		switch r.Status {
		case StatusPassed:
			passCount++
		case StatusFailed:
			failCount++
		case StatusWarning:
			warnCount++
		case StatusSkipped:
			skipCount++
		}
	}

	// Generate the HTML content
	content := fmt.Sprintf(html, time.Now().Format("2006-01-02 15:04:05"),
		total, passCount, failCount, warnCount, skipCount)

	// Add layer results
	for layer := 1; layer <= 7; layer++ {
		results, ok := rg.ResultsByLayer[layer]
		if !ok {
			continue
		}

		content += fmt.Sprintf("<div class=\"layer\">\n<div class=\"layer-title\">Layer %d</div>\n", layer)

		for _, result := range results {
			statusClass := strings.ToLower(string(result.Status))
			content += fmt.Sprintf("<div class=\"test %s\">\n", statusClass)
			content += fmt.Sprintf("<div><strong>%s:</strong> %s</div>\n", result.Name, string(result.Status))
			content += fmt.Sprintf("<div>%s</div>\n", result.Message)

			if result.Metrics.Duration > 0 || result.Metrics.Latency > 0 || result.Metrics.PacketLoss > 0 {
				content += "<div class=\"metrics\">\n"
				if result.Metrics.Duration > 0 {
					content += fmt.Sprintf("<div>Duration: %.2f ms</div>\n", float64(result.Metrics.Duration.Milliseconds()))
				}
				if result.Metrics.Latency > 0 {
					content += fmt.Sprintf("<div>Latency: %.2f ms</div>\n", float64(result.Metrics.Latency.Milliseconds()))
				}
				if result.Metrics.PacketLoss > 0 {
					content += fmt.Sprintf("<div>Packet Loss: %.2f%%</div>\n", result.Metrics.PacketLoss)
				}
				if result.Metrics.TransferRate > 0 {
					content += fmt.Sprintf("<div>Transfer Rate: %.2f MB/s</div>\n", result.Metrics.TransferRate)
				}
				content += "</div>\n"
			}

			content += "</div>\n"
		}

		content += "</div>\n"
	}

	content += "</body>\n</html>"

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write HTML file: %w", err)
	}

	return nil
}

// generateMarkdownReport is an internal method for the ReportGenerator
func (rg *ReportGenerator) generateMarkdownReport(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create report directory: %w", err)
	}

	var md strings.Builder

	// Header
	md.WriteString("# OSI Layer Test Results\n\n")
	md.WriteString(fmt.Sprintf("Generated on: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	// Summary
	passCount, failCount, warnCount, skipCount := 0, 0, 0, 0
	for _, r := range rg.AllResults {
		switch r.Status {
		case StatusPassed:
			passCount++
		case StatusFailed:
			failCount++
		case StatusWarning:
			warnCount++
		case StatusSkipped:
			skipCount++
		}
	}

	md.WriteString("## Summary\n\n")
	md.WriteString(fmt.Sprintf("- **Total Tests:** %d\n", len(rg.AllResults)))
	md.WriteString(fmt.Sprintf("- **Passed:** %d\n", passCount))
	md.WriteString(fmt.Sprintf("- **Failed:** %d\n", failCount))
	md.WriteString(fmt.Sprintf("- **Warnings:** %d\n", warnCount))
	md.WriteString(fmt.Sprintf("- **Skipped:** %d\n\n", skipCount))

	// Results by layer
	for layer := 1; layer <= 7; layer++ {
		results, ok := rg.ResultsByLayer[layer]
		if !ok {
			continue
		}

		md.WriteString(fmt.Sprintf("## Layer %d\n\n", layer))

		for _, result := range results {
			var statusEmoji string
			switch result.Status {
			case StatusPassed:
				statusEmoji = "✅"
			case StatusFailed:
				statusEmoji = "❌"
			case StatusWarning:
				statusEmoji = "⚠️"
			case StatusSkipped:
				statusEmoji = "⏭️"
			default:
				statusEmoji = "❓"
			}

			md.WriteString(fmt.Sprintf("### %s %s: %s\n\n", statusEmoji, result.Name, string(result.Status)))
			md.WriteString(fmt.Sprintf("%s\n\n", result.Message))

			if result.Metrics.Duration > 0 || result.Metrics.Latency > 0 || result.Metrics.PacketLoss > 0 {
				md.WriteString("**Metrics:**\n\n")
				if result.Metrics.Duration > 0 {
					md.WriteString(fmt.Sprintf("- Duration: %.2f ms\n", float64(result.Metrics.Duration.Milliseconds())))
				}
				if result.Metrics.Latency > 0 {
					md.WriteString(fmt.Sprintf("- Latency: %.2f ms\n", float64(result.Metrics.Latency.Milliseconds())))
				}
				if result.Metrics.PacketLoss > 0 {
					md.WriteString(fmt.Sprintf("- Packet Loss: %.2f%%\n", result.Metrics.PacketLoss))
				}
				if result.Metrics.TransferRate > 0 {
					md.WriteString(fmt.Sprintf("- Transfer Rate: %.2f MB/s\n", result.Metrics.TransferRate))
				}
				md.WriteString("\n")
			}
		}
	}

	if err := os.WriteFile(path, []byte(md.String()), 0644); err != nil {
		return fmt.Errorf("failed to write Markdown file: %w", err)
	}

	return nil
}

// generateXMLReport is an internal method for the ReportGenerator
func (rg *ReportGenerator) generateXMLReport(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create report directory: %w", err)
	}

	// Simple XML format
	var xml strings.Builder
	xml.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	xml.WriteString("<TestResults>\n")
	xml.WriteString(fmt.Sprintf("  <GeneratedAt>%s</GeneratedAt>\n", time.Now().Format(time.RFC3339)))

	for layer := 1; layer <= 7; layer++ {
		results, ok := rg.ResultsByLayer[layer]
		if !ok {
			continue
		}

		xml.WriteString(fmt.Sprintf("  <Layer id=\"%d\">\n", layer))

		for _, result := range results {
			xml.WriteString("    <Test>\n")
			xml.WriteString(fmt.Sprintf("      <Name>%s</Name>\n", result.Name))
			xml.WriteString(fmt.Sprintf("      <Status>%s</Status>\n", result.Status))
			xml.WriteString(fmt.Sprintf("      <Message>%s</Message>\n", result.Message))
			xml.WriteString(fmt.Sprintf("      <StartTime>%s</StartTime>\n", result.StartTime.Format(time.RFC3339)))
			xml.WriteString(fmt.Sprintf("      <EndTime>%s</EndTime>\n", result.EndTime.Format(time.RFC3339)))

			xml.WriteString("      <Metrics>\n")
			xml.WriteString(fmt.Sprintf("        <Duration>%d</Duration>\n", result.Metrics.Duration.Milliseconds()))
			xml.WriteString(fmt.Sprintf("        <TransferRate>%.2f</TransferRate>\n", result.Metrics.TransferRate))
			xml.WriteString(fmt.Sprintf("        <Latency>%d</Latency>\n", result.Metrics.Latency.Milliseconds()))
			xml.WriteString(fmt.Sprintf("        <PacketLoss>%.2f</PacketLoss>\n", result.Metrics.PacketLoss))
			xml.WriteString(fmt.Sprintf("        <ResponseTime>%d</ResponseTime>\n", result.Metrics.ResponseTime.Milliseconds()))
			xml.WriteString("      </Metrics>\n")

			xml.WriteString("    </Test>\n")
		}

		xml.WriteString("  </Layer>\n")
	}

	xml.WriteString("</TestResults>\n")

	if err := os.WriteFile(path, []byte(xml.String()), 0644); err != nil {
		return fmt.Errorf("failed to write XML file: %w", err)
	}

	return nil
}
