// Package layer7 implements application layer (OSI Layer 7) testing functionality
package layer7

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"ghostshell/app/layers/common"
)

// Runner implements application layer tests
type Runner struct {
	Endpoints       []string
	Timeout         time.Duration
	HTTPMethods     []string
	Headers         map[string]string
	FollowRedirects bool
	VerifySSL       bool
	ValidateContent bool
	ContentPattern  string
	BasicAuth       struct {
		Username string
		Password string
		Enabled  bool
	}
	BearerToken string
	Proxy       string
}

// HTTPRequestInfo stores detailed information about an HTTP request
type HTTPRequestInfo struct {
	URL               string            `json:"url"`
	Method            string            `json:"method"`
	DNSLookupTime     time.Duration     `json:"dns_lookup_time_ms"`
	ConnectTime       time.Duration     `json:"connect_time_ms"`
	TLSHandshakeTime  time.Duration     `json:"tls_handshake_time_ms,omitempty"`
	FirstByteTime     time.Duration     `json:"first_byte_time_ms"`
	TotalTime         time.Duration     `json:"total_time_ms"`
	StatusCode        int               `json:"status_code"`
	ContentLength     int64             `json:"content_length"`
	ContentType       string            `json:"content_type"`
	RemoteAddr        string            `json:"remote_addr"`
	TLSVersion        string            `json:"tls_version,omitempty"`
	TLSCipherSuite    string            `json:"tls_cipher_suite,omitempty"`
	CertificateExpiry time.Time         `json:"certificate_expiry,omitempty"`
	ServerHeaders     map[string]string `json:"server_headers"`
	RedirectCount     int               `json:"redirect_count"`
	Error             string            `json:"error,omitempty"`
	ContentMatch      bool              `json:"content_match,omitempty"`
}

// New creates a new Layer7Runner
func New(endpoints []string, timeout time.Duration) *Runner {
	// Default HTTP methods if none are specified
	methods := []string{"GET"}

	return &Runner{
		Endpoints:       endpoints,
		Timeout:         timeout,
		HTTPMethods:     methods,
		Headers:         make(map[string]string),
		FollowRedirects: true,
		VerifySSL:       true,
		ValidateContent: false,
		ContentPattern:  "",
	}
}

// WithHTTPMethods adds HTTP methods to test
func (r *Runner) WithHTTPMethods(methods []string) *Runner {
	if len(methods) > 0 {
		r.HTTPMethods = methods
	}
	return r
}

// WithHeaders adds custom HTTP headers
func (r *Runner) WithHeaders(headers map[string]string) *Runner {
	for k, v := range headers {
		r.Headers[k] = v
	}
	return r
}

// WithBasicAuth adds basic authentication
func (r *Runner) WithBasicAuth(username, password string) *Runner {
	r.BasicAuth.Username = username
	r.BasicAuth.Password = password
	r.BasicAuth.Enabled = true
	return r
}

// WithBearerToken adds bearer token authentication
func (r *Runner) WithBearerToken(token string) *Runner {
	r.BearerToken = token
	return r
}

// WithContentValidation adds content validation
func (r *Runner) WithContentValidation(pattern string) *Runner {
	r.ValidateContent = true
	r.ContentPattern = pattern
	return r
}

// WithProxy sets a proxy server
func (r *Runner) WithProxy(proxyURL string) *Runner {
	r.Proxy = proxyURL
	return r
}

// GetName returns the name of this layer
func (r *Runner) GetName() string {
	return "Application Layer"
}

// GetDescription returns a description of this layer's functionality
func (r *Runner) GetDescription() string {
	return "Tests application layer protocols including HTTP, HTTPS, and API endpoints"
}

// GetDependencies returns the layer numbers this layer depends on
func (r *Runner) GetDependencies() []int {
	// Layer 7 depends on lower layers
	return []int{3, 4, 5, 6}
}

// ValidateConfig validates the configuration for this layer
func (r *Runner) ValidateConfig() error {
	if len(r.Endpoints) == 0 {
		return fmt.Errorf("at least one endpoint must be specified")
	}

	// Validate URLs
	for _, endpoint := range r.Endpoints {
		if _, err := url.Parse(endpoint); err != nil {
			return fmt.Errorf("invalid endpoint URL '%s': %w", endpoint, err)
		}
	}

	if r.Timeout <= 0 {
		return fmt.Errorf("timeout must be greater than 0")
	}

	return nil
}

