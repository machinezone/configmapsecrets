package mzlog

import (
	"os"
	"runtime"

	"github.com/machinezone/configmapsecrets/pkg/buildinfo"
	"go.uber.org/zap"
)

// Process logs info about the process to the logger.
func Process(logger *zap.Logger) {
	logger.Info(
		"running",
		zap.String("command", os.Args[0]),
		zap.Strings("args", os.Args[1:]),
	)
	logger.Info(
		"buildinfo",
		zap.String("binary", buildinfo.Binary()),
		zap.String("version", buildinfo.Version()),
		zap.String("repo", buildinfo.Repo()),
		zap.String("branch", buildinfo.Branch()),
		zap.String("revision", buildinfo.Revision()),
		zap.Time("build_time", buildinfo.BuildTime()),
	)
	logger.Info(
		"runtime",
		zap.String("version", runtime.Version()),
		zap.String("os", runtime.GOOS),
		zap.String("arch", runtime.GOARCH),
		zap.Int("gomaxprocs", runtime.GOMAXPROCS(0)),
		zap.Int("cpus", runtime.NumCPU()),
	)
}
