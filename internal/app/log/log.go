package log

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

const (
	LevelError = iota
	LevelWarn
	LevelInfo
	LevelDebug
	LevelTrace
)

const levelTrace = slog.LevelDebug - 4

var levelNames = map[string]slog.Level{
	"error": slog.LevelError,
	"warn":  slog.LevelWarn,
	"info":  slog.LevelInfo,
	"debug": slog.LevelDebug,
	"trace": levelTrace,
}

var levelVar = new(slog.LevelVar)

func init() {
	lvl := os.Getenv("LOG_LEVEL")
	if lvl == "" {
		lvl = "info"
	}
	level := slog.LevelInfo
	if v, ok := levelNames[strings.ToLower(lvl)]; ok {
		level = v
	}
	levelVar.Set(level)
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: levelVar})))
}

type Event struct {
	Event            string
	Method           string
	Path             string
	Status           int
	DurationMs       int64
	Error            error
	Message          string
	DeveloperMessage string
	Op               string
	Bucket           string
	Key              string
	Volume           string
	Snapshot         string
}

func LogEvent(level int, e Event) {
	var slogLevel slog.Level
	switch level {
	case LevelError:
		slogLevel = slog.LevelError
	case LevelWarn:
		slogLevel = slog.LevelWarn
	case LevelInfo:
		slogLevel = slog.LevelInfo
	case LevelDebug:
		slogLevel = slog.LevelDebug
	case LevelTrace:
		slogLevel = levelTrace
	default:
		slogLevel = slog.LevelInfo
	}
	attrs := make([]slog.Attr, 0, 12)
	if e.Method != "" {
		attrs = append(attrs, slog.String("method", e.Method))
	}
	if e.Path != "" {
		attrs = append(attrs, slog.String("path", e.Path))
	}
	if e.Status != 0 {
		attrs = append(attrs, slog.Int("status", e.Status))
	}
	if e.DurationMs != 0 {
		attrs = append(attrs, slog.Int64("duration_ms", e.DurationMs))
	}
	if e.Error != nil {
		attrs = append(attrs, slog.Any("error", e.Error))
	}
	if e.Message != "" {
		attrs = append(attrs, slog.String("message", e.Message))
	}
	if e.DeveloperMessage != "" {
		attrs = append(attrs, slog.String("developer_message", e.DeveloperMessage))
	}
	if e.Op != "" {
		attrs = append(attrs, slog.String("op", e.Op))
	}
	if e.Bucket != "" {
		attrs = append(attrs, slog.String("bucket", e.Bucket))
	}
	if e.Key != "" {
		attrs = append(attrs, slog.String("key", e.Key))
	}
	if e.Volume != "" {
		attrs = append(attrs, slog.String("volume", e.Volume))
	}
	if e.Snapshot != "" {
		attrs = append(attrs, slog.String("snapshot", e.Snapshot))
	}
	slog.LogAttrs(context.Background(), slogLevel, e.Event, attrs...)
}

func logLevel() slog.Level {
	return levelVar.Level()
}

// Derives restic CLI verbosity from app log level
func Verbosity() int {
	switch l := levelVar.Level(); {
	case l <= levelTrace:
		return 2
	case l <= slog.LevelDebug:
		return 1
	default:
		return 0
	}
}

func SetLevel(level int) {
	var slogLevel slog.Level
	switch level {
	case LevelError:
		slogLevel = slog.LevelError
	case LevelWarn:
		slogLevel = slog.LevelWarn
	case LevelInfo:
		slogLevel = slog.LevelInfo
	case LevelDebug:
		slogLevel = slog.LevelDebug
	case LevelTrace:
		slogLevel = levelTrace
	default:
		slogLevel = slog.LevelInfo
	}
	levelVar.Set(slogLevel)
}

func Info(event string) {
	slog.Info(event)
}

func Debug(event string) {
	slog.Debug(event)
}

func Warn(event string) {
	slog.Warn(event)
}

func Error(event string, err error) {
	if err != nil {
		slog.Error(event, "error", err)
	} else {
		slog.Error(event)
	}
}

func Infof(event string, format string, args ...any) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	slog.Info(event, "message", msg)
}

func Debugf(event string, format string, args ...any) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	slog.Debug(event, "message", msg)
}

func Warnf(event string, format string, args ...any) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	slog.Warn(event, "message", msg)
}

func Errorf(event string, err error, format string, args ...any) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	slog.Error(event, "message", msg, "error", err)
}

func InfoDev(event string, devMsg string) {
	slog.Info(event, "developer_message", devMsg)
}

func DebugDev(event string, devMsg string) {
	slog.Debug(event, "developer_message", devMsg)
}

func WarnDev(event string, devMsg string) {
	slog.Warn(event, "developer_message", devMsg)
}

func ErrorDev(event string, err error, devMsg string) {
	if err != nil {
		slog.Error(event, "error", err, "developer_message", devMsg)
	} else {
		slog.Error(event, "developer_message", devMsg)
	}
}

func InfofDev(event string, devMsg string, format string, args ...any) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	slog.Info(event, "message", msg, "developer_message", devMsg)
}

func DebugfDev(event string, devMsg string, format string, args ...any) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	slog.Debug(event, "message", msg, "developer_message", devMsg)
}

func WarnfDev(event string, devMsg string, format string, args ...any) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	slog.Warn(event, "message", msg, "developer_message", devMsg)
}

func ErrorfDev(event string, err error, devMsg string, format string, args ...any) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	slog.Error(event, "message", msg, "error", err, "developer_message", devMsg)
}

func S3Call(op, bucket, key string, dur time.Duration, err error) {
	attrs := []any{"op", op, "bucket", bucket, "key", key, "duration_ms", dur.Milliseconds()}
	if err != nil {
		attrs = append(attrs, "error", err)
	}
	slog.Debug("s3_call", attrs...)
}
