//go:build integration

package integration

import (
	"bufio"
	"bytes"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func setupLogCapture(t *testing.T) {
	t.Helper()

	var buf bytes.Buffer
	mw := io.MultiWriter(os.Stdout, &buf)
	old := slog.Default()

	inner := slog.NewJSONHandler(mw, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(inner))

	t.Cleanup(func() {
		slog.SetDefault(old)
		scanner := bufio.NewScanner(&buf)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, `"level":"ERROR"`) {
				t.Errorf("unexpected ERROR log:\n%s", line)
			}
		}
	})
}
