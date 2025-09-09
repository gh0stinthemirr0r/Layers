// Package layer2 implements data link layer (OSI Layer 2) testing functionality
package layer2

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"ghostshell/app/layers/common"
)

// Runner implements data link layer tests
type Runner struct {
	*common.Layer2Runner
}

// GetDependencies returns the layer numbers this layer depends on
func (r *Runner) GetDependencies() []int {
	return []int{1} // Layer 2 depends on Layer 1
}

// GetDescription returns a description of this layer's functionality
func (r *Runner) GetDescription() string {
	return "Tests data link layer functionality including MAC addressing and frame handling"
}

// GetName returns the name of this layer
func (r *Runner) GetName() string {
	return "Data Link Layer"
}

// ValidateConfig validates the configuration for this layer
func (r *Runner) ValidateConfig() error {
	if len(r.Targets) == 0 {
		return fmt.Errorf("at least one target must be specified")
	}
	return nil
}

// New creates a new Layer2Runner with the specified parameters
func New(targets []string, checkMAC bool, checkMTU bool) *Runner {
	return &Runner{
		Layer2Runner: &common.Layer2Runner{
			Targets:  targets,
			CheckMAC: checkMAC,
			CheckMTU: checkMTU,
		},
	}
}

// RunTests implements the LayerRunner interface
func (r *Runner) RunTests(ctx context.Context, logger *zap.Logger) ([]common.TestResult, error) {
	logger.Info("Starting Layer 2 (Data Link Layer) tests...")

	interfaces, err := net.Interfaces()
	if err != nil {
		msg := fmt.Sprintf("Failed to get network interfaces: %v", err)
		logger.Error(msg)
		return []common.TestResult{{
			Layer:   2,
			Status:  common.StatusFailed,
			Message: msg,
		}}, err
	}

	if len(interfaces) == 0 {
		msg := "No network interfaces found"
		logger.Error(msg)
		return []common.TestResult{{
			Layer:   2,
			Status:  common.StatusFailed,
			Message: msg,
		}}, fmt.Errorf(msg)
	}

	var subResults []common.TestResult
	var failedTests []string
	var warningTests []string
	successCount := 0

	// Test each interface (excluding loopback)
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// Create a result for this interface
		ifaceResult := common.TestResult{
			Layer:     2,
			Name:      fmt.Sprintf("Interface %s Test", iface.Name),
			StartTime: time.Now(),
		}

		var ifaceIssues []string
		var ifaceWarnings []string

		// Check if this is a VPN interface
		isVPN := isVPNInterface(iface.Name)

		// Check MAC address if enabled
		if r.CheckMAC {
			if iface.HardwareAddr == nil || len(iface.HardwareAddr) == 0 {
				if isVPN {
					ifaceWarnings = append(ifaceWarnings, "No MAC address (normal for VPN interface)")
				} else {
					ifaceIssues = append(ifaceIssues, "No MAC address available")
				}
			}
		}

		// Check MTU if enabled
		if r.CheckMTU {
			if iface.MTU <= 0 {
				ifaceIssues = append(ifaceIssues, "Invalid MTU value")
			} else if iface.MTU < 1500 {
				if isVPN {
					// VPNs often use different MTU values, so just note it
					ifaceWarnings = append(ifaceWarnings,
						fmt.Sprintf("Non-standard MTU: %d (common for VPN interfaces)", iface.MTU))
				} else {
					ifaceWarnings = append(ifaceWarnings,
						fmt.Sprintf("Non-standard MTU: %d (standard is 1500)", iface.MTU))
				}
			}
		}

		// Get interface addresses
		addrs, err := iface.Addrs()
		if err != nil {
			ifaceIssues = append(ifaceIssues, fmt.Sprintf("Failed to get addresses: %v", err))
		} else if len(addrs) == 0 {
			if isVPN && (iface.Flags&net.FlagUp) == 0 {
				// VPN interface being down with no addresses is normal when not connected
				ifaceWarnings = append(ifaceWarnings, "No IP addresses assigned (VPN might be disconnected)")
			} else {
				ifaceIssues = append(ifaceIssues, "No IP addresses assigned")
			}
		}

		// Get interface details
		operstate, carrier := getInterfaceDetails(iface.Name)
		txBytes, rxBytes := getInterfaceStats(iface.Name)

		// Set result status based on issues found
		if len(ifaceIssues) > 0 {
			ifaceResult.Status = common.StatusFailed
			ifaceResult.Message = fmt.Sprintf("Interface %s failed:\n- %s",
				iface.Name, strings.Join(ifaceIssues, "\n- "))
			if len(ifaceWarnings) > 0 {
				ifaceResult.Message += fmt.Sprintf("\n\nWarnings:\n- %s",
					strings.Join(ifaceWarnings, "\n- "))
			}
			failedTests = append(failedTests, ifaceResult.Message)
		} else if len(ifaceWarnings) > 0 {
			ifaceResult.Status = common.StatusWarning
			ifaceResult.Message = fmt.Sprintf("Interface %s passed with warnings:\n- %s",
				iface.Name, strings.Join(ifaceWarnings, "\n- "))
			warningTests = append(warningTests, ifaceResult.Message)
		} else {
			ifaceResult.Status = common.StatusPassed
			successCount++
			ifaceResult.Message = fmt.Sprintf("Interface %s passed all checks:\n"+
				"- Type: %s\n"+
				"- MAC: %s\n"+
				"- MTU: %d\n"+
				"- Operational State: %s\n"+
				"- Carrier: %d\n"+
				"- Addresses: %s\n"+
				"- TX Bytes: %d\n"+
				"- RX Bytes: %d",
				iface.Name,
				getInterfaceType(iface.Name, isVPN),
				iface.HardwareAddr.String(),
				iface.MTU,
				operstate,
				carrier,
				formatAddresses(addrs),
				txBytes,
				rxBytes)
		}

		ifaceResult.EndTime = time.Now()
		ifaceResult.Metrics.Duration = ifaceResult.EndTime.Sub(ifaceResult.StartTime)
		ifaceResult.Diagnostics = map[string]interface{}{
			"interface":     iface.Name,
			"type":          getInterfaceType(iface.Name, isVPN),
			"hardware_addr": iface.HardwareAddr.String(),
			"mtu":           iface.MTU,
			"flags":         iface.Flags.String(),
			"oper_state":    operstate,
			"carrier":       carrier,
			"tx_bytes":      txBytes,
			"rx_bytes":      rxBytes,
			"addresses":     formatAddresses(addrs),
			"is_vpn":        isVPN,
		}

		subResults = append(subResults, ifaceResult)
	}

	// Create parent result
	parentResult := common.TestResult{
		Layer:      2,
		Name:       "Data Link Layer Tests",
		StartTime:  time.Now(),
		SubResults: subResults,
	}

	// Set overall status and message
	var messageBuilder strings.Builder
	if len(failedTests) > 0 && successCount > 0 {
		parentResult.Status = common.StatusMixed
		messageBuilder.WriteString(fmt.Sprintf("Layer 2 tests completed with mixed results:\n"+
			"- %d interfaces passed\n"+
			"- %d interfaces failed\n"+
			"- %d interfaces have warnings\n\n",
			successCount, len(failedTests), len(warningTests)))
	} else if len(failedTests) > 0 {
		parentResult.Status = common.StatusFailed
		messageBuilder.WriteString(fmt.Sprintf("Layer 2 tests failed with %d failures:\n\n",
			len(failedTests)))
	} else if len(warningTests) > 0 {
		parentResult.Status = common.StatusWarning
		messageBuilder.WriteString(fmt.Sprintf("Layer 2 tests completed with %d warnings:\n\n",
			len(warningTests)))
	} else {
		parentResult.Status = common.StatusPassed
		messageBuilder.WriteString(fmt.Sprintf("All Layer 2 tests passed successfully (%d interfaces tested)\n",
			len(subResults)))
	}

	if len(failedTests) > 0 {
		messageBuilder.WriteString("\nFailures:\n")
		messageBuilder.WriteString(strings.Join(failedTests, "\n\n"))
	}
	if len(warningTests) > 0 {
		messageBuilder.WriteString("\n\nWarnings:\n")
		messageBuilder.WriteString(strings.Join(warningTests, "\n"))
	}

	parentResult.Message = messageBuilder.String()
	parentResult.EndTime = time.Now()
	parentResult.Metrics.Duration = parentResult.EndTime.Sub(parentResult.StartTime)

	logger.Info("Layer 2 tests completed",
		zap.String("status", string(parentResult.Status)),
		zap.Int("total_interfaces", len(subResults)),
		zap.Int("passed", successCount),
		zap.Int("failed", len(failedTests)),
		zap.Int("warnings", len(warningTests)),
	)

	if len(failedTests) > 0 {
		return []common.TestResult{parentResult}, fmt.Errorf("layer 2 tests failed")
	}
	return []common.TestResult{parentResult}, nil
}

