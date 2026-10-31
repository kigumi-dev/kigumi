package driver

import "strings"

var allocatorSymbols = map[string]bool{
	"malloc": true, "calloc": true, "realloc": true, "free": true,
	"posix_memalign": true, "aligned_alloc": true, "brk": true, "sbrk": true,
	"mmap": true, "munmap": true,
}

var stdioSymbols = map[string]bool{
	"printf": true, "fprintf": true, "sprintf": true, "snprintf": true,
	"vprintf": true, "vfprintf": true, "scanf": true, "sscanf": true,
	"fopen": true, "fclose": true, "fread": true, "fwrite": true,
	"fputs": true, "fgets": true, "fputc": true, "fgetc": true,
	"puts": true, "putchar": true, "getchar": true, "perror": true,
	"stdin": true, "stdout": true, "stderr": true,
}

// classifySymbol names the category an undefined symbol falls in, for a more specific
// audit message than "not in the allow-list"; "" means none of the known ones applies.
func classifySymbol(name string) string {
	switch {
	case allocatorSymbols[name]:
		return "allocator"
	case stdioSymbols[name]:
		return "stdio"
	case strings.HasPrefix(name, "pthread_") || strings.HasPrefix(name, "thrd_") ||
		strings.HasPrefix(name, "mtx_") || strings.HasPrefix(name, "cnd_") ||
		name == "__tls_get_addr" || name == "tls_get_addr" ||
		name == "_tls_index" || name == "_tls_used":
		return "thread"
	default:
		return ""
	}
}
