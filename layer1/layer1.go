// Package layer1 implements physical layer (OSI Layer 1) testing functionality
package layer1

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"ghostshell/app/layers/common"
)

// Runner implements physical layer tests
type Runner struct {
	AttemptCount      int
	MinSignalStrength int
	Interfaces        []string
}

// New creates a new Layer1Runner with the specified parameters
func New(attemptCount int, minSignalStrength int) *Runner {
	if attemptCount <= 0 {
		attemptCount = 3
	}
	if minSignalStrength <= 0 {
		minSignalStrength = 50
	}

	// Default interfaces based on platform
	defaultInterfaces := getDefaultInterfaces()

	return &Runner{
		AttemptCount:      attemptCount,
		MinSignalStrength: minSignalStrength,
		Interfaces:        defaultInterfaces,
	}
}

// getDefaultInterfaces returns default network interfaces based on the OS
func getDefaultInterfaces() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"Ethernet", "Wi-Fi"}
	case "darwin":
		return []string{"en0", "en1"}
	case "linux":
		// On Linux, we'll try to find the actual interfaces
		ifaces, err := net.Interfaces()
		if err != nil {
			return []string{"eth0", "wlan0"}
		}

		// Filter out loopback and non-up interfaces
		var validIfaces []string
		for _, iface := range ifaces {
			if (iface.Flags&net.FlagLoopback) == 0 && (iface.Flags&net.FlagUp) != 0 {
				validIfaces = append(validIfaces, iface.Name)
			}
		}

		if len(validIfaces) > 0 {
			return validIfaces
		}
		return []string{"eth0", "wlan0"}
	default:
		return []string{"eth0", "wlan0"}
	}
}

// GetName returns the name of this layer
func (r *Runner) GetName() string {
	return "Physical Layer"
}

// GetDescription returns a description of this layer's functionality
func (r *Runner) GetDescription() string {
	return "Tests physical connectivity and signal strength of network interfaces"
}

// GetDependencies returns the layer numbers this layer depends on
func (r *Runner) GetDependencies() []int {
	// Layer 1 has no dependencies
	return []int{}
}

// ValidateConfig validates the configuration for this layer
func (r *Runner) ValidateConfig() error {
	if r.AttemptCount <= 0 {
		return fmt.Errorf("attempt count must be greater than 0")
	}
	if r.MinSignalStrength <= 0 || r.MinSignalStrength > 100 {
		return fmt.Errorf("min signal strength must be between 1 and 100")
	}
	return nil
}

