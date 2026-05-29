package logger

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is the platform-wide structured logging interface.
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Fatal(msg string, fields ...Field)
	With(fields ...Field) Logger
}

// Field is an alias for zap.Field so callers don't import zap directly.
type Field = zap.Field

// ── Field constructors ────────────────────────────────────────────────────────

func String(key, val string) Field        { return zap.String(key, val) }
func Int(key string, val int) Field       { return zap.Int(key, val) }
func Bool(key string, val bool) Field     { return zap.Bool(key, val) }
func Error(err error) Field               { return zap.Error(err) }
func Any(key string, val interface{}) Field { return zap.Any(key, val) }
func Duration(key string, val time.Duration) Field { return zap.Duration(key, val) }

// ── zapLogger implementation ──────────────────────────────────────────────────

type zapLogger struct{ z *zap.Logger }

// NewLogger creates a production-ready zap logger.
func NewLogger() Logger {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	z, err := cfg.Build(zap.AddCallerSkip(1))
	if err != nil {
		panic("failed to initialise logger: " + err.Error())
	}
	return &zapLogger{z: z}
}

// NewDevelopmentLogger creates a human-friendly logger for local development.
func NewDevelopmentLogger() Logger {
	z, err := zap.NewDevelopment(zap.AddCallerSkip(1))
	if err != nil {
		panic("failed to initialise dev logger: " + err.Error())
	}
	return &zapLogger{z: z}
}

func (l *zapLogger) Debug(msg string, fields ...Field) { l.z.Debug(msg, fields...) }
func (l *zapLogger) Info(msg string, fields ...Field)  { l.z.Info(msg, fields...) }
func (l *zapLogger) Warn(msg string, fields ...Field)  { l.z.Warn(msg, fields...) }
func (l *zapLogger) Error(msg string, fields ...Field) { l.z.Error(msg, fields...) }
func (l *zapLogger) Fatal(msg string, fields ...Field) { l.z.Fatal(msg, fields...) }

func (l *zapLogger) With(fields ...Field) Logger {
	return &zapLogger{z: l.z.With(fields...)}
}
