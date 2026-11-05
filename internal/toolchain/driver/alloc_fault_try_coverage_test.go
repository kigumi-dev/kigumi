package driver_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/toolchain/contract/schema"
)

// oomCovers names, for every std try function, the testdata/oom fixture pair (name and name_twice) whose sweeps drive it.
var oomCovers = map[string]string{
	"array.Array.tryClone": "array_clone", "array.Array.tryCloneWith": "array_clone_with", "array.Array.tryFill": "array_fill",
	"array.Array.tryWithCapacity": "array_with_capacity", "array.Array.tryEmpty": "array_try_empty", "array.Array.tryCopyRange": "array_take", "array.Array.tryReverse": "array_reverse",
	"array.Array.tryPush": "array_push", "array.Array.tryReserve": "array_reserve", "array.Array.tryAppendAll": "array_append_all",
	"array.Array.tryInsertAt": "array_insert_at", "array.Array.tryTake": "array_take", "array.Array.tryMap": "array_map",
	"array.Array.tryFilter": "array_filter", "array.Array.trySlice": "array_slice", "array.Array.tryConcat": "array_concat",
	"array.tryJoin": "array_join", "array.trySortedBy": "array_sorted_by", "sort.tryStable": "sort_stable",
	"prelude.String.trySliceBytes": "string_slice", "prelude.String.tryTrim": "string_trim", "prelude.String.trySplit": "string_split",
	"prelude.String.tryConcat": "string_concat", "prelude.String.tryRepeat": "string_repeat", "prelude.String.tryToUpper": "string_to_upper",
	"prelude.String.tryToLower": "string_to_lower", "prelude.String.tryChars": "string_chars", "prelude.String.tryFromBytes": "string_from_bytes",
	"prelude.Bytes.tryFromArray": "bytes_from_array", "prelude.Bytes.trySlice": "bytes_slice", "prelude.Bytes.tryZeros": "bytes_zeros",
	"prelude.Bytes.tryFill": "bytes_fill", "prelude.Bytes.tryConcat": "bytes_concat", "prelude.ByteBuffer.tryNew": "byte_buffer_new",
	"prelude.tryDebug":     "debug",
	"os.OsString.tryClone": "os_string_try_clone", "os.OsString.tryToStringLossy": "os_string_try_to_string_lossy",
	"alloc.Arena.tryCreate": "arena_create", "alloc.Shared.tryNew": "shared_new", "alloc.Shared.tryGet": "shared_get",
	"alloc.Shared.tryDowngrade": "shared_downgrade", "alloc.Box.tryNew": "box_new",
	"map.Map.tryClone": "map_clone", "map.tryEmpty": "map_try_empty", "map.Map.trySet": "map_set_new_key",
	"map.Map.tryEntries": "map_entries", "map.Map.tryKeys": "map_keys", "map.Map.tryValues": "map_values",
	"text.StringBuilder.tryNew": "builder_new", "text.StringBuilder.tryPush": "builder_push", "text.StringBuilder.tryPushByte": "builder_push_byte",
	"text.StringBuilder.tryFreeze": "builder_freeze",
	"text.tryLines":                "text_lines", "text.tryScalars": "text_scalars", "text.tryReplace": "text_replace",
	"text.tryTrimPrefix": "text_trim_prefix", "text.tryTrimSuffix": "text_trim_suffix", "text.trySlice": "text_slice",
}

// oomInert are try functions with no allocation on their path, so no tick can be refused inside them.
var oomInert = map[string]string{
	"mem.trySecureErase":       "zeroes the region in place and builds no result of its own",
	"mem.BoundedArray.tryPush": "fills a slot new already allocated; it fails only on a full array",
}

func TestEveryTryFunctionHasATwoCallSweep(t *testing.T) {
	t.Parallel()
	res := sem.CheckWith(&sem.Module{Packages: semfixture.LoadStd(t, "../../../std")}, schema.GenerateDerive)
	var tries []string
	for id := range res.Entities {
		e := &res.Entities[id]
		if e.Kind != sem.EntFn || e.Flags&sem.EfStd == 0 || len(e.Name) < 4 || !strings.HasPrefix(e.Name, "try") || e.Name[3] < 'A' || e.Name[3] > 'Z' {
			continue
		}
		tries = append(tries, opdef.StdKey(res, sem.EntityID(id), true))
	}
	if len(tries) < 50 {
		t.Fatalf("found only %d try functions; std not loaded?", len(tries))
	}
	for _, key := range tries {
		fixture, covered := oomCovers[key]
		if _, inert := oomInert[key]; inert {
			continue
		}
		if !covered {
			t.Errorf("%s has no fault sweep fixture; add testdata/oom/<name>.kg and <name>_twice.kg and list it in oomCovers", key)
			continue
		}
		for _, name := range []string{fixture, fixture + "_twice"} {
			if _, err := os.Stat(filepath.Join(oomDir, name+".kg")); err != nil {
				t.Errorf("%s: fixture %s.kg missing", key, name)
			}
		}
	}
	for key := range oomCovers {
		if !slices.Contains(tries, key) {
			t.Errorf("oomCovers lists %s, which is no std try function", key)
		}
	}
}
