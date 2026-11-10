package vm

import (
	"encoding/json"
	"io"
)

// TestEventFormatVersion versions the JSON lines RunTestsReporting emits, so a consumer within one version can ignore fields it doesn't know.
const TestEventFormatVersion = 1

// Every event type has a fixed key set: a counter that is 0 and a duration that is 0ms are written as 0, never omitted,
// so a consumer can read the field instead of treating its absence as 0.
type suiteStartEvent struct {
	FormatVersion int    `json:"formatVersion"`
	Type          string `json:"type"`
	Total         int    `json:"total"`
}

// testOutputEvent is one chunk a test wrote, in the order it wrote it.
type testOutputEvent struct {
	FormatVersion int    `json:"formatVersion"`
	Type          string `json:"type"`
	ID            string `json:"id"`
	Stream        string `json:"stream"`
	Message       string `json:"message"`
}

// testCaseEvent is one test: testList, testPass, testFail, testTimeout or testSkip.
type testCaseEvent struct {
	FormatVersion int    `json:"formatVersion"`
	Type          string `json:"type"`
	ID            string `json:"id"`
	File          string `json:"file"`
	Line          int    `json:"line"`
	DurationMS    int64  `json:"durationMs"`
	Message       string `json:"message"`
}

type suiteEndEvent struct {
	FormatVersion int    `json:"formatVersion"`
	Type          string `json:"type"`
	Total         int    `json:"total"`
	Passed        int    `json:"passed"`
	Failed        int    `json:"failed"`
	Skipped       int    `json:"skipped"`
	DurationMS    int64  `json:"durationMs"`
}

// eventWriter emits events as JSON lines when json is true, else emit is a no-op (plain mode prints its own lines).
type eventWriter struct {
	enc  *json.Encoder
	json bool
}

func newEventWriter(w io.Writer, jsonMode bool) *eventWriter {
	return &eventWriter{enc: json.NewEncoder(w), json: jsonMode}
}

func (w *eventWriter) emit(e any) {
	if !w.json {
		return
	}
	w.enc.Encode(e)
}
