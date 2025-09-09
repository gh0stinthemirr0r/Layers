package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"

	"ghostshell/app/layers"
	"ghostshell/app/layers/common"
	"ghostshell/app/layers/visualization"
)

func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}

func promptForLayerSelection() ([]int, error) {
	fmt.Println("\nOSI Layer Test Selection")
	fmt.Println("------------------------")
	fmt.Println("Available layers:")
	fmt.Println("1. Physical Layer")
	fmt.Println("2. Data Link Layer")
	fmt.Println("3. Network Layer")
	fmt.Println("4. Transport Layer")
	fmt.Println("5. Session Layer")
	fmt.Println("6. Presentation Layer")
	fmt.Println("7. Application Layer")
	fmt.Println("0. Test All Layers")
	fmt.Print("\nEnter layer numbers to test (comma-separated, e.g. 1,2,3 or 0 for all): ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "0" {
		return []int{1, 2, 3, 4, 5, 6, 7}, nil
	}

	var selectedLayers []int
	for _, s := range strings.Split(input, ",") {
		layer, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			return nil, fmt.Errorf("invalid layer number: %s", s)
		}
		if layer < 1 || layer > 7 {
			return nil, fmt.Errorf("layer number must be between 1 and 7: %d", layer)
		}
		selectedLayers = append(selectedLayers, layer)
	}

	return selectedLayers, nil
}

func main() {
	// Parse command line flags
	addr := flag.String("addr", ":8080", "Address to serve visualization dashboard")
	flag.Parse()

	// Initialize logger
	logger, cleanup, err := layers.InitializeLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	common.Logger = logger

	// Get layer selection from user
	selectedLayers, err := promptForLayerSelection()
	if err != nil {
		logger.Error("Failed to get layer selection", zap.Error(err))
		os.Exit(1)
	}

	// Create visualizer
	vis, err := visualization.NewVisualizer(logger)
	if err != nil {
		logger.Fatal("Failed to create visualizer", zap.Error(err))
	}

	// Create context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		logger.Info("Received shutdown signal", zap.String("signal", sig.String()))
		cancel()
	}()

	// Start visualizer in a goroutine
	go func() {
		if err := vis.Start(*addr); err != nil {
			if !strings.Contains(err.Error(), "http: Server closed") {
				logger.Error("Visualizer server error", zap.Error(err))
				cancel()
			}
		}
	}()

	// Give the server a moment to start
	time.Sleep(time.Second)

	// Open browser
	url := fmt.Sprintf("http://localhost%s", *addr)
	if err := openBrowser(url); err != nil {
		logger.Error("Failed to open browser", zap.Error(err))
		fmt.Printf("Please open your browser and navigate to: %s\n", url)
	}

	fmt.Printf("\nStarting OSI layer tests for layers: %v\n", selectedLayers)
	fmt.Printf("View results at: %s\n\n", url)

	// Run layer tests
	results, err := layers.RunLayerTests(selectedLayers)
	if err != nil {
		logger.Error("Failed to run layer tests", zap.Error(err))
		os.Exit(1)
	}

	// Update visualizer with results
	vis.UpdateResults(results)

	fmt.Println("\nTests completed. Press Ctrl+C to exit.")

	// Keep running until context is cancelled
	<-ctx.Done()

	// Cleanup
	if err := vis.Stop(); err != nil {
		logger.Error("Failed to stop visualizer", zap.Error(err))
	}
}