// RunTests implements the LayerRunner interface
func (r *Runner) RunTests(ctx context.Context, logger *zap.Logger) ([]common.TestResult, error) {
	logger.Info("Starting Layer 7 (Application Layer) tests...",
		zap.Strings("endpoints", r.Endpoints),
		zap.Duration("timeout", r.Timeout),
		zap.Strings("methods", r.HTTPMethods),
		zap.Bool("follow_redirects", r.FollowRedirects),
		zap.Bool("verify_ssl", r.VerifySSL),
		zap.Bool("validate_content", r.ValidateContent))

	startTime := time.Now()

	// Create parent result
	parentResult := common.TestResult{
		Layer:      7,
		Name:       "Application Layer Tests",
		Status:     common.StatusPassed,
		StartTime:  startTime,
		SubResults: []common.TestResult{},
	}

	// Add default headers if none specified
	if len(r.Headers) == 0 {
		r.Headers = map[string]string{
			"User-Agent": "GhostSuite/2.0",
			"Accept":     "*/*",
		}
	}

	// Test each endpoint with specified methods
	var wg sync.WaitGroup
	resultsChan := make(chan common.TestResult, len(r.Endpoints)*len(r.HTTPMethods))

	for _, endpoint := range r.Endpoints {
		for _, method := range r.HTTPMethods {
			// Skip if context is cancelled
			if ctx.Err() != nil {
				logger.Warn("Context cancelled, skipping remaining tests")
				break
			}

			endpoint := endpoint
			method := method

			wg.Add(1)
			go func() {
				defer wg.Done()

				testResult := common.TestResult{
					Layer:     7,
					Name:      fmt.Sprintf("%s %s", method, endpoint),
					StartTime: time.Now(),
					Metrics:   common.TestMetrics{},
				}

				// Create HTTP client
				client, err := r.createHTTPClient()
				if err != nil {
					testResult.Status = common.StatusFailed
					testResult.Message = fmt.Sprintf("Failed to create HTTP client: %v", err)
					testResult.EndTime = time.Now()
					testResult.Metrics.Duration = testResult.EndTime.Sub(testResult.StartTime)
					resultsChan <- testResult
					return
				}

				// Execute the test
				requestInfo, err := r.executeHTTPRequest(ctx, client, method, endpoint)

				// Set end time and duration
				testResult.EndTime = time.Now()
				testResult.Metrics.Duration = testResult.EndTime.Sub(testResult.StartTime)

				// Set metrics
				if requestInfo != nil {
					testResult.Metrics.Latency = time.Duration(requestInfo.FirstByteTime.Milliseconds()) * time.Millisecond
					testResult.Metrics.ResponseTime = time.Duration(requestInfo.TotalTime.Milliseconds()) * time.Millisecond

					// Set custom metrics
					testResult.Metrics.Custom = map[string]interface{}{
						"dns_lookup_time_ms":    requestInfo.DNSLookupTime.Milliseconds(),
						"connect_time_ms":       requestInfo.ConnectTime.Milliseconds(),
						"tls_handshake_time_ms": requestInfo.TLSHandshakeTime.Milliseconds(),
						"first_byte_time_ms":    requestInfo.FirstByteTime.Milliseconds(),
						"total_time_ms":         requestInfo.TotalTime.Milliseconds(),
						"status_code":           requestInfo.StatusCode,
						"content_length":        requestInfo.ContentLength,
						"redirect_count":        requestInfo.RedirectCount,
					}

					// Set diagnostic data
					testResult.Diagnostics = requestInfo
				}

				// Determine test status
				if err != nil {
					testResult.Status = common.StatusFailed
					testResult.Message = fmt.Sprintf("Request failed: %v", err)
				} else if requestInfo.StatusCode >= 400 {
					testResult.Status = common.StatusFailed
					testResult.Message = fmt.Sprintf("Received HTTP status %d", requestInfo.StatusCode)
				} else if r.ValidateContent && !requestInfo.ContentMatch {
					testResult.Status = common.StatusFailed
					testResult.Message = fmt.Sprintf("Content validation failed: pattern '%s' not found", r.ContentPattern)
				} else if requestInfo.StatusCode >= 300 && requestInfo.StatusCode < 400 && !r.FollowRedirects {
					testResult.Status = common.StatusWarning
					testResult.Message = fmt.Sprintf("Received HTTP redirect status %d but redirection not followed", requestInfo.StatusCode)
				} else {
					testResult.Status = common.StatusPassed
					testResult.Message = fmt.Sprintf("Successfully tested %s %s (Status: %d, Time: %d ms)",
						method, endpoint, requestInfo.StatusCode, requestInfo.TotalTime.Milliseconds())
				}

				resultsChan <- testResult
			}()
		}
	}

	// Wait for all tests to complete
	wg.Wait()
	close(resultsChan)

	// Process results
	var subResults []common.TestResult
	failureCount := 0
	warningCount := 0

	for result := range resultsChan {
		subResults = append(subResults, result)

		switch result.Status {
		case common.StatusFailed:
			failureCount++
		case common.StatusWarning:
			warningCount++
		}
	}

	// Update parent result
	parentResult.SubResults = subResults
	parentResult.EndTime = time.Now()
	parentResult.Metrics.Duration = parentResult.EndTime.Sub(parentResult.StartTime)

	// Calculate average response time
	var totalResponseTime time.Duration
	for _, result := range subResults {
		totalResponseTime += result.Metrics.ResponseTime
	}
	if len(subResults) > 0 {
		parentResult.Metrics.ResponseTime = totalResponseTime / time.Duration(len(subResults))
	}

	// Determine overall status
	if failureCount > 0 {
		parentResult.Status = common.StatusFailed
		parentResult.Message = fmt.Sprintf("Layer 7 tests failed with %d failures and %d warnings",
			failureCount, warningCount)
		return []common.TestResult{parentResult}, fmt.Errorf("layer 7 tests failed with %d failures", failureCount)
	} else if warningCount > 0 {
		parentResult.Status = common.StatusWarning
		parentResult.Message = fmt.Sprintf("Layer 7 tests completed with %d warnings", warningCount)
	} else {
		parentResult.Status = common.StatusPassed
		parentResult.Message = "All application layer tests passed successfully"
	}

	logger.Info("Layer 7 tests completed",
		zap.String("status", string(parentResult.Status)),
		zap.Int("sub_tests", len(subResults)),
		zap.Int("failures", failureCount),
		zap.Int("warnings", warningCount),
	)

	return []common.TestResult{parentResult}, nil
}