// RunTests implements the LayerRunner interface
func (r *Runner) RunTests(ctx context.Context, logger *zap.Logger) ([]common.TestResult, error) {
	logger.Info("Starting Layer 1 (Physical Layer) tests...",
		zap.Int("attempt_count", r.AttemptCount),
		zap.Int("min_signal_strength", r.MinSignalStrength),
		zap.Strings("interfaces", r.Interfaces),
	)

	startTime := time.Now()

	// Create a parent result
	parentResult := common.TestResult{
		Layer:      1,
		Name:       "Physical Layer Tests",
		Status:     common.StatusPassed,
		StartTime:  startTime,
		SubResults: []common.TestResult{},
	}

	// Get all network interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		logger.Error("Failed to get network interfaces", zap.Error(err))
		parentResult.Status = common.StatusFailed
		parentResult.Message = fmt.Sprintf("Failed to get network interfaces: %v", err)
		parentResult.EndTime = time.Now()
		return []common.TestResult{parentResult}, err
	}

	// Filter out loopback interfaces but include all others
	var matchedInterfaces []net.Interface
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback == 0 {
			matchedInterfaces = append(matchedInterfaces, iface)
		}
	}

	if len(matchedInterfaces) == 0 {
		msg := "No valid network interfaces found"
		logger.Warn(msg)
		parentResult.Status = common.StatusWarning
		parentResult.Message = msg
		parentResult.EndTime = time.Now()
		return []common.TestResult{parentResult}, nil
	}

	// Test each interface
	var wg sync.WaitGroup
	resultsChan := make(chan common.TestResult, len(matchedInterfaces)*2)

	for _, iface := range matchedInterfaces {
		iface := iface // Capture variable for goroutine

		// Test physical connection
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Create a test result for this interface's connection
			connResult := common.TestResult{
				Layer:     1,
				Name:      fmt.Sprintf("Interface %s Connection", iface.Name),
				StartTime: time.Now(),
				Metrics:   common.TestMetrics{},
			}

			// Check if context is done
			select {
			case <-ctx.Done():
				connResult.Status = common.StatusSkipped
				connResult.Message = "Test was cancelled"
				connResult.EndTime = time.Now()
				connResult.Metrics.Duration = connResult.EndTime.Sub(connResult.StartTime)
				resultsChan <- connResult
				return
			default:
				// Continue with test
			}

			// Check if this is a VPN interface
			isVPN := isVPNInterface(iface.Name)

			// Test connection with multiple attempts
			connectionResults := make(chan bool, r.AttemptCount)
			var connWg sync.WaitGroup

			for i := 0; i < r.AttemptCount; i++ {
				connWg.Add(1)
				go func(iter int) {
					defer connWg.Done()
					connectionResults <- checkPhysicalConnection(iface.Name)
				}(i)
			}

			connWg.Wait()
			close(connectionResults)

			// Count successes
			successCount := 0
			failCount := 0
			for result := range connectionResults {
				if result {
					successCount++
				} else {
					failCount++
				}
			}

			// Calculate connection reliability
			connReliability := float64(successCount) / float64(r.AttemptCount) * 100

			// Set result based on connection status and VPN status
			if failCount > r.AttemptCount/2 {
				if isVPN {
					// For VPN interfaces, being down might be normal
					connResult.Status = common.StatusWarning
					connResult.Message = fmt.Sprintf("VPN interface %s is down (%d/%d attempts failed)",
						iface.Name, failCount, r.AttemptCount)
				} else {
					connResult.Status = common.StatusFailed
					connResult.Message = fmt.Sprintf("Physical connection check failed: %d/%d attempts failed. Interface %s might be down or disconnected.",
						failCount, r.AttemptCount, iface.Name)
				}
			} else {
				connResult.Status = common.StatusPassed
				if isVPN {
					connResult.Message = fmt.Sprintf("VPN interface %s is up and running (%d/%d attempts successful)",
						iface.Name, successCount, r.AttemptCount)
				} else {
					connResult.Message = fmt.Sprintf("Physical connection check passed: %d/%d attempts successful on interface %s",
						successCount, r.AttemptCount, iface.Name)
				}
			}

			// Get MTU and carrier info
			mtu := iface.MTU
			operstate, carrier := getInterfaceDetails(iface.Name)
			txBytes, rxBytes := getInterfaceStats(iface.Name)

			// Set metrics
			connResult.EndTime = time.Now()
			connResult.Metrics.Duration = connResult.EndTime.Sub(connResult.StartTime)
			connResult.Metrics.ReliabilityPct = connReliability

			// Add connection diagnostic data
			connResult.Diagnostics = map[string]interface{}{
				"interface":     iface.Name,
				"hardware_addr": iface.HardwareAddr.String(),
				"mtu":           mtu,
				"flags":         iface.Flags.String(),
				"success_count": successCount,
				"fail_count":    failCount,
				"oper_state":    operstate,
				"carrier":       carrier,
				"tx_bytes":      txBytes,
				"rx_bytes":      rxBytes,
				"is_vpn":        isVPN,
			}

			resultsChan <- connResult
		}()

		// Test signal strength (for wireless interfaces)
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Create a test result for this interface's signal strength
			signalResult := common.TestResult{
				Layer:     1,
				Name:      fmt.Sprintf("Interface %s Signal Strength", iface.Name),
				StartTime: time.Now(),
				Metrics:   common.TestMetrics{},
			}

			// Check if context is done
			select {
			case <-ctx.Done():
				signalResult.Status = common.StatusSkipped
				signalResult.Message = "Test was cancelled"
				signalResult.EndTime = time.Now()
				signalResult.Metrics.Duration = signalResult.EndTime.Sub(signalResult.StartTime)
				resultsChan <- signalResult
				return
			default:
				// Continue with test
			}

			// Only check signal strength for wireless interfaces
			isWireless, err := isWirelessInterface(iface.Name)
			if err != nil || !isWireless {
				signalResult.Status = common.StatusSkipped
				signalResult.Message = "Not a wireless interface, skipping signal strength test"
				signalResult.EndTime = time.Now()
				signalResult.Metrics.Duration = signalResult.EndTime.Sub(signalResult.StartTime)
				resultsChan <- signalResult
				return
			}

			// Get wireless signal info
			strength, linkQuality, noise, bitRate, frequency := getWirelessInfo(iface.Name)

			// Set result based on signal strength threshold
			if strength < r.MinSignalStrength {
				signalResult.Status = common.StatusWarning
				signalResult.Message = fmt.Sprintf("Low signal strength: %d%% (minimum: %d%%)",
					strength, r.MinSignalStrength)
			} else {
				signalResult.Status = common.StatusPassed
				signalResult.Message = fmt.Sprintf("Signal strength is good: %d%%", strength)
			}

			// Set metrics
			signalResult.EndTime = time.Now()
			signalResult.Metrics.Duration = signalResult.EndTime.Sub(signalResult.StartTime)
			signalResult.Metrics.Custom = map[string]interface{}{
				"signal_strength": strength,
				"link_quality":    linkQuality,
				"noise_level":     noise,
				"bit_rate":        bitRate,
				"frequency":       frequency,
			}

			// Add signal strength diagnostic data
			signalResult.Diagnostics = map[string]interface{}{
				"interface":       iface.Name,
				"signal_strength": strength,
				"min_threshold":   r.MinSignalStrength,
				"link_quality":    linkQuality,
				"noise_level":     noise,
				"bit_rate":        bitRate,
				"frequency":       frequency,
			}

			resultsChan <- signalResult
		}()
	}

	// Wait for all tests to complete
	wg.Wait()
	close(resultsChan)

	// Process results
	var subResults []common.TestResult
	failureCount := 0
	warningCount := 0
	successCount := 0

	for result := range resultsChan {
		subResults = append(subResults, result)

		switch result.Status {
		case common.StatusFailed:
			failureCount++
		case common.StatusWarning:
			warningCount++
		case common.StatusPassed:
			successCount++
		}
	}

	// Update parent result
	parentResult.SubResults = subResults
	parentResult.EndTime = time.Now()
	parentResult.Metrics.Duration = parentResult.EndTime.Sub(parentResult.StartTime)

	// Collect failure and warning details
	var failureDetails []string
	var warningDetails []string
	for _, result := range subResults {
		switch result.Status {
		case common.StatusFailed:
			failureDetails = append(failureDetails, fmt.Sprintf("- %s: %s", result.Name, result.Message))
		case common.StatusWarning:
			warningDetails = append(warningDetails, fmt.Sprintf("- %s: %s", result.Name, result.Message))
		}
	}

	// Determine overall status and build detailed message
	var messageBuilder strings.Builder
	if failureCount > 0 && successCount > 0 {
		parentResult.Status = common.StatusMixed
		messageBuilder.WriteString(fmt.Sprintf("Layer 1 tests completed with mixed results:\n"+
			"- %d interfaces passed\n"+
			"- %d interfaces failed\n"+
			"- %d interfaces have warnings\n\n",
			successCount, failureCount, warningCount))
	} else if failureCount > 0 {
		parentResult.Status = common.StatusFailed
		messageBuilder.WriteString(fmt.Sprintf("Layer 1 tests failed with %d failures:\n\n",
			failureCount))
	} else if warningCount > 0 {
		parentResult.Status = common.StatusWarning
		messageBuilder.WriteString(fmt.Sprintf("Layer 1 tests completed with %d warnings:\n\n",
			warningCount))
	} else {
		parentResult.Status = common.StatusPassed
		messageBuilder.WriteString(fmt.Sprintf("All Layer 1 tests passed successfully (%d interfaces tested)\n",
			len(subResults)))
	}

	if len(failureDetails) > 0 {
		messageBuilder.WriteString("\nFailures:\n")
		messageBuilder.WriteString(strings.Join(failureDetails, "\n"))
	}
	if len(warningDetails) > 0 {
		messageBuilder.WriteString("\n\nWarnings:\n")
		messageBuilder.WriteString(strings.Join(warningDetails, "\n"))
	}

	parentResult.Message = messageBuilder.String()
	logger.Info("Layer 1 tests completed",
		zap.String("status", string(parentResult.Status)),
		zap.Int("total_interfaces", len(subResults)),
		zap.Int("passed", successCount),
		zap.Int("failed", failureCount),
		zap.Int("warnings", warningCount),
	)

	if failureCount > 0 {
		return []common.TestResult{parentResult}, fmt.Errorf("layer 1 tests failed with %d failures", failureCount)
	}
	return []common.TestResult{parentResult}, nil
}

