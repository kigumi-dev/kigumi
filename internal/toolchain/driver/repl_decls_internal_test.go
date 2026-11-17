package driver

import "testing"

func TestDeclChunkAtFollowsJoinedLines(t *testing.T) {
	chunks := []replDeclChunk{
		{src: "fn a() {\n}\n", origin: "prelude"},
		{src: "fn b() {}", origin: "cell 1"},
		{src: "fn c() {\n    1\n}", origin: "cell 2"},
	}
	joined := joinDeclChunks(chunks)
	for line, want := range map[int]string{1: "prelude", 3: "prelude", 4: "cell 1", 5: "cell 2", 7: "cell 2"} {
		c, ok := declChunkAt(chunks, line)
		if !ok || c.origin != want {
			t.Errorf("line %d: %q %v, want %q in\n%s", line, c.origin, ok, want, joined)
		}
	}
	if _, ok := declChunkAt(chunks, 8); ok {
		t.Error("line past the declarations matched a chunk")
	}
}
