package lsp_test

import (
	"context"
	"strings"
	"testing"

	"kigumi/internal/tooling/lsp"
)

func TestInitializeRejectsEmptyCwd(t *testing.T) {
	_, err := lsp.New("../../../std").Initialize(context.Background(), initParams("file:///tmp/ws"))
	if err == nil || !strings.Contains(err.Error(), "Cwd") {
		t.Fatalf("Initialize without a Cwd = %v, want a Cwd error", err)
	}
}
