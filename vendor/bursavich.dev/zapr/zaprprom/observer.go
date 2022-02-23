// SPDX-License-Identifier: BSD-3-Clause
//
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

// An Observer observes zapr metrics for Prometheus.
type Observer interface {
	zapr.Observer
	prometheus.Collector
}

type observer struct {
	lines  *prometheus.CounterVec
	bytes  *prometheus.CounterVec
	errors *prometheus.CounterVec
}

// NewObserver returns new Observer.
func NewObserver() Observer {
	return &observer{
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

func (o *observer) Describe(ch chan<- *prometheus.Desc) {
	o.lines.Describe(ch)
	o.bytes.Describe(ch)
	o.errors.Describe(ch)
}

func (o *observer) Collect(ch chan<- prometheus.Metric) {
	o.lines.Collect(ch)
	o.bytes.Collect(ch)
	o.errors.Collect(ch)
}

func (o *observer) Init(logger string) {
	for _, lvl := range []zapcore.Level{zapcore.InfoLevel, zapcore.ErrorLevel} {
		o.lines.WithLabelValues(logger, lvl.String())
		o.bytes.WithLabelValues(logger, lvl.String())
	}
	o.errors.WithLabelValues(logger)
}

func (o *observer) ObserveEntryLogged(logger string, level string, bytes int) {
	o.bytes.WithLabelValues(logger, level).Add(float64(bytes))
	o.lines.WithLabelValues(logger, level).Inc()
}

func (o *observer) ObserveEncoderError(logger string) {
	o.errors.WithLabelValues(logger).Inc()
}
