# Zapr
[![License](https://img.shields.io/badge/license-mit-blue.svg?style=for-the-badge)](https://raw.githubusercontent.com/abursavich/zapr/master/LICENSE)
[![GoDev Reference](https://img.shields.io/static/v1?logo=go&logoColor=white&color=00ADD8&label=dev&message=reference&style=for-the-badge)](https://pkg.go.dev/bursavich.dev/zapr)
[![Go Report Card](https://goreportcard.com/badge/bursavich.dev/zapr?style=for-the-badge)](https://goreportcard.com/report/bursavich.dev/zapr)


Zapr provides a [logr.LogSink](https://pkg.go.dev/github.com/go-logr/logr#LogSink) implementation using [zap](https://pkg.go.dev/go.uber.org/zap). It includes optional flag registration, Prometheus metrics, and a standard library [*log.Logger](https://pkg.go.dev/log#Logger) adapter.

## Example

```go
addr := flag.String("http-address", ":8080", "HTTP server listen address.")
zaprObserver := zaprprom.NewObserver()
zaprOptions := zapr.RegisterFlags(flag.CommandLine, zapr.AllOptions(
    zapr.WithObserver(zaprObserver),
    zapr.WithLevel(2), // Override default logging level.
)...)
flag.Parse()

log, sink := zapr.NewLogger(zaprOptions...)
defer sink.Flush() // For most GOOS (linux and darwin), flushing to stderr is a no-op.
log.Info("Hello, zap logr with option flags!")

reg := prometheus.NewRegistry()
reg.MustRegister(
    collectors.NewGoCollector(),
    collectors.NewBuildInfoCollector(),
    collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
    zaprObserver, // Register Observer with Prometheus.
)
log.Info("Hello, zap logr Prometheus metrics!")

mux := http.NewServeMux()
mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

srv := http.Server{
    Addr:     *addr,
    Handler:  mux,
    ErrorLog: zapr.NewStdErrorLogger(sink), // Adapt LogSink to stdlib *log.Logger.
}
if err := srv.ListenAndServe(); err != nil {
    log.Error(err, "Failed to serve HTTP")
}
```