// Helper functions for physical layer tests

// checkPhysicalConnection tests the physical connectivity of an interface
// Returns true if the interface is up and has carrier
func checkPhysicalConnection(interfaceName string) bool {
	switch runtime.GOOS {
	case "linux":
		// On Linux, check /sys/class/net/[iface]/carrier
		carrierPath := fmt.Sprintf("/sys/class/net/%s/carrier", interfaceName)
		data, err := os.ReadFile(carrierPath)
		if err == nil {
			// Carrier file exists, check if it's 1 (connected)
			return strings.TrimSpace(string(data)) == "1"
		}

		// Alternative: check if operstate is "up"
		operstPath := fmt.Sprintf("/sys/class/net/%s/operstate", interfaceName)
		data, err = os.ReadFile(operstPath)
		if err == nil {
			state := strings.TrimSpace(string(data))
			return state == "up" || state == "unknown"
		}

		// If can't check carrier or operstate, just check if interface exists and is up
		iface, err := net.InterfaceByName(interfaceName)
		if err != nil {
			return false
		}
		return (iface.Flags & net.FlagUp) != 0

	case "windows":
		// On Windows, use PowerShell to check interface status
		cmd := exec.Command("powershell", "-Command",
			fmt.Sprintf("Get-NetAdapter | Where-Object {$_.Name -eq '%s' -or $_.InterfaceDescription -like '*%s*'} | Select-Object -ExpandProperty Status",
				interfaceName, interfaceName))

		output, err := cmd.Output()
		if err != nil {
			return false
		}

		status := strings.TrimSpace(string(output))
		return status == "Up"

	case "darwin":
		// On macOS, use ifconfig to check interface status
		cmd := exec.Command("ifconfig", interfaceName)
		output, err := cmd.Output()
		if err != nil {
			return false
		}

		// Check if interface is up and running
		outputStr := string(output)
		return strings.Contains(outputStr, "status: active") ||
			(strings.Contains(outputStr, "UP") &&
				strings.Contains(outputStr, "RUNNING"))

	default:
		// Generic method for other platforms
		iface, err := net.InterfaceByName(interfaceName)
		if err != nil {
			return false
		}
		return (iface.Flags & net.FlagUp) != 0
	}
}

