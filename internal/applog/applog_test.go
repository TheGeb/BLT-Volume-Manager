package applog

import (
	"testing"
)

func TestLogLevelInit(t *testing.T) {
	if CurrentLevel() < LevelError {
		t.Errorf("CurrentLevel (%d) should be >= LevelError (%d)", CurrentLevel(), LevelError)
	}
}

func TestLogLevelFiltering(t *testing.T) {
	oldLevel := CurrentLevel()
	defer func() { SetLevel(oldLevel) }()

	SetLevel(LevelError)

	Log(Entry{Level: "info", Event: "test_event"})
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
	oldLevel := CurrentLevel()
	defer func() { SetLevel(oldLevel) }()

	SetLevel(LevelError)

	Log(Entry{Level: "trace", Event: "should_not_appear"})
}

func TestLogJSONInvalidLevel(t *testing.T) {
	oldLevel := CurrentLevel()
	defer func() { SetLevel(oldLevel) }()

	SetLevel(LevelTrace)

	Log(Entry{Level: "invalid", Event: "test"})
}

func TestLogInfo(t *testing.T) {
	Info("test_info_event")
}

func TestLogError(t *testing.T) {
	Error("test_error_event", nil)
}
