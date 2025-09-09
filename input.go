package layers

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// InputArgs holds the parsed command-line arguments.
type InputArgs struct {
	Layers       []int  // Layers to test (1-7 or empty for all)
	OutputFormat string // Desired output format: csv, pdf, or json
	OutputPath   string // Path to save the output report
	ConfigPath   string // Path to the configuration file
	Verbose      bool   // Enable verbose output
	Timeout      int    // Timeout in seconds for each test
}

// ParseInput parses and validates command-line arguments.
func ParseInput() (*InputArgs, error) {
	// Define command-line flags
	layers := flag.String("layers", "", "Comma-separated list of OSI layers to test (1-7). Empty means test all layers")
	outputFormat := flag.String("format", "csv", "Output format for the report (csv, pdf, or json)")
	outputPath := flag.String("output", "", "Path to save the output report (default: osi_report_<timestamp>.<format>)")
	configPath := flag.String("config", "config.json", "Path to the configuration file")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	timeout := flag.Int("timeout", 30, "Timeout in seconds for each test")

	// Parse flags
	flag.Parse()

	// Parse layers
	var selectedLayers []int
	if *layers != "" {
		for _, l := range strings.Split(*layers, ",") {
			layer := 0
			_, err := fmt.Sscanf(l, "%d", &layer)
			if err != nil || layer < 1 || layer > 7 {
				return nil, fmt.Errorf("invalid layer number: %s. Must be between 1 and 7", l)
			}
			selectedLayers = append(selectedLayers, layer)
		}
	}

	// Validate output format
	if *outputFormat != "csv" && *outputFormat != "pdf" && *outputFormat != "json" {
		return nil, fmt.Errorf("invalid output format: %s. Allowed values are: csv, pdf, json", *outputFormat)
	}

	// Generate default output path if not provided
	if *outputPath == "" {
		timestamp := time.Now().Format("20060102_150405")
		*outputPath = fmt.Sprintf("osi_report_%s.%s", timestamp, *outputFormat)
	}

	// Create and return the InputArgs struct
	return &InputArgs{
		Layers:       selectedLayers,
		OutputFormat: *outputFormat,
		OutputPath:   *outputPath,
		ConfigPath:   *configPath,
		Verbose:      *verbose,
		Timeout:      *timeout,
	}, nil
}

// PrintUsage displays the application usage instructions.
func PrintUsage() {
	fmt.Println("OSI Layer Network Tester")
	fmt.Println("\nTests network connectivity across OSI layers and generates a detailed report.")
	fmt.Println("\nUsage: osi-tester [options]")
	fmt.Println("\nOptions:")
	flag.PrintDefaults()
	fmt.Println("\nExamples:")
	fmt.Println("  Test all layers:")
	fmt.Println("    osi-tester")
	fmt.Println("  Test specific layers:")
	fmt.Println("    osi-tester -layers 3,4 -format json")
	fmt.Println("  Test with custom timeout:")
	fmt.Println("    osi-tester -layers 1,2,3 -timeout 60 -verbose")
}

// ValidateArgs ensures that the provided arguments meet the application's requirements.
func ValidateArgs(args *InputArgs) error {
	// Check if config file exists
	if _, err := os.Stat(args.ConfigPath); os.IsNotExist(err) {
		return fmt.Errorf("configuration file does not exist at path: %s", args.ConfigPath)
	}

	// Validate timeout
	if args.Timeout < 1 {
		return fmt.Errorf("timeout must be at least 1 second")
	}

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(args.OutputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %s", err)
	}

	return nil
}
