package log

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
)

func TestLogLevelInit(t *testing.T) {
	t.Parallel()
	if l := logLevel(); l < slog.LevelInfo {
		t.Errorf("logLevel() = %d, should be >= %d", l, slog.LevelInfo)
	}
}

func TestLogLevelFiltering(t *testing.T) {
	t.Parallel()
	oldLevel := logLevel()
	oldHandler := slog.Default().Handler()
	defer func() {
		levelVar.Set(oldLevel)
		slog.SetDefault(slog.New(oldHandler))
	}()

	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: levelVar})))

	SetLevel(LevelError)

	for _, level := range []slog.Level{slog.LevelInfo, slog.LevelDebug} {
		buf.Reset()
		slog.Log(context.Background(), level, "test_event")
		if buf.Len() != 0 {
			t.Errorf("expected no log output for level %v when configured level is Error", level)
		}
	}
}

func TestLogLevelNames(t *testing.T) {
	t.Parallel()
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