// isWirelessInterface determines if an interface is wireless
func isWirelessInterface(interfaceName string) (bool, error) {
	switch runtime.GOOS {
	case "linux":
		// On Linux, check if /sys/class/net/[iface]/wireless exists
		wirelessDir := fmt.Sprintf("/sys/class/net/%s/wireless", interfaceName)
		_, err := os.Stat(wirelessDir)
		if err == nil {
			// Directory exists, it's a wireless interface
			return true, nil
		}

		// Alternative: check for /proc/net/wireless
		file, err := os.Open("/proc/net/wireless")
		if err == nil {
			defer file.Close()

			scanner := bufio.NewScanner(file)
			// Skip header lines (first two lines)
			scanner.Scan()
			scanner.Scan()

			// Check if interface is listed
			for scanner.Scan() {
				line := scanner.Text()
				if strings.Contains(line, interfaceName+":") {
					return true, nil
				}
			}
		}

		// Last resort: check driver type with ethtool
		cmd := exec.Command("ethtool", "-i", interfaceName)
		output, err := cmd.CombinedOutput()
		if err == nil {
			// Check for common wireless drivers
			outputStr := string(output)
			return strings.Contains(outputStr, "driver: ath") ||
					strings.Contains(outputStr, "driver: iwl") ||
					strings.Contains(outputStr, "driver: rtw") ||
					strings.Contains(outputStr, "driver: wl"),
				nil
		}

		// If all methods fail, assume it's not wireless
		return false, nil

	case "windows":
		// On Windows, use netsh to check adapter types
		cmd := exec.Command("powershell", "-Command",
			fmt.Sprintf("Get-NetAdapter | Where-Object {$_.Name -eq '%s' -or $_.InterfaceDescription -like '*%s*'} | Select-Object -ExpandProperty MediaType",
				interfaceName, interfaceName))

		output, err := cmd.Output()
		if err != nil {
			return false, err
		}

		mediaType := strings.TrimSpace(string(output))
		return mediaType == "Native 802.11" || mediaType == "Wireless LAN", nil

	case "darwin":
		// On macOS, use system_profiler to check interface types
		cmd := exec.Command("networksetup", "-listallhardwareports")
		output, err := cmd.Output()
		if err != nil {
			return false, err
		}

		outputStr := string(output)
		sections := strings.Split(outputStr, "Hardware Port: ")

		for _, section := range sections {
			if strings.Contains(section, "Device: "+interfaceName) {
				return strings.Contains(section, "Wi-Fi") || strings.Contains(section, "AirPort"), nil
			}
		}

		return false, nil

	default:
		// Generic method: check if interface name suggests wireless
		return strings.HasPrefix(interfaceName, "wl") ||
				strings.HasPrefix(interfaceName, "ath") ||
				strings.HasPrefix(interfaceName, "ra") ||
				strings.Contains(strings.ToLower(interfaceName), "wifi") ||
				strings.Contains(strings.ToLower(interfaceName), "wireless"),
			nil
	}
}

