package observability

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestNewLogger_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger("engine", &buf, FormatJSON, slog.LevelDebug)
	logger.Info("hello", slog.String("workflow_id", "wf-1"))

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, buf.String())
	}
	if got := line["component"]; got != "engine" {
		t.Errorf("component = %v, want engine", got)
	}
	if got := line["workflow_id"]; got != "wf-1" {
		t.Errorf("workflow_id = %v, want wf-1", got)
	}
	if got := line["msg"]; got != "hello" {
		t.Errorf("msg = %v, want hello", got)
	}
}

func TestNewLogger_TextFormatCarriesComponent(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger("worker", &buf, FormatText, slog.LevelInfo)
	logger.Info("ok")

	if !strings.Contains(buf.String(), "component=worker") {
		t.Errorf("missing component attr in text output: %q", buf.String())
	}
}

func TestLevelFromEnv_Default(t *testing.T) {
	t.Setenv("SCHED_LOG_LEVEL", "")
	if lvl := levelFromEnv(); lvl != slog.LevelInfo {
		t.Errorf("default level = %v, want INFO", lvl)
	}
	t.Setenv("SCHED_LOG_LEVEL", "debug")
	if lvl := levelFromEnv(); lvl != slog.LevelDebug {
		t.Errorf("debug level = %v, want DEBUG", lvl)
	}
}
