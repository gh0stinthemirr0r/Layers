package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"ghostshell/app/layers"
	"ghostshell/app/layers/common"
)

// NetworkDetails contains information about network interfaces and their status
type NetworkDetails struct {
	InterfaceName string   `json:"interfaceName"`
	Status        string   `json:"status"`
	IPv4Address   []string `json:"ipv4Address"`
	IPv6Address   []string `json:"ipv6Address"`
	IsPrimary     bool     `json:"isPrimary"`
	IsVPN         bool     `json:"isVPN"`
}

// PortInfo contains information about an open port
type PortInfo struct {
	Port         int    `json:"port"`
	Protocol     string `json:"protocol"`
	Service      string `json:"service"`
	IsVulnerable bool   `json:"isVulnerable"`
}

// SecurityFindings contains the overall security assessment
type SecurityFindings struct {
	NetworkDetails  []NetworkDetails `json:"networkDetails"`
	OpenPorts       []PortInfo       `json:"openPorts"`
	Vulnerabilities []string         `json:"vulnerabilities"`
}

// App struct
type App struct {
	ctx    context.Context
	logger *zap.Logger
}

// NewApp creates a new App application struct
func NewApp() *App {
	// Create logging directory first
	if err := os.MkdirAll(common.LogDir, 0755); err != nil {
		panic(fmt.Sprintf("Failed to create logging directory: %v", err))
	}

	// Initialize logger with enhanced configuration
	config := zap.NewProductionConfig()
	config.OutputPaths = []string{
		"stdout",
		filepath.Join(common.LogDir, fmt.Sprintf("layers_gui_%s.log", time.Now().Format("20060102_150405"))),
	}
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.StacktraceKey = "stacktrace"

	logger, err := config.Build()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}

	return &App{
		logger: logger,
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Create required directories relative to the current directory
	dirs := []string{common.LogDir, common.ReportDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			a.logger.Error("Failed to create directory",
				zap.String("dir", dir),
				zap.Error(err),
			)
			runtime.LogError(ctx, fmt.Sprintf("Failed to create directory %s: %v", dir, err))
		} else {
			a.logger.Info("Created directory", zap.String("dir", dir))
		}
	}
}

// RunLayerTests executes tests for the selected OSI layers
func (a *App) RunLayerTests(selectedLayers []int) ([]common.TestResult, error) {
	a.logger.Info("Starting layer tests execution",
		zap.Ints("layers", selectedLayers),
		zap.String("timestamp", time.Now().Format(time.RFC3339)),
	)
	runtime.LogInfo(a.ctx, fmt.Sprintf("Starting tests for layers: %v", selectedLayers))

	// Log progress to GUI
	runtime.EventsEmit(a.ctx, "test_status", "Running tests...")

	results, err := layers.RunLayerTests(selectedLayers)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to run layer tests: %v", err)
		a.logger.Error(errMsg,
			zap.Error(err),
			zap.Ints("failed_layers", selectedLayers),
		)
		runtime.LogError(a.ctx, errMsg)
		runtime.EventsEmit(a.ctx, "test_status", "Test execution failed")
		return nil, fmt.Errorf("failed to run layer tests: %w", err)
	}

	// Get security findings
	findings, err := a.GetSecurityFindings()
	if err != nil {
		a.logger.Warn("Failed to get security findings",
			zap.Error(err),
		)
	} else if findings != nil {
		// Create diagnostic data structure
		diagnostics := map[string]interface{}{
			"network_info":    findings.NetworkDetails,
			"open_ports":      findings.OpenPorts,
			"vulnerabilities": findings.Vulnerabilities,
		}

		// Add security findings as a special test result
		results = append(results, common.TestResult{
			Layer:       8, // Special layer ID for security findings
			Name:        "Security Findings",
			Status:      common.StatusWarning, // Default to warning to ensure visibility
			Message:     "Comprehensive security assessment results",
			StartTime:   time.Now().Add(-1 * time.Second),
			EndTime:     time.Now(),
			Diagnostics: diagnostics,
		})
	}

	a.logger.Info("Layer tests completed successfully",
		zap.Int("total_results", len(results)),
		zap.String("timestamp", time.Now().Format(time.RFC3339)),
	)
	runtime.LogInfo(a.ctx, fmt.Sprintf("Completed tests for %d layers", len(results)))

	// Generate PDF report
	reportPath := a.GetReportPath()
	a.logger.Info("Generating PDF report", zap.String("path", reportPath))
	runtime.EventsEmit(a.ctx, "test_status", "Generating report...")

	if err := common.WritePDFReport(results, reportPath); err != nil {
		errMsg := fmt.Sprintf("Failed to generate PDF report: %v", err)
		a.logger.Error(errMsg,
			zap.String("path", reportPath),
			zap.Error(err),
		)
		runtime.LogError(a.ctx, errMsg)
		runtime.EventsEmit(a.ctx, "test_status", "Report generation failed")
	} else {
		a.logger.Info("Generated PDF report successfully",
			zap.String("path", reportPath),
			zap.String("timestamp", time.Now().Format(time.RFC3339)),
		)
		runtime.LogInfo(a.ctx, fmt.Sprintf("Report saved to: %s", reportPath))
		runtime.EventsEmit(a.ctx, "test_status", "Tests completed successfully")
	}

	return results, nil
}