// getInterfaceDetails gets operational state and carrier status
func getInterfaceDetails(interfaceName string) (string, int) {
	operstate := "unknown"
	carrier := -1

	if runtime.GOOS == "linux" {
		// Check operstate
		operstPath := fmt.Sprintf("/sys/class/net/%s/operstate", interfaceName)
		data, err := os.ReadFile(operstPath)
		if err == nil {
			operstate = strings.TrimSpace(string(data))
		}

		// Check carrier
		carrierPath := fmt.Sprintf("/sys/class/net/%s/carrier", interfaceName)
		data, err = os.ReadFile(carrierPath)
		if err == nil {
			carrierVal, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err == nil {
				carrier = carrierVal
			}
		}
	}

	return operstate, carrier
}

// getInterfaceStats gets RX/TX byte counts
func getInterfaceStats(interfaceName string) (int64, int64) {
	var txBytes, rxBytes int64 = -1, -1

	if runtime.GOOS == "linux" {
		// Get transmitted bytes
		txPath := fmt.Sprintf("/sys/class/net/%s/statistics/tx_bytes", interfaceName)
		data, err := os.ReadFile(txPath)
		if err == nil {
			txBytes, _ = strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		}

		// Get received bytes
		rxPath := fmt.Sprintf("/sys/class/net/%s/statistics/rx_bytes", interfaceName)
		data, err = os.ReadFile(rxPath)
		if err == nil {
			rxBytes, _ = strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		}
	}

	return txBytes, rxBytes
}

// getWirelessInfo returns signal strength and related wireless information
func getWirelessInfo(interfaceName string) (int, int, int, string, string) {
	switch runtime.GOOS {
	case "linux":
		return getLinuxWirelessInfo(interfaceName)
	case "windows":
		return getWindowsWirelessInfo(interfaceName)
	case "darwin":
		return getMacWirelessInfo(interfaceName)
	default:
		return 50, 0, 0, "unknown", "unknown" // Default values
	}
}

