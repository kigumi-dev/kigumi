package llgen

import "kigumi/internal/backend/llgen/runtimegen"

// RuntimeFile is one C source or header of the runtime.
type RuntimeFile = runtimegen.RuntimeFile

// RuntimeFiles includes rt_signal_hosted.c only for hosted POSIX OSes, which alone have <signal.h>.
func RuntimeFiles(sys, os string) []RuntimeFile { return runtimegen.Files(sys, os) }

// RuntimeZigFiles are the runtime's Zig translation units, compiled as one
// object by `zig build-obj` rather than handed to the C compiler.
func RuntimeZigFiles() []RuntimeFile { return runtimegen.ZigFiles() }

// RuntimeSource is the C runtime as one text, for tests that scan it.
func RuntimeSource() string { return runtimegen.Source() }
