package log

import (
	"bytes"
	"log/slog"
	"testing"
)

func TestLogLevelInit(t *testing.T) {
	if l := logLevel(); l < slog.LevelInfo {
		t.Errorf("logLevel() = %d, should be >= %d", l, slog.LevelInfo)
	}
}

func TestLogLevelFiltering(t *testing.T) {
	oldLevel := logLevel()
	oldHandler := slog.Default().Handler()
	defer func() {
		levelVar.Set(oldLevel)
		slog.SetDefault(slog.New(oldHandler))
	}()

	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: levelVar})))

	SetLevel(LevelError)

	slog.Info("test_event")

	if buf.Len() != 0 {
		t.Error("expected no log output when level is Error")
	}
}

func TestLogLevelNames(t *testing.T) {
	levels := map[string]slog.Level{
		"error": slog.LevelError,
		"info":  slog.LevelInfo,
		"debug": slog.LevelDebug,
		"trace": levelTrace,
	}
	for name, expected := range levels {
		if v, ok := levelNames[name]; !ok || v != expected {
			t.Errorf("levelNames[%q] = %d, want %d", name, v, expected)
		}
	}
}

func TestLogJSONBelowLevel(t *testing.T) {
	oldLevel := logLevel()
	oldHandler := slog.Default().Handler()
	defer func() {
		levelVar.Set(oldLevel)
		slog.SetDefault(slog.New(oldHandler))
	}()

	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: levelVar})))

	SetLevel(LevelError)

	slog.Debug("should_not_appear")

	if buf.Len() != 0 {
		t.Error("expected no log output when level is Error")
	}
}

func TestLogInfo(t *testing.T) {
	Info("test_info_event")
}

func TestLogError(t *testing.T) {
	Error("test_error_event", nil)
}

func TestS3Call(t *testing.T) {
	S3Call("PutObject", "bucket", "key", 0, nil)
}