// getLinuxWirelessInfo returns wireless info on Linux
func getLinuxWirelessInfo(interfaceName string) (int, int, int, string, string) {
	strength := 0
	linkQuality := 0
	noise := 0
	bitRate := "unknown"
	frequency := "unknown"

	// Try to get info from /proc/net/wireless
	file, err := os.Open("/proc/net/wireless")
	if err == nil {
		defer file.Close()

		scanner := bufio.NewScanner(file)
		// Skip header lines (first two lines)
		scanner.Scan()
		scanner.Scan()

		// Parse interface lines
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, interfaceName+":") {
				// Format: Interface : status link level noise nwid crypt   misc
				fields := strings.Fields(line)
				if len(fields) >= 5 {
					linkQualityRaw, _ := strconv.Atoi(fields[2])
					linkQuality = linkQualityRaw

					signalLevelRaw, _ := strconv.Atoi(fields[3])
					strength = normalizeSignalStrength(signalLevelRaw, "dbm")

					noiseRaw, _ := strconv.Atoi(fields[4])
					noise = noiseRaw
				}
				break
			}
		}
	}

	// If wireless info not found, try iwconfig
	if strength == 0 {
		cmd := exec.Command("iwconfig", interfaceName)
		output, err := cmd.CombinedOutput()
		if err == nil {
			outputStr := string(output)

			// Extract signal level
			signalRe := regexp.MustCompile(`Signal level[=:]?\s*([-\d]+)(\s*dBm)?`)
			matches := signalRe.FindStringSubmatch(outputStr)
			if len(matches) >= 2 {
				signalLevel, _ := strconv.Atoi(matches[1])
				strength = normalizeSignalStrength(signalLevel, "dbm")
			}

			// Extract link quality
			qualityRe := regexp.MustCompile(`Link Quality[=:]?\s*(\d+)/(\d+)`)
			matches = qualityRe.FindStringSubmatch(outputStr)
			if len(matches) >= 3 {
				quality, _ := strconv.Atoi(matches[1])
				qualityMax, _ := strconv.Atoi(matches[2])
				if qualityMax > 0 {
					linkQuality = quality * 100 / qualityMax
				}
			}

			// Extract bit rate
			rateRe := regexp.MustCompile(`Bit Rate[=:]?\s*([\d.]+)\s*([GM]b/s)`)
			matches = rateRe.FindStringSubmatch(outputStr)
			if len(matches) >= 3 {
				bitRate = matches[1] + " " + matches[2]
			}

			// Extract frequency
			freqRe := regexp.MustCompile(`Frequency[=:]?\s*([\d.]+)\s*([GM]Hz)`)
			matches = freqRe.FindStringSubmatch(outputStr)
			if len(matches) >= 3 {
				frequency = matches[1] + " " + matches[2]
			}
		}
	}

	// If strength still not determined, try iw dev
	if strength == 0 {
		cmd := exec.Command("iw", "dev", interfaceName, "link")
		output, err := cmd.CombinedOutput()
		if err == nil {
			outputStr := string(output)

			// Extract signal level
			signalRe := regexp.MustCompile(`signal:\s*([-\d]+)\s*dBm`)
			matches := signalRe.FindStringSubmatch(outputStr)
			if len(matches) >= 2 {
				signalLevel, _ := strconv.Atoi(matches[1])
				strength = normalizeSignalStrength(signalLevel, "dbm")
			}

			// Extract bit rate
			rateRe := regexp.MustCompile(`tx bitrate:\s*([\d.]+)\s*([GMk]bit/s)`)
			matches = rateRe.FindStringSubmatch(outputStr)
			if len(matches) >= 3 {
				bitRate = matches[1] + " " + matches[2]
			}
		}
	}

	// Get frequency with iw dev if not already
	if frequency == "unknown" {
		cmd := exec.Command("iw", "dev", interfaceName, "info")
		output, err := cmd.CombinedOutput()
		if err == nil {
			outputStr := string(output)

			// Extract frequency
			freqRe := regexp.MustCompile(`channel .+ \(([\d.]+)\s*([GM]Hz)\)`)
			matches := freqRe.FindStringSubmatch(outputStr)
			if len(matches) >= 3 {
				frequency = matches[1] + " " + matches[2]
			}
		}
	}

	// If strength still unknown, set default
	if strength == 0 {
		strength = 50
	}

	return strength, linkQuality, noise, bitRate, frequency
}

