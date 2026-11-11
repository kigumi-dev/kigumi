//! C ABI surface of the native float runtime: shortest round-trip
//! formatting and correctly rounded decimal parsing, libc-free. Formatting
//! is Dragon4 (Steele & White); parsing is "Algorithm M" big-integer
//! division, so one bignum engine serves both.

const algorithm_m = @import("float/algorithm_m.zig");
const dragon4 = @import("float/dragon4.zig");

/// Safety panics trap: this object has no libc or stderr to report through,
/// and a broken invariant is deterministic either way. Overriding the panic
/// namespace keeps std's debug/stack-trace machinery (and its libc
/// references) out of a runtime object that must stay freestanding.
pub const panic = struct {
    pub fn call(msg: []const u8, ra: ?usize) noreturn {
        _ = msg;
        _ = ra;
        @trap();
    }
    pub fn sentinelMismatch(expected: anytype, found: @TypeOf(expected)) noreturn {
        // expected is used by the type of found.
        _ = found;
        call("sentinel mismatch", null);
    }
    pub fn unwrapError(err: anyerror) noreturn {
        _ = err;
        call("attempt to unwrap error", null);
    }
    pub fn unexpectedErrorCode(err: anyerror) noreturn {
        _ = err;
        @trap();
    }
    pub fn outOfBounds(index: usize, len: usize) noreturn {
        _ = index;
        _ = len;
        call("index out of bounds", null);
    }
    pub fn startGreaterThanEnd(start: usize, end: usize) noreturn {
        _ = start;
        _ = end;
        call("start index is larger than end index", null);
    }
    pub fn inactiveUnionField(active: anytype, accessed: @TypeOf(active)) noreturn {
        // active is used by the type of accessed.
        _ = accessed;
        call("access of inactive union field", null);
    }
    pub fn sliceCastLenRemainder(src_len: usize) noreturn {
        _ = src_len;
        call("slice length does not divide exactly into destination elements", null);
    }
    pub fn reachedUnreachable() noreturn {
        call("reached unreachable code", null);
    }
    pub fn unwrapNull() noreturn {
        call("attempt to use null value", null);
    }
    pub fn castToNull() noreturn {
        call("cast causes pointer to be null", null);
    }
    pub fn incorrectAlignment() noreturn {
        call("incorrect alignment", null);
    }
    pub fn invalidErrorCode() noreturn {
        call("invalid error code", null);
    }
    pub fn integerOutOfBounds() noreturn {
        call("integer does not fit in destination type", null);
    }
    pub fn integerOverflow() noreturn {
        call("integer overflow", null);
    }
    pub fn shlOverflow() noreturn {
        call("left shift overflowed bits", null);
    }
    pub fn shrOverflow() noreturn {
        call("right shift overflowed bits", null);
    }
    pub fn divideByZero() noreturn {
        call("division by zero", null);
    }
    pub fn exactDivisionRemainder() noreturn {
        call("exact division produced remainder", null);
    }
    pub fn integerPartOutOfBounds() noreturn {
        call("integer part of floating point value out of bounds", null);
    }
    pub fn corruptSwitch() noreturn {
        call("switch on corrupt value", null);
    }
    pub fn shiftRhsTooBig() noreturn {
        call("shift amount is greater than the type size", null);
    }
    pub fn invalidEnumValue() noreturn {
        call("invalid enum value", null);
    }
    pub fn forLenMismatch() noreturn {
        call("for loop over objects with non-equal lengths", null);
    }
    pub fn copyLenMismatch() noreturn {
        call("source and destination arguments have non-equal lengths", null);
    }
    pub fn memcpyAlias() noreturn {
        call("@memcpy arguments alias", null);
    }
    pub fn noreturnReturned() noreturn {
        call("'noreturn' function returned", null);
    }
};

/// Formats x into out (a NUL-terminated buffer of n bytes), in Go strconv's
/// %g layout; is_f32 non-zero formats x as the f32 value it came from.
export fn rt_format_float(x: f64, is_f32: c_int, out: [*c]u8, n: usize) callconv(.c) void {
    if (out == null or n == 0) return;
    dragon4.formatFloat(x, is_f32 != 0, out[0..n]);
}

/// Parses s as f64. *end is one past the consumed text, *erange is set the
/// way glibc sets errno==ERANGE (over- or underflow, even though *out still
/// holds the correctly rounded result); erange may be NULL.
export fn rt_parse_f64(s: [*c]const u8, end: *[*c]const u8, out: *f64, erange: ?*c_int) callconv(.c) c_int {
    var bits: u64 = 0;
    var local_erange: c_int = 0;
    if (algorithm_m.parseGeneric(s, end, 53, -1022, 1023, 2047, 63, &bits, &local_erange) == 0) return 0;
    if (erange) |e| e.* = local_erange;
    out.* = @bitCast(bits);
    return 1;
}

/// Parses s as f32; same contract as rt_parse_f64.
export fn rt_parse_f32(s: [*c]const u8, end: *[*c]const u8, out: *f32, erange: ?*c_int) callconv(.c) c_int {
    var bits: u64 = 0;
    var local_erange: c_int = 0;
    if (algorithm_m.parseGeneric(s, end, 24, -126, 127, 255, 31, &bits, &local_erange) == 0) return 0;
    if (erange) |e| e.* = local_erange;
    out.* = @bitCast(@as(u32, @truncate(bits)));
    return 1;
}
