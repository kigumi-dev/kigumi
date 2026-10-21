package sem

import "strings"

var libcSymbols = symbolSet(`
	malloc calloc realloc free memcpy memmove memset memcmp memchr
	strlen strcmp strncmp strchr strrchr strstr strcpy strncpy strcat strdup
	printf fprintf sprintf snprintf puts putchar getchar fopen fclose fread fwrite fflush
	read write open close lseek exit abort atexit getenv setenv atoi atol strtol
	abs labs qsort bsearch rand srand time clock sleep usleep signal raise
	sqrt pow floor ceil round trunc fabs fmod sin cos tan exp log
`)

// The compiler emits calls to these itself, so a bare target has to provide them.
var compilerCalledSymbols = symbolSet(`memcpy memmove memset memcmp`)

func LibcSymbols() map[string]bool { return libcSymbols }

func symbolSet(words string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(words) {
		out[w] = true
	}
	return out
}
