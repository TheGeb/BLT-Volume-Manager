package applog

import (
	"encoding/json"
	"fmt"
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

var levelNames = map[string]int{
	"error": LevelError,
	"warn":  LevelWarn,
	"warning": LevelWarn,
	"info":  LevelInfo,
	"debug": LevelDebug,
	"trace": LevelTrace,
}

var CurrentLevel int

func init() {
	lvl := os.Getenv("LOG_LEVEL")
	if lvl == "" {
		lvl = "info"
	}
	if v, ok := levelNames[strings.ToLower(lvl)]; ok {
		CurrentLevel = v
	} else {
		CurrentLevel = LevelInfo
	}
}

type Entry struct {
	Level      string `json:"level"`
	Event      string `json:"event"`
	Method     string `json:"method,omitempty"`
	Path       string `json:"path,omitempty"`
	Status     int    `json:"status,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	Error      string `json:"error,omitempty"`
	Message    string `json:"message,omitempty"`
	Op         string `json:"op,omitempty"`
	Bucket     string `json:"bucket,omitempty"`
	Key        string `json:"key,omitempty"`
	Volume     string `json:"volume,omitempty"`
	Snapshot   string `json:"snapshot,omitempty"`
	Time       string `json:"time"`
}

func Log(e Entry) {
	v, ok := levelNames[e.Level]
	if !ok || v > CurrentLevel {
		return
	}
	e.Time = time.Now().UTC().Format(time.RFC3339Nano)
	_ = json.NewEncoder(os.Stdout).Encode(e)
}

func Debug(event string) {
	Log(Entry{Level: "debug", Event: event})
}

func Info(event string) {
	Log(Entry{Level: "info", Event: event})
}

func Warn(event string) {
	Log(Entry{Level: "warn", Event: event})
}

func Error(event string, err error) {
	e := Entry{Level: "error", Event: event}
	if err != nil {
		e.Error = err.Error()
	}
	Log(e)
}

// Infof logs a formatted message at info level.
func Infof(event string, format string, args ...any) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	Log(Entry{Level: "info", Event: event, Message: msg})
}

// Debugf logs a formatted message at debug level.
func Debugf(event string, format string, args ...any) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	Log(Entry{Level: "debug", Event: event, Message: msg})
}

// Warnf logs a formatted message at warn level.
func Warnf(event string, format string, args ...any) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	Log(Entry{Level: "warn", Event: event, Message: msg})
}

// Errorf logs a formatted error message at error level.
func Errorf(event string, err error, format string, args ...any) {
	e := Entry{Level: "error", Event: event}
	if err != nil {
		e.Error = err.Error()
	}
	if format != "" {
		if len(args) > 0 {
			e.Message = fmt.Sprintf(format, args...)
		} else {
			e.Message = format
		}
	}
	Log(e)
}

func S3Call(op, bucket, key string, dur time.Duration, err error) {
	e := Entry{Level: "debug", Event: "s3_call", Op: op, Bucket: bucket, Key: key, DurationMs: dur.Milliseconds()}
	if err != nil {
		e.Error = err.Error()
	}
	Log(e)
}

func Printf(format string, args ...any) {
	Infof("log", format, args...)
}