// createHTTPClient creates an HTTP client with the given options
func (r *Runner) createHTTPClient() (*http.Client, error) {
	// Set up TLS configuration
	tlsConfig := &tls.Config{
		InsecureSkipVerify: !r.VerifySSL,
	}

	// Set up transport with TLS config
	transport := &http.Transport{
		TLSClientConfig:     tlsConfig,
		DisableCompression:  false,
		DisableKeepAlives:   false,
		MaxIdleConnsPerHost: 10,
		Proxy:               http.ProxyFromEnvironment,
	}

	// Add proxy if specified
	if r.Proxy != "" {
		proxyURL, err := url.Parse(r.Proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	// Create client
	client := &http.Client{
		Transport: transport,
		Timeout:   r.Timeout,
	}

	// Configure redirect handling
	if !r.FollowRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	} else {
		// Set a maximum redirect limit
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		}
	}

	return client, nil
}

// executeHTTPRequest performs an HTTP request and captures detailed metrics
func (r *Runner) executeHTTPRequest(ctx context.Context, client *http.Client, method string, endpoint string) (*HTTPRequestInfo, error) {
	reqInfo := &HTTPRequestInfo{
		URL:           endpoint,
		Method:        method,
		ServerHeaders: make(map[string]string),
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		reqInfo.Error = err.Error()
		return reqInfo, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}

	// Add auth if specified
	if r.BasicAuth.Enabled {
		req.SetBasicAuth(r.BasicAuth.Username, r.BasicAuth.Password)
	} else if r.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+r.BearerToken)
	}

	// Timing variables
	var dnsStart, connectStart, tlsStart, firstByteStart time.Time
	var dnsTime, connectTime, tlsTime, firstByteTime time.Duration
	redirectCount := 0

	// Create HTTP trace to capture detailed timing
	trace := &httptrace.ClientTrace{
		DNSStart: func(info httptrace.DNSStartInfo) {
			dnsStart = time.Now()
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			dnsTime = time.Since(dnsStart)
		},
		ConnectStart: func(network, addr string) {
			connectStart = time.Now()
		},
		ConnectDone: func(network, addr string, err error) {
			connectTime = time.Since(connectStart)
		},
		TLSHandshakeStart: func() {
			tlsStart = time.Now()
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			tlsTime = time.Since(tlsStart)

			// Capture TLS details if available
			if err == nil {
				reqInfo.TLSVersion = tlsVersionToString(state.Version)
				reqInfo.TLSCipherSuite = tls.CipherSuiteName(state.CipherSuite)

				// Get certificate expiry
				if len(state.PeerCertificates) > 0 {
					reqInfo.CertificateExpiry = state.PeerCertificates[0].NotAfter
				}
			}
		},
		GotFirstResponseByte: func() {
			firstByteTime = time.Since(firstByteStart)
		},
	}

	// Set up redirect tracking
	if r.FollowRedirects {
		origCheckRedirect := client.CheckRedirect
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			redirectCount++
			if origCheckRedirect != nil {
				return origCheckRedirect(req, via)
			}
			return nil
		}
	}

	// Apply the trace to the request context
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	// Start timing
	startTime := time.Now()
	firstByteStart = startTime

	// Execute request
	resp, err := client.Do(req)
	totalTime := time.Since(startTime)

	// Capture timing information
	reqInfo.DNSLookupTime = dnsTime
	reqInfo.ConnectTime = connectTime
	reqInfo.TLSHandshakeTime = tlsTime
	reqInfo.FirstByteTime = firstByteTime
	reqInfo.TotalTime = totalTime
	reqInfo.RedirectCount = redirectCount

	if err != nil {
		reqInfo.Error = err.Error()
		return reqInfo, err
	}
	defer resp.Body.Close()

	// Capture response information
	reqInfo.StatusCode = resp.StatusCode
	reqInfo.ContentLength = resp.ContentLength
	reqInfo.ContentType = resp.Header.Get("Content-Type")
	if resp.TLS != nil {
		reqInfo.TLSVersion = tlsVersionToString(resp.TLS.Version)
		reqInfo.TLSCipherSuite = tls.CipherSuiteName(resp.TLS.CipherSuite)
	}
	if addr := resp.Request.RemoteAddr; addr != "" {
		reqInfo.RemoteAddr = addr
	} else if addr := req.RemoteAddr; addr != "" {
		reqInfo.RemoteAddr = addr
	}

	// Capture headers
	for k, v := range resp.Header {
		if len(v) > 0 {
			reqInfo.ServerHeaders[k] = strings.Join(v, ", ")
		}
	}

	// Read response body if content validation is enabled
	if r.ValidateContent && r.ContentPattern != "" {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return reqInfo, fmt.Errorf("failed to read response body: %w", err)
		}

		// Validate content
		contentRegex, err := regexp.Compile(r.ContentPattern)
		if err != nil {
			return reqInfo, fmt.Errorf("invalid content pattern: %w", err)
		}

		reqInfo.ContentMatch = contentRegex.Match(body)
	}

	return reqInfo, nil
}

