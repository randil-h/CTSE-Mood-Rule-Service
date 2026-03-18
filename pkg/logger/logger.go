package logger

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type contextKey string

const traceIDKey contextKey = "trace_id"

var log *zap.Logger

// Init initializes the global logger
func Init(development bool) error {
	var cfg zap.Config
	if development {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		cfg = zap.NewProductionConfig()
		cfg.EncoderConfig.TimeKey = "timestamp"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	var err error
	log, err = cfg.Build(zap.AddCallerSkip(1))
	if err != nil {
		return err
	}

	return nil
}

// WithTraceID adds trace ID to context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// FromContext extracts trace ID from context
func FromContext(ctx context.Context) string {
	if traceID, ok := ctx.Value(traceIDKey).(string); ok {
		return traceID
	}
	return ""
}

// Debug logs a debug message
func Debug(ctx context.Context, msg string, fields ...zap.Field) {
	if traceID := FromContext(ctx); traceID != "" {
		fields = append(fields, zap.String("trace_id", traceID))
	}
	log.Debug(msg, fields...)
}

// Info logs an info message
func Info(ctx context.Context, msg string, fields ...zap.Field) {
	if traceID := FromContext(ctx); traceID != "" {
		fields = append(fields, zap.String("trace_id", traceID))
	}
	log.Info(msg, fields...)
}

// Warn logs a warning message
func Warn(ctx context.Context, msg string, fields ...zap.Field) {
	if traceID := FromContext(ctx); traceID != "" {
		fields = append(fields, zap.String("trace_id", traceID))
	}
	log.Warn(msg, fields...)
}

// Error logs an error message
func Error(ctx context.Context, msg string, fields ...zap.Field) {
	if traceID := FromContext(ctx); traceID != "" {
		fields = append(fields, zap.String("trace_id", traceID))
	}
	log.Error(msg, fields...)
}

// Fatal logs a fatal message and exits
func Fatal(ctx context.Context, msg string, fields ...zap.Field) {
	if traceID := FromContext(ctx); traceID != "" {
		fields = append(fields, zap.String("trace_id", traceID))
	}
	log.Fatal(msg, fields...)
}

// Sync flushes any buffered log entries
func Sync() error {
	if log != nil {
		return log.Sync()
	}
	return nil
}
