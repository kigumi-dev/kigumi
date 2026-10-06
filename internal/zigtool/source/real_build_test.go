package source_test

import "testing"

// TestBuildFromRealSource is skipped: a real cmake+LLVM/Clang/LLD 22.x build
// is past this machine's time/disk budget; orchestration is covered by TestBuildInstallsAndRecordsCommit and its neighbors.
func TestBuildFromRealSource(t *testing.T) {
	t.Skip("needs LLVM/Clang/LLD 22.x dev libraries not installed on this machine; see package comment")
}
