// Generated from internal/mir/stdops.reg by TestStdOpHeaderFresh; do not edit.
#ifndef KIGUMI_RT_STD_OPS_H
#define KIGUMI_RT_STD_OPS_H
enum std_op {
    OP_LEN = 0,
    OP_STRING_TO_BYTES = 1,
    OP_STRING_FROM_BYTES = 2,
    OP_STRING_TRY_FROM_BYTES = 3,
    OP_EQUALS = 4,
    OP_COMPARE_TO = 5,
    OP_HASH = 6,
    OP_DISPLAY = 7,
    OP_STRING_BYTES = 8,
    OP_BYTES_SLICE = 9,
    OP_BYTES_ZEROS = 10,
    OP_BYTES_FILL = 11,
    OP_BYTES_FROM_ARRAY = 12,
    OP_BYTES_TRY_FROM_ARRAY = 13,
    OP_BYTES_CONCAT = 14,
    OP_BYTES_TRY_CONCAT = 15,
    OP_ARRAY_EMPTY = 16,
    OP_ARRAY_OF = 17,
    OP_ARRAY_PUSH = 18,
    OP_ARRAY_GROW_BY = 19,
    OP_ARRAY_LEN = 20,
    OP_ARRAY_GET = 21,
    OP_ARRAY_WITH = 22,
    OP_ARRAY_WITH_MUT = 23,
    OP_ARRAY_TAKE_AT = 24,
    OP_OPTION_TAKE = 25,
    OP_TEXT_PARSE_FLOAT = 26,
    OP_HOST_SHELL = 27,
    OP_HOST_ARGS = 28,
    OP_HOST_FILES = 29,
    OP_HOST_NET = 30,
    OP_HOST_DL = 31,
    OP_HOST_ENTROPY = 32,
    OP_HOST_SIGNALS = 33,
    OP_ARGS_GET = 34,
    OP_CSTRING_NEW = 35,
    OP_CSTRING_PTR = 36,
    OP_CSTRING_DROP = 37,
    OP_FFI_BYTES_FROM = 38,
    OP_FFI_BYTES_PTR = 39,
    OP_BYTES_GET = 40,
    OP_SUBTLE_CT_EQ = 41,
    OP_DL_SYMBOL_CALL = 42,
    OP_HOST_STDIN = 43,
    OP_HOST_STDOUT = 44,
    OP_PIN_NEW = 45,
    OP_PIN_PTR = 46,
    OP_PIN_WITH = 47,
    OP_PIN_DROP = 48,
    OP_PIN_RELEASE = 49,
    OP_PIN_RECLAIM = 50,
    OP_FFI_STRING = 51,
    OP_FFI_STRING_CHECKED = 52,
    OP_TASK_CURRENT_EXECUTOR = 53,
    OP_EXECUTOR_RUN_TO_COMPLETION = 54,
    OP_TASK_SPAWN = 55,
    OP_TASK_CANCEL = 56,
    OP_ARENA_CREATE = 57,
    OP_ALLOCATOR_SCOPE = 58,
    OP_ALLOCATOR_ALLOCATED = 59,
    OP_SHARED_NEW = 60,
    OP_SHARED_CLONE = 61,
    OP_SHARED_GET = 62,
    OP_SHARED_SET = 63,
    OP_SHARED_WITH = 64,
    OP_SHARED_WITH_MUT = 65,
    OP_SHARED_DOWNGRADE = 66,
    OP_WEAK_UPGRADE = 67,
    OP_NET_HTTP = 68,
    OP_NET_CONN_DROP = 69,
    OP_NET_LISTENER_DROP = 70,
    OP_BUILD_ONLY = 71,
    OP_ARGS_LEN = 72,
    OP_MATH_SQRT = 73,
    OP_MATH_FLOOR = 74,
    OP_MATH_CEIL = 75,
    OP_MATH_ROUND = 76,
    OP_MATH_POW = 77,
    OP_STRING_SLICE_BYTES = 78,
    OP_ARRAY_CLONE = 79,
    OP_WRAPPING_ADD = 80,
    OP_WRAPPING_SUB = 81,
    OP_WRAPPING_MUL = 82,
    OP_WRAPPING_SHL = 83,
    OP_WRAPPING_SHR = 84,
    OP_ROTATE_LEFT = 85,
    OP_ROTATE_RIGHT = 86,
    OP_CHAR_FROM_INT = 87,
    OP_FFI_STRING_LOSSY = 88,
    OP_UCELL_NEW = 89,
    OP_UCELL_GET = 90,
    OP_UCELL_SET = 91,
    OP_HOST_STDERR = 92,
    OP_BYTES_TRY_ZEROS = 93,
    OP_BYTES_TRY_FILL = 94,
    OP_SHARED_TRY_NEW = 95,
    OP_BLOCKING_POOL_NEW = 96,
    OP_BLOCKING_POOL_RUN = 97,
    OP_HANDLE_TABLE_NEW = 98,
    OP_HANDLE_TABLE_INSERT = 99,
    OP_HANDLE_TABLE_WITH = 100,
    OP_HANDLE_TABLE_REMOVE = 101,
    OP_HANDLE_TABLE_RECLAIM = 102,
    OP_HANDLE_TABLE_CLOSE = 103,
    OP_HANDLE_TABLE_DROP = 104,
    OP_HANDLE_RAW = 105,
    OP_HANDLE_DROP = 106,
    OP_WITH_ADDRESS = 107,
    OP_TASK_CANCEL_REQUESTED = 108,
    OP_SECURE_ERASE = 109,
    OP_SECURE_ERASE_RAW = 110,
    OP_SECURE_ERASE_TRY = 111,
    OP_TASK_THREAD_POOL_EXECUTOR = 112,
    OP_TASK_HANDOFF = 113,
    OP_ARRAY_FIRST = 114,
    OP_ARRAY_LAST = 115,
    OP_ARRAY_TRY_WITH_CAPACITY = 116,
    OP_ARRAY_TRY_COPY_RANGE = 117,
    OP_ARRAY_TRY_SLICE = 118,
    OP_ARRAY_TRY_REVERSE = 119,
    OP_ALLOC_WRAP = 120,
    OP_STRING_TRY_CONCAT = 121,
    OP_STRING_TRY_SLICE_BYTES = 122,
    OP_BYTES_TRY_SLICE = 123,
    OP_MAP_TRY_CLONE = 124,
    OP_MAP_TRY_ENTRIES = 125,
    OP_MAP_TRY_EMPTY = 126,
    OP_STRING_BUILDER_TRY_NEW = 127,
    OP_ARRAY_TRY_MAP = 128,
    OP_ARRAY_TRY_FILTER = 129,
    OP_ARRAY_TRY_CONCAT = 130,
    OP_ARRAY_TRY_APPEND_ALL = 131,
    OP_ARRAY_TRY_INSERT_AT = 132,
    OP_SHARED_TRY_GET = 133,
    OP_SHARED_TRY_DOWNGRADE = 134,
    OP_ARRAY_TRY_CLONE = 135,
    OP_BOX_NEW = 136,
    OP_BOX_TRY_NEW = 137,
    OP_BOX_GET = 138,
    OP_BOX_SET = 139,
    OP_BOX_WITH = 140,
    OP_BOX_WITH_MUT = 141,
    OP_BOX_INTO = 142,
    OP_STRING_TRY_TRIM = 143,
    OP_STRING_TRY_TO_UPPER = 144,
    OP_STRING_TRY_TO_LOWER = 145,
    OP_STRING_TRY_CHARS = 146,
    OP_STRING_TRY_SPLIT = 147,
    OP_TEXT_TRY_LINES = 148,
    OP_TEXT_TRY_GRAPHEMES = 149,
    OP_TEXT_TRY_REPLACE = 150,
    OP_TEXT_TRY_TRIM_PREFIX = 151,
    OP_TEXT_TRY_TRIM_SUFFIX = 152,
    OP_ARRAY_TRY_JOIN = 153,
    OP_ARRAY_TRY_SORTED_BY = 154,
    OP_STRING_BUILDER_TRY_PUSH = 155,
    OP_DEBUG_TRY = 156,
    OP_STRING_BUILDER_TRY_PUSH_BYTE = 157,
    OP_ARENA_TRY_CREATE = 158,
    OP_STRING_BUILDER_TRY_FREEZE = 159,
    OP_ARGS_GET_BYTES = 160,
    OP_OS_STRING_TRY_CLONE = 161,
    OP_TASK_YIELD_NOW = 162,
    OP_MAP_SEED = 163,
    OP_BYTES_INDEX_OF = 164,
    OP_STRING_CONTAINS = 165,
    OP_BYTES_LAST_INDEX_OF = 166,
};
static int rt_std_op_of(const char* key) {
    const char* k = key;
    if (!strncmp(k, "std/", 4)) k += 4;
    if (!strcmp(k, "prelude.Bytes.len") || !strcmp(k, "prelude.String.len")) return OP_LEN;
    if (!strcmp(k, "prelude.String.toBytes")) return OP_STRING_TO_BYTES;
    if (!strcmp(k, "prelude.String.fromBytes")) return OP_STRING_FROM_BYTES;
    if (!strcmp(k, "prelude.String.tryFromBytes")) return OP_STRING_TRY_FROM_BYTES;
    if (!strcmp(k, "prelude.debug") || !strcmp(k, "text.fromChar")) return OP_DISPLAY;
    if (!strcmp(k, "prelude.String.bytes")) return OP_STRING_BYTES;
    if (!strcmp(k, "prelude.Bytes.slice")) return OP_BYTES_SLICE;
    if (!strcmp(k, "prelude.Bytes.zeros")) return OP_BYTES_ZEROS;
    if (!strcmp(k, "prelude.Bytes.fill")) return OP_BYTES_FILL;
    if (!strcmp(k, "prelude.Bytes.fromArray")) return OP_BYTES_FROM_ARRAY;
    if (!strcmp(k, "prelude.Bytes.tryFromArray")) return OP_BYTES_TRY_FROM_ARRAY;
    if (!strcmp(k, "prelude.Bytes.concat")) return OP_BYTES_CONCAT;
    if (!strcmp(k, "prelude.Bytes.tryConcat")) return OP_BYTES_TRY_CONCAT;
    if (!strcmp(k, "array.Array.empty")) return OP_ARRAY_EMPTY;
    if (!strcmp(k, "array.Array.of") || !strcmp(k, "prelude.FixedArray.adopt")) return OP_ARRAY_OF;
    if (!strcmp(k, "array.Array.push")) return OP_ARRAY_PUSH;
    if (!strcmp(k, "array.Array.growBy")) return OP_ARRAY_GROW_BY;
    if (!strcmp(k, "array.Array.len")) return OP_ARRAY_LEN;
    if (!strcmp(k, "array.Array.get")) return OP_ARRAY_GET;
    if (!strcmp(k, "array.Array.with")) return OP_ARRAY_WITH;
    if (!strcmp(k, "array.Array.withMut") || !strcmp(k, "prelude.FixedArray.withMut")) return OP_ARRAY_WITH_MUT;
    if (!strcmp(k, "array.Array.takeAt")) return OP_ARRAY_TAKE_AT;
    if (!strcmp(k, "prelude.Option.take")) return OP_OPTION_TAKE;
    if (!strcmp(k, "text.parseFloat")) return OP_TEXT_PARSE_FLOAT;
    if (!strcmp(k, "os.Host.shell")) return OP_HOST_SHELL;
    if (!strcmp(k, "os.Host.args")) return OP_HOST_ARGS;
    if (!strcmp(k, "os.Host.files")) return OP_HOST_FILES;
    if (!strcmp(k, "os.Host.net")) return OP_HOST_NET;
    if (!strcmp(k, "os.Host.dl")) return OP_HOST_DL;
    if (!strcmp(k, "os.Host.entropy")) return OP_HOST_ENTROPY;
    if (!strcmp(k, "os.Host.signals")) return OP_HOST_SIGNALS;
    if (!strcmp(k, "os.Args.get")) return OP_ARGS_GET;
    if (!strcmp(k, "ffi.CString.new")) return OP_CSTRING_NEW;
    if (!strcmp(k, "ffi.CString.ptr")) return OP_CSTRING_PTR;
    if (!strcmp(k, "ffi.CString.drop")) return OP_CSTRING_DROP;
    if (!strcmp(k, "ffi.bytesFrom")) return OP_FFI_BYTES_FROM;
    if (!strcmp(k, "ffi.bytesPtr")) return OP_FFI_BYTES_PTR;
    if (!strcmp(k, "prelude.Bytes.get")) return OP_BYTES_GET;
    if (!strcmp(k, "crypto/subtle.constantTimeEq")) return OP_SUBTLE_CT_EQ;
    if (!strcmp(k, "dl.Symbol.call")) return OP_DL_SYMBOL_CALL;
    if (!strcmp(k, "os.Host.stdin")) return OP_HOST_STDIN;
    if (!strcmp(k, "os.Host.stdout")) return OP_HOST_STDOUT;
    if (!strcmp(k, "ffi.Pin.new")) return OP_PIN_NEW;
    if (!strcmp(k, "ffi.Pin.ptr")) return OP_PIN_PTR;
    if (!strcmp(k, "ffi.Pin.with")) return OP_PIN_WITH;
    if (!strcmp(k, "ffi.Pin.drop")) return OP_PIN_DROP;
    if (!strcmp(k, "ffi.Pin.release")) return OP_PIN_RELEASE;
    if (!strcmp(k, "ffi.Pin.reclaim")) return OP_PIN_RECLAIM;
    if (!strcmp(k, "ffi.string")) return OP_FFI_STRING;
    if (!strcmp(k, "ffi.stringChecked")) return OP_FFI_STRING_CHECKED;
    if (!strcmp(k, "task.currentThreadExecutor")) return OP_TASK_CURRENT_EXECUTOR;
    if (!strcmp(k, "task.Executor.runToCompletion")) return OP_EXECUTOR_RUN_TO_COMPLETION;
    if (!strcmp(k, "task.spawn")) return OP_TASK_SPAWN;
    if (!strcmp(k, "task.Task.cancel")) return OP_TASK_CANCEL;
    if (!strcmp(k, "alloc.Arena.create")) return OP_ARENA_CREATE;
    if (!strcmp(k, "alloc.AllocatorHandle.scope")) return OP_ALLOCATOR_SCOPE;
    if (!strcmp(k, "alloc.AllocatorHandle.allocated")) return OP_ALLOCATOR_ALLOCATED;
    if (!strcmp(k, "alloc.Shared.new")) return OP_SHARED_NEW;
    if (!strcmp(k, "alloc.Shared.clone") || !strcmp(k, "alloc.Weak.clone")) return OP_SHARED_CLONE;
    if (!strcmp(k, "alloc.Shared.get")) return OP_SHARED_GET;
    if (!strcmp(k, "alloc.Shared.set")) return OP_SHARED_SET;
    if (!strcmp(k, "alloc.Shared.with")) return OP_SHARED_WITH;
    if (!strcmp(k, "alloc.Shared.withMut")) return OP_SHARED_WITH_MUT;
    if (!strcmp(k, "alloc.Shared.downgrade")) return OP_SHARED_DOWNGRADE;
    if (!strcmp(k, "alloc.Weak.upgrade")) return OP_WEAK_UPGRADE;
    if (!strcmp(k, "net.Net.http")) return OP_NET_HTTP;
    if (!strcmp(k, "net.Conn.drop")) return OP_NET_CONN_DROP;
    if (!strcmp(k, "net.Listener.drop")) return OP_NET_LISTENER_DROP;
    if (!strcmp(k, "os.Args.len")) return OP_ARGS_LEN;
    if (!strcmp(k, "math.sqrt")) return OP_MATH_SQRT;
    if (!strcmp(k, "math.floor")) return OP_MATH_FLOOR;
    if (!strcmp(k, "math.ceil")) return OP_MATH_CEIL;
    if (!strcmp(k, "math.round")) return OP_MATH_ROUND;
    if (!strcmp(k, "math.pow")) return OP_MATH_POW;
    if (!strcmp(k, "prelude.String.sliceBytes")) return OP_STRING_SLICE_BYTES;
    if (!strcmp(k, "array.Array.clone")) return OP_ARRAY_CLONE;
    if (!strcmp(k, "prelude.Char.fromInt")) return OP_CHAR_FROM_INT;
    if (!strcmp(k, "ffi.stringLossy")) return OP_FFI_STRING_LOSSY;
    if (!strcmp(k, "mem.UnsafeCell.new")) return OP_UCELL_NEW;
    if (!strcmp(k, "mem.UnsafeCell.get")) return OP_UCELL_GET;
    if (!strcmp(k, "mem.UnsafeCell.set")) return OP_UCELL_SET;
    if (!strcmp(k, "os.Host.stderr")) return OP_HOST_STDERR;
    if (!strcmp(k, "prelude.Bytes.tryZeros")) return OP_BYTES_TRY_ZEROS;
    if (!strcmp(k, "prelude.Bytes.tryFill")) return OP_BYTES_TRY_FILL;
    if (!strcmp(k, "alloc.Shared.tryNew")) return OP_SHARED_TRY_NEW;
    if (!strcmp(k, "task.BlockingPool.new")) return OP_BLOCKING_POOL_NEW;
    if (!strcmp(k, "task.BlockingPool.run")) return OP_BLOCKING_POOL_RUN;
    if (!strcmp(k, "ffi.HandleTable.new")) return OP_HANDLE_TABLE_NEW;
    if (!strcmp(k, "ffi.HandleTable.insert")) return OP_HANDLE_TABLE_INSERT;
    if (!strcmp(k, "ffi.HandleTable.with")) return OP_HANDLE_TABLE_WITH;
    if (!strcmp(k, "ffi.HandleTable.remove")) return OP_HANDLE_TABLE_REMOVE;
    if (!strcmp(k, "ffi.HandleTable.reclaim")) return OP_HANDLE_TABLE_RECLAIM;
    if (!strcmp(k, "ffi.HandleTable.close")) return OP_HANDLE_TABLE_CLOSE;
    if (!strcmp(k, "ffi.HandleTable.drop")) return OP_HANDLE_TABLE_DROP;
    if (!strcmp(k, "ffi.FfiHandle.raw")) return OP_HANDLE_RAW;
    if (!strcmp(k, "ffi.FfiHandle.drop")) return OP_HANDLE_DROP;
    if (!strcmp(k, "ffi.withAddress") || !strcmp(k, "ffi.withAddressConst")) return OP_WITH_ADDRESS;
    if (!strcmp(k, "task.cancelRequested")) return OP_TASK_CANCEL_REQUESTED;
    if (!strcmp(k, "mem.secureErase")) return OP_SECURE_ERASE;
    if (!strcmp(k, "ffi.secureEraseRaw")) return OP_SECURE_ERASE_RAW;
    if (!strcmp(k, "mem.trySecureErase")) return OP_SECURE_ERASE_TRY;
    if (!strcmp(k, "task.threadPoolExecutor")) return OP_TASK_THREAD_POOL_EXECUTOR;
    if (!strcmp(k, "task.Executor.handoff")) return OP_TASK_HANDOFF;
    if (!strcmp(k, "array.Array.first")) return OP_ARRAY_FIRST;
    if (!strcmp(k, "array.Array.last")) return OP_ARRAY_LAST;
    if (!strcmp(k, "array.Array.tryWithCapacity")) return OP_ARRAY_TRY_WITH_CAPACITY;
    if (!strcmp(k, "array.Array.tryCopyRange")) return OP_ARRAY_TRY_COPY_RANGE;
    if (!strcmp(k, "array.Array.trySlice")) return OP_ARRAY_TRY_SLICE;
    if (!strcmp(k, "array.Array.tryReverse")) return OP_ARRAY_TRY_REVERSE;
    if (!strcmp(k, "prelude.AllocError.wrap")) return OP_ALLOC_WRAP;
    if (!strcmp(k, "prelude.String.tryConcat")) return OP_STRING_TRY_CONCAT;
    if (!strcmp(k, "prelude.String.trySliceBytes")) return OP_STRING_TRY_SLICE_BYTES;
    if (!strcmp(k, "prelude.Bytes.trySlice")) return OP_BYTES_TRY_SLICE;
    if (!strcmp(k, "map.Map.tryClone")) return OP_MAP_TRY_CLONE;
    if (!strcmp(k, "map.Map.tryEntries")) return OP_MAP_TRY_ENTRIES;
    if (!strcmp(k, "map.tryEmpty")) return OP_MAP_TRY_EMPTY;
    if (!strcmp(k, "text.StringBuilder.tryNew")) return OP_STRING_BUILDER_TRY_NEW;
    if (!strcmp(k, "array.Array.tryMap")) return OP_ARRAY_TRY_MAP;
    if (!strcmp(k, "array.Array.tryFilter")) return OP_ARRAY_TRY_FILTER;
    if (!strcmp(k, "array.Array.tryConcat")) return OP_ARRAY_TRY_CONCAT;
    if (!strcmp(k, "array.Array.tryAppendAll")) return OP_ARRAY_TRY_APPEND_ALL;
    if (!strcmp(k, "array.Array.tryInsertAt")) return OP_ARRAY_TRY_INSERT_AT;
    if (!strcmp(k, "alloc.Shared.tryGet")) return OP_SHARED_TRY_GET;
    if (!strcmp(k, "alloc.Shared.tryDowngrade")) return OP_SHARED_TRY_DOWNGRADE;
    if (!strcmp(k, "array.Array.tryClone")) return OP_ARRAY_TRY_CLONE;
    if (!strcmp(k, "alloc.Box.new")) return OP_BOX_NEW;
    if (!strcmp(k, "alloc.Box.tryNew")) return OP_BOX_TRY_NEW;
    if (!strcmp(k, "alloc.Box.get")) return OP_BOX_GET;
    if (!strcmp(k, "alloc.Box.set")) return OP_BOX_SET;
    if (!strcmp(k, "alloc.Box.with")) return OP_BOX_WITH;
    if (!strcmp(k, "alloc.Box.withMut")) return OP_BOX_WITH_MUT;
    if (!strcmp(k, "alloc.Box.into")) return OP_BOX_INTO;
    if (!strcmp(k, "prelude.String.tryTrim")) return OP_STRING_TRY_TRIM;
    if (!strcmp(k, "prelude.String.tryToUpper")) return OP_STRING_TRY_TO_UPPER;
    if (!strcmp(k, "prelude.String.tryToLower")) return OP_STRING_TRY_TO_LOWER;
    if (!strcmp(k, "prelude.String.tryChars")) return OP_STRING_TRY_CHARS;
    if (!strcmp(k, "prelude.String.trySplit")) return OP_STRING_TRY_SPLIT;
    if (!strcmp(k, "text.tryLines")) return OP_TEXT_TRY_LINES;
    if (!strcmp(k, "text.tryGraphemes")) return OP_TEXT_TRY_GRAPHEMES;
    if (!strcmp(k, "text.tryReplace")) return OP_TEXT_TRY_REPLACE;
    if (!strcmp(k, "text.tryTrimPrefix")) return OP_TEXT_TRY_TRIM_PREFIX;
    if (!strcmp(k, "text.tryTrimSuffix")) return OP_TEXT_TRY_TRIM_SUFFIX;
    if (!strcmp(k, "array.tryJoin")) return OP_ARRAY_TRY_JOIN;
    if (!strcmp(k, "array.trySortedBy")) return OP_ARRAY_TRY_SORTED_BY;
    if (!strcmp(k, "text.StringBuilder.tryPush")) return OP_STRING_BUILDER_TRY_PUSH;
    if (!strcmp(k, "prelude.tryDebug")) return OP_DEBUG_TRY;
    if (!strcmp(k, "text.StringBuilder.tryPushByte")) return OP_STRING_BUILDER_TRY_PUSH_BYTE;
    if (!strcmp(k, "alloc.Arena.tryCreate")) return OP_ARENA_TRY_CREATE;
    if (!strcmp(k, "text.StringBuilder.tryFreeze")) return OP_STRING_BUILDER_TRY_FREEZE;
    if (!strcmp(k, "os.Args.getBytes")) return OP_ARGS_GET_BYTES;
    if (!strcmp(k, "os.OsString.tryClone")) return OP_OS_STRING_TRY_CLONE;
    if (!strcmp(k, "task.yieldNow")) return OP_TASK_YIELD_NOW;
    if (!strcmp(k, "map.seed")) return OP_MAP_SEED;
    if (!strcmp(k, "prelude.Bytes.indexOf")) return OP_BYTES_INDEX_OF;
    if (!strcmp(k, "prelude.String.contains")) return OP_STRING_CONTAINS;
    if (!strcmp(k, "prelude.Bytes.lastIndexOf")) return OP_BYTES_LAST_INDEX_OF;
    const char* dot = strrchr(k, '.');
    const char* m = dot ? dot + 1 : k;
    if (!strcmp(m, "equals")) return OP_EQUALS;
    if (!strcmp(m, "compareTo")) return OP_COMPARE_TO;
    if (!strcmp(m, "hash")) return OP_HASH;
    if (!strcmp(m, "wrappingAdd")) return OP_WRAPPING_ADD;
    if (!strcmp(m, "wrappingSub")) return OP_WRAPPING_SUB;
    if (!strcmp(m, "wrappingMul")) return OP_WRAPPING_MUL;
    if (!strcmp(m, "wrappingShiftLeft")) return OP_WRAPPING_SHL;
    if (!strcmp(m, "wrappingShiftRight")) return OP_WRAPPING_SHR;
    if (!strcmp(m, "rotateLeft")) return OP_ROTATE_LEFT;
    if (!strcmp(m, "rotateRight")) return OP_ROTATE_RIGHT;
    if (!strncmp(k, "build.", 6)) return OP_BUILD_ONLY;
    return -1;
}
#endif
