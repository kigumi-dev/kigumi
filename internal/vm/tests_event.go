package vm

import (
	"encoding/json"
	"io"
)

// TestEventFormatVersion versions the JSON lines RunTestsReporting emits, so a consumer within one version can ignore fields it doesn't know.
const TestEventFormatVersion = 1

// testEvent is one line of the test-runner event stream: a suiteStart, one testPass/testFail/testTimeout/testSkip per test, then a suiteEnd.
type testEvent struct {
	FormatVersion int    `json:"formatVersion"`
	Type          string `json:"type"`
	ID            string `json:"id,omitempty"`
	File          string `json:"file,omitempty"`
	Line          int    `json:"line,omitempty"`
	Message       string `json:"message,omitempty"`
	DurationMS    int64  `json:"durationMs,omitempty"`
	Total         int    `json:"total,omitempty"`
	Passed        int    `json:"passed,omitempty"`
	Failed        int    `json:"failed,omitempty"`
	Skipped       int    `json:"skipped,omitempty"`
}

// eventWriter emits testEvents as JSON lines when json is true, else emit is a no-op (plain mode prints its own lines).
type eventWriter struct {
	enc  *json.Encoder
	json bool
}

func newEventWriter(w io.Writer, jsonMode bool) *eventWriter {
	return &eventWriter{enc: json.NewEncoder(w), json: jsonMode}
}

func (w *eventWriter) emit(e testEvent) {
	if !w.json {
		return
	}
	e.FormatVersion = TestEventFormatVersion
	w.enc.Encode(e)
}