// getWindowsWirelessInfo returns wireless info on Windows
func getWindowsWirelessInfo(interfaceName string) (int, int, int, string, string) {
	signalStrength := 0
	linkQuality := 0
	noise := 0
	bitRate := "unknown"
	frequency := "unknown"

	// Get wireless signal using PowerShell
	cmd := exec.Command("powershell", "-Command",
		fmt.Sprintf("(Get-NetAdapter | Where-Object {$_.Name -eq '%s' -or $_.InterfaceDescription -like '*%s*'} | Get-NetAdapterAdvancedProperty -DisplayName '*Signal*' | Select-Object DisplayValue).DisplayValue",
			interfaceName, interfaceName))

	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		// Try to parse the signal strength
		signalStr := strings.TrimSpace(string(output))

		// Remove % if present
		signalStr = strings.TrimSuffix(signalStr, "%")

		// Parse as integer
		signalVal, err := strconv.Atoi(signalStr)
		if err == nil {
			signalStrength = signalVal
		}
	}

	// If strength still unknown, try another method
	if signalStrength == 0 {
		cmd := exec.Command("powershell", "-Command",
			`(netsh wlan show interfaces) -match '^\s+Signal' -replace '.*:\s*',''`)

		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			// Signal is in format "XX%"
			signalStr := strings.TrimSpace(string(output))
			signalStr = strings.TrimSuffix(signalStr, "%")
			signalVal, err := strconv.Atoi(signalStr)
			if err == nil {
				signalStrength = signalVal
			}
		}
	}

	// Get channel/frequency
	cmd = exec.Command("powershell", "-Command",
		`(netsh wlan show interfaces) -match '^\s+Channel'`)

	output, err = cmd.Output()
	if err == nil && len(output) > 0 {
		channelStr := strings.TrimSpace(string(output))
		channelRe := regexp.MustCompile(`Channel\s*:\s*(\d+)`)
		matches := channelRe.FindStringSubmatch(channelStr)
		if len(matches) >= 2 {
			channel, _ := strconv.Atoi(matches[1])
			// Convert channel to frequency (approximate)
			if channel >= 1 && channel <= 14 {
				freqGHz := 2.407 + 0.005*float64(channel)
				frequency = fmt.Sprintf("%.3f GHz", freqGHz)
			} else if channel >= 36 && channel <= 165 {
				freqGHz := 5 + 0.005*float64(channel-36)
				frequency = fmt.Sprintf("%.3f GHz", freqGHz)
			}
		}
	}

	// Get bit rate
	cmd = exec.Command("powershell", "-Command",
		`(netsh wlan show interfaces) -match '^\s+Receive|^\s+Transmit'`)

	output, err = cmd.Output()
	if err == nil && len(output) > 0 {
		rateStr := strings.TrimSpace(string(output))
		rateRe := regexp.MustCompile(`Rate\s*:\s*([\d.]+)\s*([MGk]bps)`)
		matches := rateRe.FindStringSubmatch(rateStr)
		if len(matches) >= 3 {
			bitRate = matches[1] + " " + matches[2]
		}
	}

	// If strength still unknown, set default
	if signalStrength == 0 {
		signalStrength = 50
	}

	return signalStrength, linkQuality, noise, bitRate, frequency
}

// getMacWirelessInfo returns wireless info on macOS
func getMacWirelessInfo(interfaceName string) (int, int, int, string, string) {
	signalStrength := 0
	linkQuality := 0
	noise := 0
	bitRate := "unknown"
	frequency := "unknown"

	// Get wireless info using airport command
	airportPath := "/System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/airport"
	cmd := exec.Command(airportPath, "-I")
	output, err := cmd.Output()

	if err == nil {
		outputStr := string(output)

		// Extract RSSI (signal strength)
		rssiRe := regexp.MustCompile(`agrCtlRSSI:\s*([-\d]+)`)
		matches := rssiRe.FindStringSubmatch(outputStr)
		if len(matches) >= 2 {
			rssi, _ := strconv.Atoi(matches[1])
			signalStrength = normalizeSignalStrength(rssi, "rssi")
		}

		// Extract noise
		noiseRe := regexp.MustCompile(`agrCtlNoise:\s*([-\d]+)`)
		matches = noiseRe.FindStringSubmatch(outputStr)
		if len(matches) >= 2 {
			noise, _ = strconv.Atoi(matches[1])
		}

		// Compute link quality from RSSI and noise
		if signalStrength > 0 && noise != 0 {
			// Link quality is a function of signal-to-noise ratio
			snr := signalStrength - noise
			if snr > 40 {
				linkQuality = 100
			} else if snr < 15 {
				linkQuality = 0
			} else {
				linkQuality = (snr - 15) * 100 / 25
			}
		}

		// Extract channel/frequency
		channelRe := regexp.MustCompile(`channel:\s*(\d+)(,\d+)?`)
		matches = channelRe.FindStringSubmatch(outputStr)
		if len(matches) >= 2 {
			channel, _ := strconv.Atoi(matches[1])
			// Convert channel to frequency (approximate)
			if channel >= 1 && channel <= 14 {
				freqGHz := 2.407 + 0.005*float64(channel)
				frequency = fmt.Sprintf("%.3f GHz", freqGHz)
			} else if channel >= 36 && channel <= 165 {
				freqGHz := 5 + 0.005*float64(channel-36)
				frequency = fmt.Sprintf("%.3f GHz", freqGHz)
			}
		}

		// Extract rate
		rateRe := regexp.MustCompile(`lastTxRate:\s*([\d]+)`)
		matches = rateRe.FindStringSubmatch(outputStr)
		if len(matches) >= 2 {
			rate, _ := strconv.Atoi(matches[1])
			bitRate = fmt.Sprintf("%d Mbps", rate)
		}
	}

	// If strength still unknown, try another method with system_profiler
	if signalStrength == 0 {
		cmd := exec.Command("system_profiler", "SPAirPortDataType")
		output, err := cmd.Output()

		if err == nil {
			outputStr := string(output)

			// Extract signal from system profiler output (might not contain exact value)
			if strings.Contains(outputStr, "Signal / Noise:") {
				// Estimate from textual description (this is not precise)
				if strings.Contains(outputStr, "Excellent") {
					signalStrength = 90
				} else if strings.Contains(outputStr, "Good") {
					signalStrength = 70
				} else if strings.Contains(outputStr, "Fair") {
					signalStrength = 50
				} else if strings.Contains(outputStr, "Poor") {
					signalStrength = 30
				} else {
					signalStrength = 50
				}
			}
		}
	}

	// If strength still unknown, set default
	if signalStrength == 0 {
		signalStrength = 50
	}

	return signalStrength, linkQuality, noise, bitRate, frequency
}

