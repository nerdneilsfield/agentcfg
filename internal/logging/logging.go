// Package logging constructs the CLI logger.
//
// The CLI logs to stderr only because gen reserves stdout for generated
// artifacts. zapper v1.5.3 has no option to route its console core away
// from stdout, so this package builds a zap stderr core and exposes it
// through the zapper.Zapper facade interface; application code depends
// only on zapper types.
package logging

import (
	"os"

	"github.com/GoFarsi/zapper"
	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Level selects the initial log level.
type Level int

const (
	LevelWarn Level = iota
	LevelInfo
	LevelDebug
)

// New returns a zapper-facade logger writing structured logs to stderr.
func New(level Level) zapper.Zapper {
	zapLevel := zapcore.WarnLevel
	switch level {
	case LevelInfo:
		zapLevel = zapcore.InfoLevel
	case LevelDebug:
		zapLevel = zapcore.DebugLevel
	}
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
			TimeKey:        "time",
			LevelKey:       "level",
			NameKey:        "logger",
			MessageKey:     "msg",
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
		}),
		zapcore.Lock(os.Stderr),
		zapLevel,
	)
	return &stderrLogger{z: zap.New(core, zap.AddCaller())}
}

type stderrLogger struct {
	z     *zap.Logger
	sugar *zap.SugaredLogger
}

func (l *stderrLogger) lazy() *zap.SugaredLogger {
	if l.sugar == nil {
		l.sugar = l.z.Sugar()
	}
	return l.sugar
}

func (l *stderrLogger) Debug(args ...any)            { l.lazy().Debug(args...) }
func (l *stderrLogger) DebugF(f string, args ...any) { l.lazy().Debugf(f, args...) }
func (l *stderrLogger) DebugW(msg string, kv ...any) { l.lazy().Debugw(msg, kv...) }
func (l *stderrLogger) Info(args ...any)             { l.lazy().Info(args...) }
func (l *stderrLogger) InfoF(f string, args ...any)  { l.lazy().Infof(f, args...) }
func (l *stderrLogger) InfoW(msg string, kv ...any)  { l.lazy().Infow(msg, kv...) }
func (l *stderrLogger) Warn(args ...any)             { l.lazy().Warn(args...) }
func (l *stderrLogger) WarnF(f string, args ...any)  { l.lazy().Warnf(f, args...) }
func (l *stderrLogger) WarnW(msg string, kv ...any)  { l.lazy().Warnw(msg, kv...) }
func (l *stderrLogger) Error(args ...any)            { l.lazy().Error(args...) }
func (l *stderrLogger) ErrorF(f string, args ...any) { l.lazy().Errorf(f, args...) }
func (l *stderrLogger) ErrorW(msg string, kv ...any) { l.lazy().Errorw(msg, kv...) }
func (l *stderrLogger) DPanic(args ...any)           { l.lazy().DPanic(args...) }
func (l *stderrLogger) DPanicF(f string, args ...any) {
	l.lazy().DPanicf(f, args...)
}
func (l *stderrLogger) DPanicW(msg string, kv ...any) { l.lazy().DPanicw(msg, kv...) }
func (l *stderrLogger) Panic(args ...any)             { l.lazy().Panic(args...) }
func (l *stderrLogger) PanicF(f string, args ...any)  { l.lazy().Panicf(f, args...) }
func (l *stderrLogger) PanicW(msg string, kv ...any)  { l.lazy().Panicw(msg, kv...) }
func (l *stderrLogger) Fatal(args ...any)             { l.lazy().Fatal(args...) }
func (l *stderrLogger) FatalF(f string, args ...any)  { l.lazy().Fatalf(f, args...) }
func (l *stderrLogger) FatalW(msg string, kv ...any)  { l.lazy().Fatalw(msg, kv...) }

func (l *stderrLogger) NewCore(...zapper.Core) error    { return nil }
func (l *stderrLogger) GetServiceCode() uint            { return 0 }
func (l *stderrLogger) GetServiceName() string          { return "" }
func (l *stderrLogger) GetZap() *zap.Logger             { return l.z }
func (l *stderrLogger) GetSentryClient() *sentry.Client { return nil }
