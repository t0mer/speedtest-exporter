// Package metrics owns the Prometheus registry and all speedtest_ collectors.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/t0mer/speedtest-exporter/internal/model"
)

// Metrics holds all Prometheus collectors on a private registry.
type Metrics struct {
	registry      *prometheus.Registry
	downloadMbps  prometheus.Gauge
	uploadMbps    prometheus.Gauge
	pingMs        prometheus.Gauge
	jitterMs      prometheus.Gauge
	packetLoss    prometheus.Gauge
	lastTimestamp prometheus.Gauge
	serverInfo    *prometheus.GaugeVec
	testsTotal    *prometheus.CounterVec
	testDuration  prometheus.Histogram
	breachesTotal *prometheus.CounterVec
}

// New creates and registers all speedtest_ metrics on a fresh registry.
func New() *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		registry: reg,
		downloadMbps: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "speedtest", Name: "download_mbps",
			Help: "Latest download speed in Mbps.",
		}),
		uploadMbps: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "speedtest", Name: "upload_mbps",
			Help: "Latest upload speed in Mbps.",
		}),
		pingMs: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "speedtest", Name: "ping_ms",
			Help: "Latest ping latency in milliseconds.",
		}),
		jitterMs: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "speedtest", Name: "jitter_ms",
			Help: "Latest jitter in milliseconds.",
		}),
		packetLoss: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "speedtest", Name: "packet_loss_ratio",
			Help: "Latest packet loss as a ratio (0–1).",
		}),
		lastTimestamp: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "speedtest", Name: "last_test_timestamp_seconds",
			Help: "Unix timestamp of the last completed test.",
		}),
		serverInfo: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "speedtest", Name: "server_info",
			Help: "Constant 1, labelled with the test server details.",
		}, []string{"server_name", "server_id", "isp"}),
		testsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "speedtest", Name: "tests_total",
			Help: "Total number of speed tests.",
		}, []string{"source", "outcome"}),
		testDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "speedtest", Name: "test_duration_seconds",
			Help:    "Duration of speed tests.",
			Buckets: []float64{10, 20, 30, 45, 60, 90, 120},
		}),
		breachesTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "speedtest", Name: "threshold_breaches_total",
			Help: "Total number of threshold breaches by metric.",
		}, []string{"metric"}),
	}
	reg.MustRegister(
		m.downloadMbps, m.uploadMbps, m.pingMs, m.jitterMs,
		m.packetLoss, m.lastTimestamp, m.serverInfo,
		m.testsTotal, m.testDuration, m.breachesTotal,
	)
	return m
}

// Update sets all gauge metrics from a completed result.
func (m *Metrics) Update(r *model.Result) {
	m.downloadMbps.Set(r.DownloadMbps)
	m.uploadMbps.Set(r.UploadMbps)
	m.pingMs.Set(r.PingMs)
	m.jitterMs.Set(r.JitterMs)
	m.packetLoss.Set(r.PacketLoss)
	m.lastTimestamp.Set(float64(r.Timestamp.Unix()))
	m.serverInfo.Reset()
	m.serverInfo.WithLabelValues(r.ServerName, r.ServerID, r.ISP).Set(1)
}

// IncrTests increments the tests_total counter for the given source and outcome.
func (m *Metrics) IncrTests(source, outcome string) {
	m.testsTotal.WithLabelValues(source, outcome).Inc()
}

// ObserveDuration records a test duration observation.
func (m *Metrics) ObserveDuration(seconds float64) {
	m.testDuration.Observe(seconds)
}

// IncrBreaches increments threshold_breaches_total for the given metric name.
func (m *Metrics) IncrBreaches(metric string) {
	m.breachesTotal.WithLabelValues(metric).Inc()
}

// Handler returns an HTTP handler that serves the Prometheus metrics page.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
