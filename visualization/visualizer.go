package visualization

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"ghostshell/app/layers/common"
)

//go:embed templates/*
var templateFS embed.FS

// Visualizer manages the web-based visualization of test results
type Visualizer struct {
	logger     *zap.Logger
	results    []common.TestResult
	mu         sync.RWMutex
	httpServer *http.Server
	metrics    *metrics
}

// metrics holds Prometheus metrics for test results
type metrics struct {
	testsPassed prometheus.Counter
	testsFailed prometheus.Counter
	testLatency prometheus.Histogram
	layerStatus *prometheus.GaugeVec
}

// NewVisualizer creates a new web-based visualizer
func NewVisualizer(logger *zap.Logger) (*Visualizer, error) {
	// Initialize Prometheus metrics
	m := &metrics{
		testsPassed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "osi_tests_passed_total",
			Help: "Total number of passed OSI layer tests",
		}),
		testsFailed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "osi_tests_failed_total",
			Help: "Total number of failed OSI layer tests",
		}),
		testLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "osi_test_duration_seconds",
			Help:    "Duration of OSI layer tests",
			Buckets: prometheus.LinearBuckets(0.1, 0.1, 10),
		}),
		layerStatus: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "osi_layer_status",
			Help: "Status of each OSI layer (0=failed, 1=passed)",
		}, []string{"layer"}),
	}

	// Register metrics
	prometheus.MustRegister(m.testsPassed)
	prometheus.MustRegister(m.testsFailed)
	prometheus.MustRegister(m.testLatency)
	prometheus.MustRegister(m.layerStatus)

	return &Visualizer{
		logger:  logger,
		metrics: m,
	}, nil
}

// Start initializes and starts the web server
func (v *Visualizer) Start(addr string) error {
	// Create router
	mux := http.NewServeMux()

	// Register handlers
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/", v.handleDashboard)
	mux.HandleFunc("/api/results", v.handleResults)

	// Create server
	v.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Start server
	v.logger.Info("Starting visualization server", zap.String("addr", addr))
	return v.httpServer.ListenAndServe()
}

// Stop gracefully shuts down the server
func (v *Visualizer) Stop() error {
	if v.httpServer != nil {
		return v.httpServer.Close()
	}
	return nil
}

// UpdateResults updates the test results and metrics
func (v *Visualizer) UpdateResults(results []common.TestResult) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.results = results

	// Update metrics
	passed := 0
	failed := 0
	for _, result := range results {
		if result.Status == "Passed" {
			passed++
			v.metrics.layerStatus.WithLabelValues(fmt.Sprintf("layer%d", result.Layer)).Set(1)
		} else {
			failed++
			v.metrics.layerStatus.WithLabelValues(fmt.Sprintf("layer%d", result.Layer)).Set(0)
		}
	}

	v.metrics.testsPassed.Add(float64(passed))
	v.metrics.testsFailed.Add(float64(failed))
	v.metrics.testLatency.Observe(time.Since(time.Now()).Seconds())
}

// handleDashboard serves the main dashboard page
func (v *Visualizer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(templateFS, "templates/dashboard.html")
	if err != nil {
		http.Error(w, "Failed to load template", http.StatusInternalServerError)
		return
	}

	v.mu.RLock()
	data := struct {
		Results []common.TestResult
		Time    time.Time
	}{
		Results: v.results,
		Time:    time.Now(),
	}
	v.mu.RUnlock()

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		return
	}
}

// handleResults serves the test results as JSON
func (v *Visualizer) handleResults(w http.ResponseWriter, r *http.Request) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v.results); err != nil {
		http.Error(w, "Failed to encode results", http.StatusInternalServerError)
		return
	}
}
