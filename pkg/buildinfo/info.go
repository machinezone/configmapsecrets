package buildinfo

import (
	"bytes"
	"html/template"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	binary  = "unknown"
	version = "unknown"

	repo     = "unknown"
	revision = "unknown"
	branch   = "unknown"

	buildTime time.Time
	buildUnix string
)

func init() {
	if t, err := strconv.ParseInt(buildUnix, 10, 64); err == nil {
		buildTime = time.Unix(t, 0).UTC()
	}
}

// Collector returns a collector for build info metrics.
func Collector() prometheus.Collector {
	info := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "build_info",
			Help: "Build information (binary, version, repo, revision, branch)",
		},
		[]string{"binary", "version", "repo", "revision", "branch"},
	)
	info.WithLabelValues(binary, version, repo, revision, branch).Set(1)
	if buildTime.IsZero() {
		return info
	}
	time := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "build_time",
			Help: "Build time as Unix timestamp",
		},
	)
	time.Set(float64(buildTime.Unix()))
	return collectors{info, time}
}

type collectors []prometheus.Collector

func (s collectors) Describe(ch chan<- *prometheus.Desc) {
	for _, c := range s {
		c.Describe(ch)
	}
}

func (s collectors) Collect(ch chan<- prometheus.Metric) {
	for _, c := range s {
		c.Collect(ch)
	}
}

var tmpl = template.Must(template.New("version").Parse(`{{.binary}}
  version:    {{.version}}
  repo:       {{.repo}}
  revision:   {{.revision}}
  branch:     {{.branch}}
  build time: {{.buildTime}}
  go version: {{.goVersion}}
`))

// Print returns a string containing build information.
func Print() string {
	m := map[string]string{
		"binary":    binary,
		"version":   version,
		"repo":      repo,
		"branch":    branch,
		"revision":  revision,
		"buildTime": buildTime.Format("2006-01-02 15:04:05 MST"),
		"goVersion": runtime.Version(),
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "version", m); err != nil {
		panic(err)
	}
	return strings.TrimSpace(buf.String())
}

// Binary returns the name of the binary.
func Binary() string { return binary }

// Version returns the version of the binary.
func Version() string { return version }

// Repo returns the repo from which the binary was built.
func Repo() string { return repo }

// Revision returns the revision from which the binary was built.
func Revision() string { return revision }

// Branch returns the branch from which the binary was built.
func Branch() string { return branch }

// BuildTime returns the time the binary was built.
func BuildTime() time.Time { return buildTime }