// GetReportPath returns the path where the test report will be saved
func (a *App) GetReportPath() string {
	timestamp := time.Now().Format("20060102_150405")
	return filepath.Join(common.ReportDir, fmt.Sprintf("layer_tests_%s.pdf", timestamp))
}

// GetNetworkDetails retrieves detailed information about network interfaces
func (a *App) GetNetworkDetails() ([]NetworkDetails, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get network interfaces: %w", err)
	}

	var details []NetworkDetails
	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		var ipv4, ipv6 []string
		for _, addr := range addrs {
			ip, _, err := net.ParseCIDR(addr.String())
			if err != nil {
				continue
			}
			if ip.To4() != nil {
				ipv4 = append(ipv4, ip.String())
			} else {
				ipv6 = append(ipv6, ip.String())
			}
		}

		details = append(details, NetworkDetails{
			InterfaceName: iface.Name,
			Status:        getInterfaceStatus(iface),
			IPv4Address:   ipv4,
			IPv6Address:   ipv6,
			IsPrimary:     isPrimaryInterface(iface),
			IsVPN:         isVPNInterface(iface),
		})
	}
	return details, nil
}

// ScanPorts scans for open ports on the local system
func (a *App) ScanPorts() ([]PortInfo, error) {
	var ports []PortInfo
	var mutex sync.Mutex
	var wg sync.WaitGroup

	commonPorts := []int{21, 22, 23, 25, 53, 80, 110, 143, 443, 445, 3389, 8080}
	vulnPorts := map[int]string{
		21:   "FTP",
		23:   "Telnet",
		135:  "RPC",
		137:  "NetBIOS",
		445:  "SMB",
		3389: "RDP",
	}

	for _, port := range commonPorts {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			addr := fmt.Sprintf("127.0.0.1:%d", p)
			conn, err := net.DialTimeout("tcp", addr, time.Second)
			if err == nil {
				conn.Close()
				mutex.Lock()
				ports = append(ports, PortInfo{
					Port:         p,
					Protocol:     "TCP",
					Service:      getServiceName(p),
					IsVulnerable: vulnPorts[p] != "",
				})
				mutex.Unlock()
			}
		}(port)
	}

	wg.Wait()
	return ports, nil
}

// GetSecurityFindings performs a comprehensive security assessment
func (a *App) GetSecurityFindings() (*SecurityFindings, error) {
	networkDetails, err := a.GetNetworkDetails()
	if err != nil {
		return nil, fmt.Errorf("failed to get network details: %w", err)
	}

	openPorts, err := a.ScanPorts()
	if err != nil {
		return nil, fmt.Errorf("failed to scan ports: %w", err)
	}

	findings := &SecurityFindings{
		NetworkDetails:  networkDetails,
		OpenPorts:       openPorts,
		Vulnerabilities: analyzeVulnerabilities(networkDetails, openPorts),
	}

	return findings, nil
}

// Helper functions
func getInterfaceStatus(iface net.Interface) string {
	if iface.Flags&net.FlagUp != 0 {
		return "UP"
	}
	return "DOWN"
}

func isPrimaryInterface(iface net.Interface) bool {
	return iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0
}

func isVPNInterface(iface net.Interface) bool {
	name := strings.ToLower(iface.Name)
	vpnPatterns := []string{
		"tun", "tap", "ppp", "vpn", "ipsec", "wg",
		"cisco", "anyconnect", "ac_", "vpn_", "pangp",
		"gpd", "globalprotect", "paloalto", "pan",
		"pulse", "juniper", "network_connect",
		"f5", "bigip", "edge",
		"checkpoint", "snx", "capsule",
		"forticlient", "fortinet", "fortissl",
		"sonicwall", "netextender", "swgp",
		"citrix", "netscaler",
	}

	for _, pattern := range vpnPatterns {
		if strings.Contains(name, pattern) {
			return true
		}
	}
	return false
}

func getServiceName(port int) string {
	services := map[int]string{
		21:   "FTP",
		22:   "SSH",
		23:   "Telnet",
		25:   "SMTP",
		53:   "DNS",
		80:   "HTTP",
		110:  "POP3",
		143:  "IMAP",
		443:  "HTTPS",
		445:  "SMB",
		3389: "RDP",
		8080: "HTTP-ALT",
	}
	if service, ok := services[port]; ok {
		return service
	}
	return "Unknown"
}

func analyzeVulnerabilities(networkDetails []NetworkDetails, openPorts []PortInfo) []string {
	var vulnerabilities []string

	// Check for VPN interfaces
	vpnFound := false
	for _, nd := range networkDetails {
		if nd.IsVPN {
			vpnFound = true
			break
		}
	}
	if !vpnFound {
		vulnerabilities = append(vulnerabilities, "No VPN interface detected - consider using a VPN for enhanced security")
	}

	// Check for vulnerable ports
	for _, port := range openPorts {
		if port.IsVulnerable {
			vulnerabilities = append(vulnerabilities,
				fmt.Sprintf("Potentially vulnerable port %d (%s) is open",
					port.Port, port.Service))
		}
	}

	return vulnerabilities
}
