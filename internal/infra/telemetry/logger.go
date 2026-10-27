package telemetry

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	otelmetric "go.opentelemetry.io/otel/metric"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/trace"
)

type Logger struct {
	out  *slog.Logger
	ls   log.Logger
	errs otelmetric.Int64Counter
}

func newLogger(level slog.Level, lp *sdklog.LoggerProvider, mp otelmetric.MeterProvider, serviceName string) *Logger {
	m := mp.Meter("tripgo.logger")
	counter, err := m.Int64Counter("tripgo.log.errors",
		otelmetric.WithDescription("Number of error-level log records"))
	if err != nil {
		counter = nil
	}

	return &Logger{
		out:  newStdoutLogger(level),
		ls:   lp.Logger(serviceName),
		errs: counter,
	}
}

func (l *Logger) StdLogger() *slog.Logger { return l.out }

func (l *Logger) Debug(msg string, args ...any) {
	l.emit(context.Background(), log.SeverityDebug, slog.LevelDebug, msg, args...)
}

func (l *Logger) Info(msg string, args ...any) {
	l.emit(context.Background(), log.SeverityInfo, slog.LevelInfo, msg, args...)
}

func (l *Logger) Warn(msg string, args ...any) {
	l.emit(context.Background(), log.SeverityWarn, slog.LevelWarn, msg, args...)
}

func (l *Logger) Error(msg string, args ...any) {
	if l.errs != nil {
		l.errs.Add(context.Background(), 1)
	}
	l.emit(context.Background(), log.SeverityError, slog.LevelError, msg, args...)
}

func (l *Logger) ErrorCtx(ctx context.Context, msg string, args ...any) {
	if l.errs != nil {
		l.errs.Add(ctx, 1)
	}
	l.emit(ctx, log.SeverityError, slog.LevelError, msg, args...)
}

func (l *Logger) InfoCtx(ctx context.Context, msg string, args ...any) {
	l.emit(ctx, log.SeverityInfo, slog.LevelInfo, msg, args...)
}

func (l *Logger) emit(ctx context.Context, sev log.Severity, level slog.Level, msg string, args ...any) {
	switch level {
	case slog.LevelDebug:
		l.out.DebugContext(ctx, msg, args...)
	case slog.LevelWarn:
		l.out.WarnContext(ctx, msg, args...)
	case slog.LevelError:
		l.out.ErrorContext(ctx, msg, args...)
	default:
		l.out.InfoContext(ctx, msg, args...)
	}

	var rec log.Record
	rec.SetTimestamp(timeNow())
	rec.SetObservedTimestamp(timeNow())
	rec.SetSeverity(sev)
	rec.SetSeverityText(level.String())
	rec.SetBody(attribute.StringValue(msg))
	rec.AddAttributes(append([]attribute.KeyValue{
		attribute.Int("slog_level_num", int(level)),
	}, attrsToKeyValues(args...)...)...)

	l.ls.Emit(ctx, rec)
}

func attrsToKeyValues(args ...any) []attribute.KeyValue {
	kvs := make([]attribute.KeyValue, 0, len(args)/2)
	for i := 0; i+1 < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok {
			continue
		}
		kvs = append(kvs, anyAttr(key, args[i+1]))
	}
	return kvs
}

func anyAttr(key string, v any) attribute.KeyValue {
	switch val := v.(type) {
	case bool:
		return attribute.Bool(key, val)
	case int:
		return attribute.Int(key, val)
	case int64:
		return attribute.Int64(key, val)
	case float64:
		return attribute.Float64(key, val)
	case string:
		return attribute.String(key, val)
	case error:
		return attribute.String(key, val.Error())
	case fmt.Stringer:
		return attribute.Stringer(key, val)
	default:
		return attribute.String(key, fmt.Sprintf("%v", val))
	}
}

var _ = trace.SpanContextFromContext