// normalizeSignalStrength normalizes different signal measures to a percentage (0-100)
func normalizeSignalStrength(value int, unit string) int {
	switch unit {
	case "dbm":
		// dBm is typically between -100 (worst) and -30 (best)
		if value >= -30 {
			return 100
		} else if value <= -100 {
			return 0
		}
		return (value + 100) * 100 / 70

	case "rssi":
		// RSSI might be between -90 (worst) and -30 (best)
		if value >= -30 {
			return 100
		} else if value <= -90 {
			return 0
		}
		return (value + 90) * 100 / 60

	default:
		// Assume percentage or 0-70 quality value
		if value >= 0 && value <= 100 {
			return value
		} else if value > 0 {
			return int(float64(value) * 100 / 70) // Assume 70 is max quality
		}
		return 50 // Default
	}
}

// isVPNInterface determines if an interface is a VPN interface
func isVPNInterface(interfaceName string) bool {
	// Common VPN interface names and patterns
	vpnPatterns := []string{
		// Basic VPN types
		"tun", "tap", "ppp", "vpn", "ipsec", "wg",

		// Enterprise VPN Solutions
		"cisco", "anyconnect", "ac_", "vpn_", "pangp", // Cisco AnyConnect
		"gpd", "globalprotect", "paloalto", "pan", // Palo Alto GlobalProtect
		"pulse", "juniper", "network_connect", // Pulse Secure / Juniper
		"f5", "bigip", "edge", // F5 VPN
		"checkpoint", "snx", "capsule", // Check Point VPN
		"forticlient", "fortinet", "fortissl", // Fortinet FortiClient
		"sonicwall", "netextender", "swgp", // SonicWall
		"citrix", "netscaler", // Citrix

		// Consumer/SMB VPN Solutions
		"nordlynx", "proton", "mullvad", "express",
		"openvpn", "wireguard", "pritunl",
	}

	nameLower := strings.ToLower(interfaceName)
	for _, pattern := range vpnPatterns {
		if strings.Contains(nameLower, pattern) {
			return true
		}
	}

	// Check for VPN-specific flags or properties
	iface, err := net.InterfaceByName(interfaceName)
	if err == nil {
		// Point-to-Point interface is often used for VPNs
		if iface.Flags&net.FlagPointToPoint != 0 {
			return true
		}
	}

	// Additional OS-specific checks
	switch runtime.GOOS {
	case "windows":
		// Check network adapter type using PowerShell
		cmd := exec.Command("powershell", "-Command",
			fmt.Sprintf("Get-NetAdapter | Where-Object {$_.Name -eq '%s' -or $_.InterfaceDescription -like '*%s*'} | Select-Object -ExpandProperty InterfaceDescription",
				interfaceName, interfaceName))
		output, err := cmd.Output()
		if err == nil {
			desc := strings.ToLower(string(output))
			for _, pattern := range vpnPatterns {
				if strings.Contains(desc, pattern) {
					return true
				}
			}
		}
	case "linux":
		// Check if interface is associated with VPN services
		for _, path := range []string{
			"/sys/class/net/" + interfaceName + "/tun_flags",
			"/sys/class/net/" + interfaceName + "/device/driver/module/drivers/vpn",
		} {
			if _, err := os.Stat(path); err == nil {
				return true
			}
		}
	}

	return false
}
