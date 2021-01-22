// Copyright 2020 Andy Bursavich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package zaprprom provides a Prometheus metrics implementation for zapr.
package zaprprom

import (
	"bursavich.dev/zapr"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap/zapcore"
)

var (
	_ zapr.Metrics         = (*Metrics)(nil)
	_ prometheus.Collector = (*Metrics)(nil)
)

// Metrics represent the ability to observe zapr metrics for Prometheus.
type Metrics struct {
	lines  *prometheus.CounterVec
	bytes  *prometheus.CounterVec
	errors *prometheus.CounterVec
}

// NewMetrics returns new Metrics.
func NewMetrics() *Metrics {
	return &Metrics{
		lines: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "log_lines_total",
				Help: "Total number of log lines.",
			},
			[]string{"name", "level"},
		),
		bytes: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "log_bytes_total",
				Help: "Total bytes of encoded log lines.",
			},
			[]string{"name", "level"},
		),
		errors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "log_encoder_errors_total",
				Help: "Total number of log entry encoding failures.",
			},
			[]string{"name"},
		),
	}
}

func (m *Metrics) Describe(ch chan<- *prometheus.Desc) {
	m.lines.Describe(ch)
	m.bytes.Describe(ch)
	m.errors.Describe(ch)
}

func (m *Metrics) Collect(ch chan<- prometheus.Metric) {
	m.lines.Collect(ch)
	m.bytes.Collect(ch)
	m.errors.Collect(ch)
}

func (m *Metrics) Init(logger string) {
	for _, lvl := range []zapcore.Level{zapcore.InfoLevel, zapcore.ErrorLevel} {
		m.lines.WithLabelValues(logger, lvl.String())
		m.bytes.WithLabelValues(logger, lvl.String())
	}
	m.errors.WithLabelValues(logger)
}

func (m *Metrics) ObserveEntryLogged(logger string, level string, bytes int) {
	m.bytes.WithLabelValues(logger, level).Add(float64(bytes))
	m.lines.WithLabelValues(logger, level).Inc()
}

func (m *Metrics) ObserveEncoderError(logger string) {
	m.errors.WithLabelValues(logger).Inc()
}
