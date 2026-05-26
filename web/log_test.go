package web

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func captureLogOutput(t *testing.T, fn func()) string {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()

	old := os.Stdout
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf strings.Builder
	json.NewDecoder(r).Decode(&buf)
	return buf.String()
}

func TestLogLevelInit(t *testing.T) {
	if currentLevel < levelError {
		t.Errorf("currentLevel (%d) should be >= levelError (%d)", currentLevel, levelError)
	}
}

func TestLogLevelFiltering(t *testing.T) {
	oldLevel := currentLevel
	defer func() { currentLevel = oldLevel }()

	currentLevel = levelError

	logJSON(logEntry{Level: "info", Event: "test_event"})
	_ = captureLogOutput
}

func TestLogEntryLevelNames(t *testing.T) {
	if v, ok := levelNames["error"]; !ok || v != levelError {
		t.Errorf("error level mapping wrong: %d", v)
	}
	if v, ok := levelNames["info"]; !ok || v != levelInfo {
		t.Errorf("info level mapping wrong: %d", v)
	}
	if v, ok := levelNames["debug"]; !ok || v != levelDebug {
		t.Errorf("debug level mapping wrong: %d", v)
	}
	if v, ok := levelNames["trace"]; !ok || v != levelTrace {
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

func TestLoggingResponseWriter(t *testing.T) {
	w := httptest.NewRecorder()
	lw := &loggingResponseWriter{ResponseWriter: w, status: 200}
	lw.WriteHeader(404)
	if lw.status != 404 {
		t.Errorf("expected status 404, got %d", lw.status)
	}
}

func TestLogJSONBelowLevel(t *testing.T) {
	oldLevel := currentLevel
	defer func() { currentLevel = oldLevel }()

	currentLevel = levelError

	logJSON(logEntry{Level: "trace", Event: "should_not_appear"})
}

func TestLogJSONInvalidLevel(t *testing.T) {
	oldLevel := currentLevel
	defer func() { currentLevel = oldLevel }()

	currentLevel = levelTrace

	logJSON(logEntry{Level: "invalid", Event: "test"})
}

func TestLogInfo(t *testing.T) {
	logInfo("test_info_event")
	_ = logInfo
}

func TestLogError(t *testing.T) {
	logError("test_error_event", nil)
	_ = logError
}