// tlsVersionToString converts TLS version constants to human-readable strings
func tlsVersionToString(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("Unknown (0x%04x)", version)
	}
}

// ExecuteJSONRequest performs an HTTP request with JSON payload and response parsing
func (r *Runner) ExecuteJSONRequest(ctx context.Context, method, endpoint string, requestBody, responsePtr interface{}) (*HTTPRequestInfo, error) {
	// Create HTTP client
	client, err := r.createHTTPClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	var bodyBuffer *bytes.Buffer

	// Marshal request body if provided
	if requestBody != nil {
		jsonData, err := json.Marshal(requestBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyBuffer = bytes.NewBuffer(jsonData)
	} else {
		bodyBuffer = &bytes.Buffer{}
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyBuffer)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set content type for JSON
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Add headers
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}

	// Add auth if specified
	if r.BasicAuth.Enabled {
		req.SetBasicAuth(r.BasicAuth.Username, r.BasicAuth.Password)
	} else if r.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+r.BearerToken)
	}

	// Execute request
	reqInfo := &HTTPRequestInfo{
		URL:           endpoint,
		Method:        method,
		ServerHeaders: make(map[string]string),
	}

	startTime := time.Now()
	resp, err := client.Do(req)
	reqInfo.TotalTime = time.Since(startTime)

	if err != nil {
		reqInfo.Error = err.Error()
		return reqInfo, err
	}
	defer resp.Body.Close()

	// Capture response information
	reqInfo.StatusCode = resp.StatusCode
	reqInfo.ContentLength = resp.ContentLength
	reqInfo.ContentType = resp.Header.Get("Content-Type")

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return reqInfo, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse JSON response if pointer provided
	if responsePtr != nil && len(body) > 0 {
		if err := json.Unmarshal(body, responsePtr); err != nil {
			return reqInfo, fmt.Errorf("failed to parse JSON response: %w", err)
		}
	}

	return reqInfo, nil
}

// CreateTestSuite creates a suite of tests for common web services
func CreateTestSuite(baseURL string, timeout time.Duration) *Runner {
	// Create base runner
	runner := New([]string{}, timeout)

	// Ensure base URL ends with a slash
	if !strings.HasSuffix(baseURL, "/") {
		baseURL = baseURL + "/"
	}

	// Create common endpoints to test
	endpoints := []string{
		baseURL,                 // Homepage
		baseURL + "robots.txt",  // Robots.txt
		baseURL + "sitemap.xml", // Sitemap
		baseURL + "api/health",  // Health check
		baseURL + "api/status",  // Status API
	}

	runner.Endpoints = endpoints
	runner.HTTPMethods = []string{"GET", "HEAD"}
	runner.FollowRedirects = true
	runner.VerifySSL = true

	return runner
}
