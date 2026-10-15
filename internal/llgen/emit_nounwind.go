package llgen

import "strings"

// Panics jump with setjmp; without nounwind an ARM EHABI target emits unwind entries naming __aeabi_unwind_cpp_pr0.
func markNoUnwind(ir string) string {
	lines := strings.Split(ir, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, "define ") && strings.HasSuffix(l, " {") {
			lines[i] = l[:len(l)-1] + "nounwind {"
		}
	}
	return strings.Join(lines, "\n")
}
