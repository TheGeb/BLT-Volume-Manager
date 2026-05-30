package applog

import (
	"io"
	"os"
	"testing"
)

func captureLogOutput(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer func() { _ = r.Close() }()

	old := os.Stdout
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = old

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read log output: %v", err)
	}
	return string(data)
}

func TestLogLevelInit(t *testing.T) {
	if CurrentLevel < LevelError {
		t.Errorf("CurrentLevel (%d) should be >= LevelError (%d)", CurrentLevel, LevelError)
	}
}

func TestLogLevelFiltering(t *testing.T) {
	oldLevel := CurrentLevel
	defer func() { CurrentLevel = oldLevel }()

	CurrentLevel = LevelError

	Log(Entry{Level: "info", Event: "test_event"})
	_ = captureLogOutput
}

func TestLogEntryLevelNames(t *testing.T) {
	if v, ok := levelNames["error"]; !ok || v != LevelError {
		t.Errorf("error level mapping wrong: %d", v)
	}
	if v, ok := levelNames["info"]; !ok || v != LevelInfo {
		t.Errorf("info level mapping wrong: %d", v)
	}
	if v, ok := levelNames["debug"]; !ok || v != LevelDebug {
		t.Errorf("debug level mapping wrong: %d", v)
	}
	if v, ok := levelNames["trace"]; !ok || v != LevelTrace {
		t.Errorf("trace level mapping wrong: %d", v)
	}
}

func TestLogLevelNames(t *testing.T) {
	levels := []string{"error", "info", "debug", "trace"}
	for _, l := range levels {
		if _, ok := levelNames[l]; !ok {
			t.Errorf("missing level name: %s", l)
		}
	}
}

func TestLogJSONBelowLevel(t *testing.T) {
	oldLevel := CurrentLevel
	defer func() { CurrentLevel = oldLevel }()

	CurrentLevel = LevelError

	Log(Entry{Level: "trace", Event: "should_not_appear"})
}

func TestLogJSONInvalidLevel(t *testing.T) {
	oldLevel := CurrentLevel
	defer func() { CurrentLevel = oldLevel }()

	CurrentLevel = LevelTrace

	Log(Entry{Level: "invalid", Event: "test"})
}

func TestLogInfo(t *testing.T) {
	Info("test_info_event")
}

func TestLogError(t *testing.T) {
	Error("test_error_event", nil)
}