// formatAddresses formats a list of network addresses as a string
func formatAddresses(addrs []net.Addr) string {
	var addrStrs []string
	for _, addr := range addrs {
		addrStrs = append(addrStrs, addr.String())
	}
	return strings.Join(addrStrs, ", ")
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
	} else if runtime.GOOS == "windows" {
		// Use PowerShell to get interface status
		cmd := exec.Command("powershell", "-Command",
			fmt.Sprintf("Get-NetAdapter | Where-Object {$_.Name -eq '%s' -or $_.InterfaceDescription -like '*%s*'} | Select-Object -ExpandProperty Status",
				interfaceName, interfaceName))
		output, err := cmd.Output()
		if err == nil {
			status := strings.TrimSpace(string(output))
			operstate = status
			if status == "Up" {
				carrier = 1
			} else {
				carrier = 0
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
	} else if runtime.GOOS == "windows" {
		// Use PowerShell to get interface statistics
		cmd := exec.Command("powershell", "-Command",
			fmt.Sprintf("Get-NetAdapter | Where-Object {$_.Name -eq '%s' -or $_.InterfaceDescription -like '*%s*'} | Get-NetAdapterStatistics | Select-Object -Property ReceivedBytes,SentBytes",
				interfaceName, interfaceName))
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "ReceivedBytes") {
					fields := strings.Fields(line)
					if len(fields) > 0 {
						rxBytes, _ = strconv.ParseInt(fields[len(fields)-1], 10, 64)
					}
				} else if strings.Contains(line, "SentBytes") {
					fields := strings.Fields(line)
					if len(fields) > 0 {
						txBytes, _ = strconv.ParseInt(fields[len(fields)-1], 10, 64)
					}
				}
			}
		}
	}

	return txBytes, rxBytes
}

// getInterfaceType returns a human-readable interface type
func getInterfaceType(interfaceName string, isVPN bool) string {
	if isVPN {
		return "VPN"
	}

	nameLower := strings.ToLower(interfaceName)
	switch {
	case strings.Contains(nameLower, "wifi") || strings.Contains(nameLower, "wlan") || strings.Contains(nameLower, "wireless"):
		return "Wireless"
	case strings.Contains(nameLower, "eth") || strings.Contains(nameLower, "ethernet"):
		return "Ethernet"
	case strings.Contains(nameLower, "bluetooth"):
		return "Bluetooth"
	case strings.Contains(nameLower, "usb"):
		return "USB"
	default:
		return "Unknown"
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
