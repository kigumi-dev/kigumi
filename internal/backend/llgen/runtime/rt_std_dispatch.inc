// Generated from internal/compiler/mir/ops/stdops.reg and std_dispatch_bodies.txt by TestStdDispatchFresh; do not edit.
val rt_std_id(int op, int n, val* a) {
    val a0 = n > 0 ? rt_deref(a[0]) : NULL, a1 = n > 1 ? rt_deref(a[1]) : NULL, a2 = n > 2 ? rt_deref(a[2]) : NULL;
    if (a0 && a0->kind == K_BOX && !op_stores_a0(op)) a0 = a0->u.box.v;
    switch ((enum std_op)op) {
    RT_STD_OPS_RETIRED_CASES
        rt_panic("retired std op");
        return NULL;
#ifdef KIGUMI_OP_SELECT
    default: break;
#endif
    // prelude
#if defined(KIGUMI_OP_LEN)
    case OP_LEN: return rt_int(a0->u.s.len, 64);
#endif
#if defined(KIGUMI_OP_STRING_TO_BYTES)
    case OP_STRING_TO_BYTES: return bytes_val(a0->u.s.data, a0->u.s.len);
#endif
#if defined(KIGUMI_OP_STRING_FROM_BYTES)
    case OP_STRING_FROM_BYTES:
        if (!rt_utf8_valid(a0->u.s.data, a0->u.s.len)) return rt_err_msg("invalid UTF-8");
        return rt_ok(strdup_val(a0->u.s.data, a0->u.s.len));
#endif
#if defined(KIGUMI_OP_STRING_TRY_FROM_BYTES)
    case OP_STRING_TRY_FROM_BYTES: {
        if (!rt_utf8_valid(a0->u.s.data, a0->u.s.len)) return rt_err_msg("invalid UTF-8");
        char* d = strdup_or_null(a0->u.s.data, a0->u.s.len);
        if (!d) return oom_error();
        val v = rt_str_take_try(d, a0->u.s.len);
        if (!v) {
            free(d);
            return oom_error();
        }
        val r = rt_variant_try(V_Ok, 1, &v);
        if (!r) {
            rt_release(v);
            return oom_error();
        }
        return r;
    }
#endif
#if defined(KIGUMI_OP_EQUALS)
    case OP_EQUALS: return rt_bool(a0->kind == K_FLOAT ? float_total_equal(a0->u.f, a1->u.f) : rt_equal(a0, a1));
#endif
#if defined(KIGUMI_OP_COMPARE_TO)
    case OP_COMPARE_TO: return rt_ordering(a0->kind == K_FLOAT ? float_total_compare(a0->u.f, a1->u.f) : cmp_vals(a0, a1));
#endif
#if defined(KIGUMI_OP_HASH)
    case OP_HASH: return rt_int((int64_t)rt_hash_of(a0), 64);
#endif
#if defined(KIGUMI_OP_MAP_SEED)
    case OP_MAP_SEED: return rt_int((int64_t)map_seed(), 64);
#endif
#if defined(KIGUMI_OP_DISPLAY)
    case OP_DISPLAY: return rt_display(a0);
#endif
#if defined(KIGUMI_OP_STRING_BYTES)
    case OP_STRING_BYTES: return bytes_val(a0->u.s.data, a0->u.s.len);
#endif
#if defined(KIGUMI_OP_BYTES_TRY_ZEROS)
    case OP_BYTES_TRY_ZEROS: {
        int64_t n2 = a0->u.i;
        if (n2 < 0) return alloc_error();
        char* d = fault_tick() ? NULL : kg_calloc_n(n2);
        if (!d) return alloc_error();
        val v = rt_str_take_try(d, n2);
        if (!v) {
            free(d);
            return alloc_error();
        }
        v->kind = K_BYTES;
        return try_ok(v);
    }
#endif
#if defined(KIGUMI_OP_BYTES_TRY_FROM_ARRAY)
    case OP_BYTES_TRY_FROM_ARRAY: {
        int64_t n2 = a0->u.arr.len;
        char* d = fault_tick() ? NULL : malloc(n2 + 1);
        if (!d) return alloc_error();
        for (int64_t i = 0; i < n2; i++) d[i] = (char)rt_deref(a0->u.arr.items[i])->u.i;
        d[n2] = 0;
        val v = rt_str_take_try(d, n2);
        if (!v) {
            free(d);
            return alloc_error();
        }
        v->kind = K_BYTES;
        return try_ok(v);
    }
#endif
#if defined(KIGUMI_OP_BYTES_TRY_CONCAT)
    case OP_BYTES_TRY_CONCAT: {
        int64_t l0 = a0->u.s.len, l1 = a1->u.s.len;
        char* d = fault_tick() ? NULL : malloc(l0 + l1 + 1);
        if (!d) return alloc_error();
        memcpy(d, a0->u.s.data, l0);
        memcpy(d + l0, a1->u.s.data, l1);
        d[l0 + l1] = 0;
        val v = rt_str_take_try(d, l0 + l1);
        if (!v) {
            free(d);
            return alloc_error();
        }
        v->kind = K_BYTES;
        return try_ok(v);
    }
#endif
#if defined(KIGUMI_OP_ARRAY_TRY_MAP)
    case OP_ARRAY_TRY_MAP: {
        val out = array_try_new(a0->u.arr.len);
        if (!out) return alloc_error();
        UNWIND_PUSH(u, unwind_release_out, NULL, out, 0);
        for (int64_t i = 0; i < a0->u.arr.len; i++) {
            val arg[1] = {a0->u.arr.items[i]};
            out->u.arr.items[out->u.arr.len++] = rt_callv_lent(a1, 1, arg);
        }
        UNWIND_POP(u);
        return try_ok(out);
    }
#endif
#if defined(KIGUMI_OP_ARRAY_TRY_FILTER)
    case OP_ARRAY_TRY_FILTER: {
        val out = array_try_new(0);
        if (!out) return alloc_error();
        UNWIND_PUSH(u, unwind_release_out, NULL, out, 0);
        for (int64_t i = 0; i < a0->u.arr.len; i++) {
            val arg[1] = {a0->u.arr.items[i]};
            val keep = rt_callv_lent(a1, 1, arg);
            int kept = rt_deref(keep)->u.b;
            rt_release(keep);
            if (!kept) continue;
            if (!try_grow(out, 1)) {
                UNWIND_POP(u);
                rt_release(out);
                return alloc_error();
            }
            out->u.arr.items[out->u.arr.len++] = rt_copy(a0->u.arr.items[i]);
        }
        UNWIND_POP(u);
        return try_ok(out);
    }
#endif
#if defined(KIGUMI_OP_ARRAY_TRY_INSERT_AT)
    case OP_ARRAY_TRY_INSERT_AT: {
        // The Ok wrapper is built before the array changes, so a refusal leaves it untouched.
        uint64_t at = (uint64_t)a1->u.i, len = (uint64_t)a0->u.arr.len;
        val fits = rt_bool(at <= len);
        val done = rt_variant_try(V_Ok, 1, &fits);
        if (!done) return alloc_error();
        if (at > len) return done;
        if (!try_grow(a0, 1)) {
            rt_release(done);
            return alloc_error();
        }
        for (uint64_t k = len; k > at; k--) a0->u.arr.items[k] = a0->u.arr.items[k - 1];
        a0->u.arr.items[at] = rt_retain(a[2]);
        a0->u.arr.len++;
        return done;
    }
#endif
#if defined(KIGUMI_OP_ARRAY_TRY_WITH_CAPACITY)
    case OP_ARRAY_TRY_WITH_CAPACITY: {
        val out = array_try_new(a0->u.i);
        return out ? try_ok(out) : alloc_error();
    }
#endif
#if defined(KIGUMI_OP_ARRAY_TRY_SLICE)
    case OP_ARRAY_TRY_SLICE: {
        int64_t s = a1->u.i, e = a2->u.i;
        if (s < 0 || s > e || e > a0->u.arr.len) return try_ok(rt_none());
        val out = array_try_copy(a0, s, e);
        return out ? try_ok_some(out) : alloc_error();
    }
#endif
#if defined(KIGUMI_OP_ALLOC_WRAP)
    case OP_ALLOC_WRAP: return try_ok(rt_retain(a[0]));
#endif
#if defined(KIGUMI_OP_STRING_TRY_SLICE_BYTES)
    case OP_STRING_TRY_SLICE_BYTES: {
        int64_t s = a1->u.i, e = a2->u.i, len = a0->u.s.len;
        if (s < 0 || s > e || e > len || !boundary(a0->u.s.data, len, s) || !boundary(a0->u.s.data, len, e)) return try_ok(rt_none());
        char* d = try_dup(a0->u.s.data + s, e - s);
        if (!d) return alloc_error();
        val v = rt_str_take_try(d, e - s);
        if (!v) {
            free(d);
            return alloc_error();
        }
        return try_ok_some(v);
    }
#endif
#if defined(KIGUMI_OP_BYTES_TRY_SLICE)
    case OP_BYTES_TRY_SLICE: {
        int64_t s = a1->u.i, e = a2->u.i, len = a0->u.s.len;
        if (s < 0 || s > e || e > len) return try_ok(rt_none());
        char* d = try_dup(a0->u.s.data + s, e - s);
        if (!d) return alloc_error();
        val v = rt_str_take_try(d, e - s);
        if (!v) {
            free(d);
            return alloc_error();
        }
        v->kind = K_BYTES;
        return try_ok_some(v);
    }
#endif
#if defined(KIGUMI_OP_MAP_TRY_CLONE)
    case OP_MAP_TRY_CLONE: {
        val out = record_try_new(a0->u.rec.type, a0->u.rec.n);
        if (!out) return alloc_error();
        for (int64_t i = 0; i < a0->u.rec.n; i++) {
            if (!copy_slot_try(&out->u.rec.fields[i], a0->u.rec.fields[i])) {
                rt_release(out);
                return alloc_error();
            }
        }
        return try_ok(out);
    }
#endif
#if defined(KIGUMI_OP_OS_STRING_TRY_CLONE)
    case OP_OS_STRING_TRY_CLONE: {
        val out = copy_try(a0);
        return out ? try_ok(out) : alloc_error();
    }
#endif
#if defined(KIGUMI_OP_MAP_TRY_ENTRIES)
    case OP_MAP_TRY_ENTRIES: {
        val keys = rt_deref(a0->u.rec.fields[0]), values = rt_deref(a0->u.rec.fields[1]);
        rt_type* entry = rt_find_type("std/map.Entry");
        if (!entry) rt_panic("a runtime-named type was dropped by reachability");
        val out = array_try_new(keys->u.arr.len);
        if (!out) return alloc_error();
        for (int64_t i = 0; i < keys->u.arr.len; i++) {
            val rec = record_try_new(entry, 2);
            if (!rec) {
                rt_release(out);
                return alloc_error();
            }
            out->u.arr.items[i] = rec;
            out->u.arr.len++;
            if (!copy_slot_try(&rec->u.rec.fields[0], keys->u.arr.items[i]) || !copy_slot_try(&rec->u.rec.fields[1], values->u.arr.items[i])) {
                rt_release(out);
                return alloc_error();
            }
        }
        return try_ok(out);
    }
#endif
#if defined(KIGUMI_OP_MAP_TRY_EMPTY)
    case OP_MAP_TRY_EMPTY: return empty_record_try(rt_find_type("std/map.Map"), 5);
#endif
#if defined(KIGUMI_OP_STRING_BUILDER_TRY_NEW)
    case OP_STRING_BUILDER_TRY_NEW: return empty_record_try(rt_find_type("std/text.StringBuilder"), 1);
#endif
#if defined(KIGUMI_OP_STRING_TRY_CHARS)
    case OP_STRING_TRY_CHARS: return try_string_chars(a0);
#endif
#if defined(KIGUMI_OP_ARRAY_TRY_SORTED_BY)
    case OP_ARRAY_TRY_SORTED_BY: return try_array_sorted_by(a0, a1);
#endif
#if defined(KIGUMI_OP_STRING_BUILDER_TRY_FREEZE)
    case OP_STRING_BUILDER_TRY_FREEZE: return try_builder_freeze(a0);
#endif
#if defined(KIGUMI_OP_DEBUG_TRY)
    case OP_DEBUG_TRY: return try_debug(a0);
#endif
#if defined(KIGUMI_OP_BYTE_BUFFER_SET)
    case OP_BYTE_BUFFER_SET:
        if (a1->u.i < 0 || a1->u.i >= a0->u.s.len) rt_panic("ByteBuffer.set: index out of range");
        ((char*)a0->u.s.data)[a1->u.i] = (char)a2->u.i;
        return rt_unit();
#endif
#if defined(KIGUMI_OP_BYTE_BUFFER_COPY_FROM)
    case OP_BYTE_BUFFER_COPY_FROM: {
        val src = a2;
        int64_t at = a1->u.i, lo = rt_deref(a[3])->u.i, hi = rt_deref(a[4])->u.i;
        if (lo < 0 || lo > hi || hi > src->u.s.len || at < 0 || at > a0->u.s.len - (hi - lo)) rt_panic("ByteBuffer.copyFrom: range out of bounds");
        memcpy((char*)a0->u.s.data + at, src->u.s.data + lo, (size_t)(hi - lo));
        return rt_unit();
    }
#endif
#if defined(KIGUMI_OP_BYTE_BUFFER_FREEZE)
    case OP_BYTE_BUFFER_FREEZE: return rt_retain(a0);
#endif
#if defined(KIGUMI_OP_BYTE_BUFFER_FREEZE_STRING)
    case OP_BYTE_BUFFER_FREEZE_STRING:
        if (!rt_utf8_valid(a0->u.s.data, a0->u.s.len)) rt_panic("ByteBuffer.freezeString: invalid UTF-8");
        a0->kind = K_STR;
        return rt_retain(a0);
#endif
    // array
#if defined(KIGUMI_OP_ARRAY_EMPTY)
    case OP_ARRAY_EMPTY: return rt_array(0, NULL);
#endif
#if defined(KIGUMI_OP_ARRAY_OF)
    case OP_ARRAY_OF: return rt_retain(a0);
#endif
#if defined(KIGUMI_OP_ARRAY_PUSH)
    case OP_ARRAY_PUSH:
        rt_push(a0, rt_retain(a[1]));
        return rt_unit();
#endif
#if defined(KIGUMI_OP_ARRAY_GROW_BY)
    case OP_ARRAY_GROW_BY: return rt_bool(try_grow(a0, (uint64_t)a1->u.i));
#endif
#if defined(KIGUMI_OP_ARRAY_PUSH_COPY)
    case OP_ARRAY_PUSH_COPY: {
        if (!try_grow(a0, 1)) return rt_bool(0);
        val x = writable_inside(a1) ? copy_try(a1) : rt_retain(a1);
        if (!x && a1) return rt_bool(0);
        a0->u.arr.items[a0->u.arr.len++] = x;
        return rt_bool(1);
    }
#endif
#if defined(KIGUMI_OP_ARRAY_LEN)
    case OP_ARRAY_LEN: return rt_int(a0->u.arr.len, 64);
#endif
#if defined(KIGUMI_OP_ARRAY_GET)
    case OP_ARRAY_GET: {
        int64_t i = a1->u.i;
        if (i < 0 || i >= a0->u.arr.len) return rt_none();
        return rt_some(rt_copy(a0->u.arr.items[i]));
    }
#endif
#if defined(KIGUMI_OP_ARRAY_FIRST)
    case OP_ARRAY_FIRST:
        if (a0->u.arr.len == 0) return rt_none();
        return rt_some(rt_copy(a0->u.arr.items[0]));
#endif
#if defined(KIGUMI_OP_ARRAY_LAST)
    case OP_ARRAY_LAST:
        if (a0->u.arr.len == 0) return rt_none();
        return rt_some(rt_copy(a0->u.arr.items[a0->u.arr.len - 1]));
#endif
#if defined(KIGUMI_OP_ARRAY_WITH)
    case OP_ARRAY_WITH: {
        int64_t i = a1->u.i;
        if (i < 0 || i >= a0->u.arr.len) return rt_none();
        val arg[1] = {a0->u.arr.items[i]};
        return rt_some(rt_callv_lent(a2, 1, arg));
    }
#endif
#if defined(KIGUMI_OP_ARRAY_WITH_MUT)
    case OP_ARRAY_WITH_MUT: {
        int64_t i = a1->u.i;
        if (i < 0 || i >= a0->u.arr.len) return rt_none();
        // A scalar element aliases its obj on ordinary copy, so the
        // callback writes through a cell rather than the element itself;
        // the slot takes the cell's (possibly new) content back after.
        val cell = rt_cell(a0->u.arr.items[i]);
        val arg[1] = {cell};
        UNWIND_PUSH(u, unwind_lend_back, a0, cell, i);
        val r = rt_callv_lent(a2, 1, arg);
        UNWIND_POP(u);
        a0->u.arr.items[i] = cell->u.cell.v;
        // free_obj on a cell releases its `.v`, already handed to the
        // array slot above; detach it first so that release is a no-op.
        cell->u.cell.v = NULL;
        rt_release(cell);
        return rt_some(r);
    }
#endif
#if defined(KIGUMI_OP_ARRAY_TAKE_AT)
    case OP_ARRAY_TAKE_AT: {
        int64_t i = a1->u.i;
        if (i < 0 || i >= a0->u.arr.len) return rt_none();
        val x = a0->u.arr.items[i];
        if (i < a0->u.arr.len - 1 - i) {
            for (int64_t j = i; j > 0; j--) a0->u.arr.items[j] = a0->u.arr.items[j - 1];
            a0->u.arr.items++;
            a0->u.arr.head++;
            a0->u.arr.cap--;
        } else {
            for (int64_t j = i; j + 1 < a0->u.arr.len; j++) a0->u.arr.items[j] = a0->u.arr.items[j + 1];
        }
        a0->u.arr.len--;
        return rt_some(x);
    }
#endif
#if defined(KIGUMI_OP_OPTION_TAKE)
    case OP_OPTION_TAKE: {
        if (a0->u.var.v != V_Some) return rt_none();
        val x = rt_retain(a0->u.var.payload[0]);
        rt_cellset(a[0], rt_none());
        return rt_some(x);
    }
#endif
    // text
#if defined(KIGUMI_OP_TEXT_PARSE_FLOAT)
    case OP_TEXT_PARSE_FLOAT: {
        double d;
        if (!parse_float_text(a0, &d)) return rt_none();
        return rt_some(rt_float(d, 64 | 512));
    }
#endif
    // os / fs
#ifdef KIGUMI_CAP_HOST_SHELL
#if defined(KIGUMI_OP_HOST_SHELL)
    case OP_HOST_SHELL: return cap_host_shell();
#endif
#else
#if defined(KIGUMI_OP_HOST_SHELL)
    case OP_HOST_SHELL: rt_panic("this build has no host.shell capability"); return NULL;
#endif
#endif
#ifdef KIGUMI_CAP_HOST_ARGS
#if defined(KIGUMI_OP_HOST_ARGS)
    case OP_HOST_ARGS: return cap_host_args();
#endif
#else
#if defined(KIGUMI_OP_HOST_ARGS)
    case OP_HOST_ARGS: rt_panic("this build has no host.args capability"); return NULL;
#endif
#endif
#ifdef KIGUMI_CAP_HOST_FILES
#if defined(KIGUMI_OP_HOST_FILES)
    case OP_HOST_FILES: return cap_host_files();
#endif
#else
#if defined(KIGUMI_OP_HOST_FILES)
    case OP_HOST_FILES: rt_panic("this build has no host.files capability"); return NULL;
#endif
#endif
#ifdef KIGUMI_CAP_HOST_NET
#if defined(KIGUMI_OP_HOST_NET)
    case OP_HOST_NET: return cap_host_net();
#endif
#else
#if defined(KIGUMI_OP_HOST_NET)
    case OP_HOST_NET: rt_panic("this build has no host.net capability"); return NULL;
#endif
#endif
#ifdef KIGUMI_CAP_HOST_DL
#if defined(KIGUMI_OP_HOST_DL)
    case OP_HOST_DL: return cap_host_dl();
#endif
#else
#if defined(KIGUMI_OP_HOST_DL)
    case OP_HOST_DL: rt_panic("this build has no host.dl capability"); return NULL;
#endif
#endif
#ifdef KIGUMI_CAP_HOST_ENTROPY
#if defined(KIGUMI_OP_HOST_ENTROPY)
    case OP_HOST_ENTROPY: return cap_host_entropy();
#endif
#else
#if defined(KIGUMI_OP_HOST_ENTROPY)
    case OP_HOST_ENTROPY: rt_panic("this build has no host.entropy capability"); return NULL;
#endif
#endif
#ifdef KIGUMI_CAP_HOST_SIGNALS
#if defined(KIGUMI_OP_HOST_SIGNALS)
    case OP_HOST_SIGNALS: return cap_host_signals();
#endif
#else
#if defined(KIGUMI_OP_HOST_SIGNALS)
    case OP_HOST_SIGNALS: rt_panic("this build has no host.signals capability"); return NULL;
#endif
#endif
#ifdef KIGUMI_CAP_HOST_ARGS
#if defined(KIGUMI_OP_ARGS_GET)
    case OP_ARGS_GET: return cap_args_get(a1);
#endif
#if defined(KIGUMI_OP_ARGS_GET_BYTES)
    case OP_ARGS_GET_BYTES: return cap_args_get_bytes(a1);
#endif
#else
#if defined(KIGUMI_OP_ARGS_GET)
    case OP_ARGS_GET: rt_panic("this build has no host.args capability"); return NULL;
#endif
#if defined(KIGUMI_OP_ARGS_GET_BYTES)
    case OP_ARGS_GET_BYTES: rt_panic("this build has no host.args capability"); return NULL;
#endif
#endif
    // ffi / crypto / dl
#ifdef KIGUMI_CAP_FFI_CSTRING
#if defined(KIGUMI_OP_CSTRING_NEW)
    case OP_CSTRING_NEW: return cap_cstring_new(a0);
#endif
#if defined(KIGUMI_OP_CSTRING_PTR)
    case OP_CSTRING_PTR: return cap_cstring_ptr(a0);
#endif
#if defined(KIGUMI_OP_CSTRING_DROP)
    case OP_CSTRING_DROP: return cap_cstring_drop(a0);
#endif
#else
#if defined(KIGUMI_OP_CSTRING_NEW) || defined(KIGUMI_OP_CSTRING_PTR) || defined(KIGUMI_OP_CSTRING_DROP)
    case OP_CSTRING_NEW:
    case OP_CSTRING_PTR:
    case OP_CSTRING_DROP:
        rt_panic("this build has no ffi.cstring capability");
        return NULL;
#endif
#endif
#ifdef KIGUMI_CAP_FFI_RAW
#if defined(KIGUMI_OP_FFI_BYTES_FROM)
    case OP_FFI_BYTES_FROM: return cap_ffi_bytes_from(a0, a1);
#endif
#if defined(KIGUMI_OP_FFI_BYTES_PTR)
    case OP_FFI_BYTES_PTR: return cap_ffi_bytes_ptr(a0);
#endif
#else
#if defined(KIGUMI_OP_FFI_BYTES_FROM) || defined(KIGUMI_OP_FFI_BYTES_PTR)
    case OP_FFI_BYTES_FROM:
    case OP_FFI_BYTES_PTR:
        rt_panic("this build has no ffi.raw capability");
        return NULL;
#endif
#endif
#if defined(KIGUMI_OP_BYTES_GET)
    case OP_BYTES_GET: {
        int64_t i = a1->u.i;
        if (i < 0 || i >= a0->u.s.len) return rt_none();
        return rt_some(rt_int((uint8_t)a0->u.s.data[i], 8));
    }
#endif
#if defined(KIGUMI_OP_SECRET_BARRIER_U64)
    case OP_SECRET_BARRIER_U64: return rt_int((int64_t)secret_barrier_u64((uint64_t)a0->u.i), 64);
#endif
#if defined(KIGUMI_OP_SECRET_MASK_NONZERO_U64)
    case OP_SECRET_MASK_NONZERO_U64: return rt_int((int64_t)secret_mask_nonzero_u64((uint64_t)a0->u.i), 64);
#endif
#if defined(KIGUMI_OP_SECRET_MASK_LT_U64)
    case OP_SECRET_MASK_LT_U64: return rt_int((int64_t)secret_mask_lt_u64((uint64_t)a0->u.i, (uint64_t)a1->u.i), 64);
#endif
#if defined(KIGUMI_OP_SECRET_SELECT_U64)
    case OP_SECRET_SELECT_U64: return rt_int((int64_t)secret_select_u64((uint64_t)a0->u.i, (uint64_t)a1->u.i, (uint64_t)a2->u.i), 64);
#endif
#if defined(KIGUMI_OP_SECRET_EQ_BYTES)
    case OP_SECRET_EQ_BYTES: return rt_int((int64_t)secret_eq_bytes(a0, a1), 64);
#endif
#if defined(KIGUMI_OP_SUBTLE_CT_EQ)
    case OP_SUBTLE_CT_EQ: return rt_bool(subtle_constant_time_eq(a0, a1));
#endif
#ifdef KIGUMI_CAP_MEM_SECURE_ERASE
#if defined(KIGUMI_OP_SECURE_ERASE)
    case OP_SECURE_ERASE: return cap_secure_erase(a0);
#endif
#if defined(KIGUMI_OP_SECURE_ERASE_RAW)
    case OP_SECURE_ERASE_RAW: return cap_secure_erase_raw(a0, a1);
#endif
#if defined(KIGUMI_OP_SECURE_ERASE_TRY)
    case OP_SECURE_ERASE_TRY: return cap_secure_erase_try(a0);
#endif
#else
#if defined(KIGUMI_OP_SECURE_ERASE) || defined(KIGUMI_OP_SECURE_ERASE_RAW) || defined(KIGUMI_OP_SECURE_ERASE_TRY)
    case OP_SECURE_ERASE:
    case OP_SECURE_ERASE_RAW:
    case OP_SECURE_ERASE_TRY:
        rt_panic("this build has no mem.secure-erase capability");
        return NULL;
#endif
#endif
#ifdef KIGUMI_CAP_HOST_DL
#if defined(KIGUMI_OP_DL_SYMBOL_CALL)
    case OP_DL_SYMBOL_CALL: return cap_dl_symbol_call();
#endif
#else
#if defined(KIGUMI_OP_DL_SYMBOL_CALL)
    case OP_DL_SYMBOL_CALL: rt_panic("this build has no host.dl capability"); return NULL;
#endif
#endif
#ifdef KIGUMI_CAP_HOST_STDIO
#if defined(KIGUMI_OP_HOST_STDIN)
    case OP_HOST_STDIN: return cap_host_stdin();
#endif
#if defined(KIGUMI_OP_HOST_STDOUT)
    case OP_HOST_STDOUT: return cap_host_stdout();
#endif
#if defined(KIGUMI_OP_HOST_STDERR)
    case OP_HOST_STDERR: return cap_host_stderr();
#endif
#else
#if defined(KIGUMI_OP_HOST_STDIN) || defined(KIGUMI_OP_HOST_STDOUT) || defined(KIGUMI_OP_HOST_STDERR)
    case OP_HOST_STDIN:
    case OP_HOST_STDOUT:
    case OP_HOST_STDERR:
        rt_panic("this build has no host.stdio capability");
        return NULL;
#endif
#endif
#ifdef KIGUMI_CAP_FFI_PIN
#if defined(KIGUMI_OP_PIN_NEW)
    case OP_PIN_NEW: return cap_pin_new(a[0]);
#endif
#if defined(KIGUMI_OP_PIN_PTR)
    case OP_PIN_PTR: return cap_pin_ptr(a0);
#endif
#if defined(KIGUMI_OP_PIN_WITH)
    case OP_PIN_WITH: return cap_pin_with(a0, a1);
#endif
#else
#if defined(KIGUMI_OP_PIN_NEW) || defined(KIGUMI_OP_PIN_PTR) || defined(KIGUMI_OP_PIN_WITH)
    case OP_PIN_NEW:
    case OP_PIN_PTR:
    case OP_PIN_WITH:
        rt_panic("this build has no ffi.pin capability");
        return NULL;
#endif
#endif
#if defined(KIGUMI_OP_WITH_ADDRESS)
    case OP_WITH_ADDRESS: return with_address(a[0], a0, a1);
#endif
#ifdef KIGUMI_CAP_FFI_PIN
#if defined(KIGUMI_OP_PIN_DROP)
    case OP_PIN_DROP: return cap_pin_drop(a0);
#endif
#if defined(KIGUMI_OP_PIN_RELEASE)
    case OP_PIN_RELEASE: return cap_pin_release(a0);
#endif
#if defined(KIGUMI_OP_PIN_RECLAIM)
    case OP_PIN_RECLAIM: return cap_pin_reclaim(a0);
#endif
#else
#if defined(KIGUMI_OP_PIN_DROP) || defined(KIGUMI_OP_PIN_RELEASE) || defined(KIGUMI_OP_PIN_RECLAIM)
    case OP_PIN_DROP:
    case OP_PIN_RELEASE:
    case OP_PIN_RECLAIM:
        rt_panic("this build has no ffi.pin capability");
        return NULL;
#endif
#endif
#ifdef KIGUMI_CAP_FFI_HANDLE_TABLE
#if defined(KIGUMI_OP_HANDLE_TABLE_NEW)
    case OP_HANDLE_TABLE_NEW: return cap_handle_table_new();
#endif
#if defined(KIGUMI_OP_HANDLE_TABLE_INSERT)
    case OP_HANDLE_TABLE_INSERT: return cap_handle_table_insert(a0, a1);
#endif
#if defined(KIGUMI_OP_HANDLE_TABLE_WITH)
    case OP_HANDLE_TABLE_WITH: return cap_handle_table_with(a1, a2);
#endif
#if defined(KIGUMI_OP_HANDLE_TABLE_REMOVE)
    case OP_HANDLE_TABLE_REMOVE: return cap_handle_table_remove(a1);
#endif
#if defined(KIGUMI_OP_HANDLE_TABLE_RECLAIM)
    case OP_HANDLE_TABLE_RECLAIM: return cap_handle_table_reclaim(a0, a1);
#endif
#if defined(KIGUMI_OP_HANDLE_TABLE_CLOSE)
    case OP_HANDLE_TABLE_CLOSE: return cap_handle_table_close(a0);
#endif
#if defined(KIGUMI_OP_HANDLE_TABLE_DROP)
    case OP_HANDLE_TABLE_DROP: return cap_handle_table_drop(a0);
#endif
#if defined(KIGUMI_OP_HANDLE_RAW)
    case OP_HANDLE_RAW: return cap_handle_raw(a0);
#endif
#if defined(KIGUMI_OP_HANDLE_DROP)
    case OP_HANDLE_DROP: return cap_handle_drop(a0);
#endif
#else
#if defined(KIGUMI_OP_HANDLE_TABLE_NEW) || defined(KIGUMI_OP_HANDLE_TABLE_INSERT) || defined(KIGUMI_OP_HANDLE_TABLE_WITH) || defined(KIGUMI_OP_HANDLE_TABLE_REMOVE) || defined(KIGUMI_OP_HANDLE_TABLE_RECLAIM) || defined(KIGUMI_OP_HANDLE_TABLE_CLOSE) || defined(KIGUMI_OP_HANDLE_TABLE_DROP) || defined(KIGUMI_OP_HANDLE_RAW) || defined(KIGUMI_OP_HANDLE_DROP)
    case OP_HANDLE_TABLE_NEW:
    case OP_HANDLE_TABLE_INSERT:
    case OP_HANDLE_TABLE_WITH:
    case OP_HANDLE_TABLE_REMOVE:
    case OP_HANDLE_TABLE_RECLAIM:
    case OP_HANDLE_TABLE_CLOSE:
    case OP_HANDLE_TABLE_DROP:
    case OP_HANDLE_RAW:
    case OP_HANDLE_DROP:
        rt_panic("this build has no ffi.handle-table capability");
        return NULL;
#endif
#endif
#ifdef KIGUMI_CAP_FFI_RAW
#if defined(KIGUMI_OP_FFI_STRING)
    case OP_FFI_STRING: return cap_ffi_string(a0);
#endif
#if defined(KIGUMI_OP_FFI_STRING_LOSSY)
    case OP_FFI_STRING_LOSSY: return cap_ffi_string_lossy(a0);
#endif
#if defined(KIGUMI_OP_FFI_STRING_CHECKED)
    case OP_FFI_STRING_CHECKED: return cap_ffi_string_checked(a0);
#endif
#else
#if defined(KIGUMI_OP_FFI_STRING) || defined(KIGUMI_OP_FFI_STRING_LOSSY) || defined(KIGUMI_OP_FFI_STRING_CHECKED)
    case OP_FFI_STRING:
    case OP_FFI_STRING_LOSSY:
    case OP_FFI_STRING_CHECKED:
        rt_panic("this build has no ffi.raw capability");
        return NULL;
#endif
#endif
    // task / alloc / net
#ifdef KIGUMI_CAP_TASK_EXECUTOR
#if defined(KIGUMI_OP_TASK_CURRENT_EXECUTOR)
    case OP_TASK_CURRENT_EXECUTOR: return rt_executor_new();
#endif
#if defined(KIGUMI_OP_EXECUTOR_RUN_TO_COMPLETION)
    case OP_EXECUTOR_RUN_TO_COMPLETION: return rt_run_to_completion(a0, rt_retain(a1));
#endif
#if defined(KIGUMI_OP_TASK_SPAWN)
    case OP_TASK_SPAWN: return rt_task_spawn(rt_retain(a0));
#endif
#if defined(KIGUMI_OP_TASK_CANCEL)
    case OP_TASK_CANCEL: rt_task_cancel(a0); return rt_unit();
#endif
#if defined(KIGUMI_OP_TASK_CANCEL_REQUESTED)
    case OP_TASK_CANCEL_REQUESTED: return rt_bool(rt_cancel_requested());
#endif
#if defined(KIGUMI_OP_TASK_THREAD_POOL_EXECUTOR)
    case OP_TASK_THREAD_POOL_EXECUTOR: return rt_thread_pool_new(a0->u.i);
#endif
#if defined(KIGUMI_OP_TASK_HANDOFF)
    case OP_TASK_HANDOFF: return rt_task_handoff(a0, rt_retain(a1));
#endif
#if defined(KIGUMI_OP_TASK_YIELD_NOW)
    case OP_TASK_YIELD_NOW: return rt_task_yield_now();
#endif
#else
#if defined(KIGUMI_OP_TASK_CURRENT_EXECUTOR) || defined(KIGUMI_OP_EXECUTOR_RUN_TO_COMPLETION) || defined(KIGUMI_OP_TASK_SPAWN) || defined(KIGUMI_OP_TASK_CANCEL) || defined(KIGUMI_OP_TASK_CANCEL_REQUESTED) || defined(KIGUMI_OP_TASK_THREAD_POOL_EXECUTOR) || defined(KIGUMI_OP_TASK_HANDOFF) || defined(KIGUMI_OP_TASK_YIELD_NOW)
    case OP_TASK_CURRENT_EXECUTOR:
    case OP_EXECUTOR_RUN_TO_COMPLETION:
    case OP_TASK_SPAWN:
    case OP_TASK_CANCEL:
    case OP_TASK_CANCEL_REQUESTED:
    case OP_TASK_THREAD_POOL_EXECUTOR:
    case OP_TASK_HANDOFF:
    case OP_TASK_YIELD_NOW:
        rt_panic("this build has no task.executor capability");
        return NULL;
#endif
#endif
#if defined(KIGUMI_OP_ARENA_CREATE)
    case OP_ARENA_CREATE: {
        arena* ar = (arena_fault_injected() || fault_tick()) ? NULL : calloc(1, sizeof *ar);
        if (!ar) return oom_error();
        ar->a.alloc = arena_alloc;
        ar->a.free = arena_free;
        return rt_ok(rt_opaque("AllocatorHandle", ar));
    }
#endif
#if defined(KIGUMI_OP_ARENA_TRY_CREATE)
    case OP_ARENA_TRY_CREATE: {
        arena* ar = (arena_fault_injected() || fault_tick()) ? NULL : calloc(1, sizeof *ar);
        if (!ar) return alloc_error();
        ar->a.alloc = arena_alloc;
        ar->a.free = arena_free;
        val h = opaque_try("AllocatorHandle", ar);
        if (!h) {
            free(ar);
            return alloc_error();
        }
        return try_ok(h);
    }
#endif
#if defined(KIGUMI_OP_ALLOCATOR_SCOPE)
    case OP_ALLOCATOR_SCOPE: return opaque_in(&((arena*)a0->u.op.data)->a, "AllocatorScope", rt_retain(a0));
#endif
#if defined(KIGUMI_OP_ALLOCATOR_ALLOCATED)
    case OP_ALLOCATOR_ALLOCATED: return rt_int((int64_t)((arena*)a0->u.op.data)->a.bytes, 64 | 256);
#endif
#if defined(KIGUMI_OP_SHARED_TRY_NEW)
    case OP_SHARED_TRY_NEW: {
        sctrl* c = fault_tick() ? NULL : calloc(1, sizeof *c);
        if (!c) return alloc_error();
        val cell = rt_cell_try(rt_retain(a0));
        if (!cell) {
            rt_release(a0);
            free(c);
            return alloc_error();
        }
        c->cell = cell;
        c->alive = 1;
        val sh = opaque_try("Shared", c);
        if (!sh) {
            rt_release(c->cell);
            free(c);
            return alloc_error();
        }
        c->shared = sh;
        return try_ok(sh);
    }
#endif
#if defined(KIGUMI_OP_SHARED_CLONE)
    case OP_SHARED_CLONE: return rt_retain(a0);
#endif
#if defined(KIGUMI_OP_SHARED_SET)
    case OP_SHARED_SET: {
        sctrl* c = a0->u.op.data;
        if (c->borrow != 0) rt_panic("Shared is already borrowed");
        val cell = c->cell;
        val old = cell->u.cell.v;
        cell->u.cell.v = rt_retain(a1);
        rt_release(old);
        return rt_unit();
    }
#endif
#if defined(KIGUMI_OP_SHARED_WITH)
    case OP_SHARED_WITH: {
        sctrl* c = a0->u.op.data;
        if (c->borrow < 0) rt_panic("Shared is exclusively borrowed");
        c->borrow++;
        val arg[1] = {c->cell->u.cell.v};
        UNWIND_PUSH(u, unwind_borrow_release, c, NULL, 0);
        val r = rt_callv_lent(a1, 1, arg);
        UNWIND_POP(u);
        c->borrow--;
        return r;
    }
#endif
#if defined(KIGUMI_OP_SHARED_WITH_MUT)
    case OP_SHARED_WITH_MUT: {
        sctrl* c = a0->u.op.data;
        if (c->borrow != 0) rt_panic("Shared is already borrowed");
        c->borrow = -1;
        // The callback writes through c->cell itself, same as any
        // other &mut Cell store, instead of a snapshot of its content.
        val arg[1] = {c->cell};
        UNWIND_PUSH(u, unwind_borrow_free, c, NULL, 0);
        val r = rt_callv_lent(a1, 1, arg);
        UNWIND_POP(u);
        c->borrow = 0;
        return r;
    }
#endif
#if defined(KIGUMI_OP_SHARED_TRY_GET)
    case OP_SHARED_TRY_GET: {
        val content = ((sctrl*)a0->u.op.data)->cell->u.cell.v;
        val copy = NULL;
        if (!copy_slot_try(&copy, content)) return alloc_error();
        return try_ok(copy);
    }
#endif
#if defined(KIGUMI_OP_SHARED_TRY_DOWNGRADE)
    case OP_SHARED_TRY_DOWNGRADE: {
        sctrl* c = a0->u.op.data;
        val weak = opaque_try("Weak", c);
        if (!weak) return alloc_error();
        c->weak++;
        return try_ok(weak);
    }
#endif
#if defined(KIGUMI_OP_WEAK_UPGRADE)
    case OP_WEAK_UPGRADE: {
        sctrl* c = a0->u.op.data;
        return c->alive ? rt_some(rt_retain(c->shared)) : rt_none();
    }
#endif
#if defined(KIGUMI_OP_BOX_TRY_NEW)
    case OP_BOX_TRY_NEW: {
        val cell = rt_cell_try(rt_retain(a0));
        if (!cell) {
            rt_release(a0);
            return alloc_error();
        }
        val box = opaque_try("Box", cell);
        if (!box) {
            rt_release(cell);
            return alloc_error();
        }
        return try_ok(box);
    }
#endif
#if defined(KIGUMI_OP_BOX_GET)
    case OP_BOX_GET: return rt_copy(((val)a0->u.op.data)->u.cell.v);
#endif
#if defined(KIGUMI_OP_BOX_SET)
    case OP_BOX_SET: {
        val cell = (val)a0->u.op.data;
        val old = cell->u.cell.v;
        cell->u.cell.v = rt_retain(a1);
        rt_release(old);
        return rt_unit();
    }
#endif
#if defined(KIGUMI_OP_BOX_WITH)
    case OP_BOX_WITH: {
        val arg[1] = {((val)a0->u.op.data)->u.cell.v};
        return rt_callv_lent(a1, 1, arg);
    }
#endif
#if defined(KIGUMI_OP_BOX_WITH_MUT)
    case OP_BOX_WITH_MUT: {
        val arg[1] = {(val)a0->u.op.data};
        return rt_callv_lent(a1, 1, arg);
    }
#endif
#if defined(KIGUMI_OP_BOX_INTO)
    case OP_BOX_INTO: return rt_retain(((val)a0->u.op.data)->u.cell.v);
#endif
#ifdef KIGUMI_CAP_HOST_NET
#if defined(KIGUMI_OP_NET_LISTENER_DROP) || defined(KIGUMI_OP_NET_CONN_DROP)
    case OP_NET_LISTENER_DROP:
    case OP_NET_CONN_DROP:
        return cap_net_conn_drop(a0);
#endif
#else
#if defined(KIGUMI_OP_NET_LISTENER_DROP) || defined(KIGUMI_OP_NET_CONN_DROP)
    case OP_NET_LISTENER_DROP:
    case OP_NET_CONN_DROP:
        rt_panic("this build has no host.net capability");
        return NULL;
#endif
#endif
#if defined(KIGUMI_OP_BUILD_ONLY)
    case OP_BUILD_ONLY:
        rt_panic("std/build only runs inside `kigumi build`");
        return NULL;
#endif
    // math / misc
#ifdef KIGUMI_CAP_HOST_ARGS
#if defined(KIGUMI_OP_ARGS_LEN)
    case OP_ARGS_LEN: return cap_args_len();
#endif
#else
#if defined(KIGUMI_OP_ARGS_LEN)
    case OP_ARGS_LEN: rt_panic("this build has no host.args capability"); return NULL;
#endif
#endif
#if defined(KIGUMI_OP_MATH_SQRT)
    case OP_MATH_SQRT: return rt_float(sqrt(a0->u.f), 64 | 512);
#endif
#if defined(KIGUMI_OP_MATH_FLOOR)
    case OP_MATH_FLOOR: return rt_float(floor(a0->u.f), 64 | 512);
#endif
#if defined(KIGUMI_OP_MATH_CEIL)
    case OP_MATH_CEIL: return rt_float(ceil(a0->u.f), 64 | 512);
#endif
#if defined(KIGUMI_OP_MATH_ROUND)
    case OP_MATH_ROUND: return rt_float(round(a0->u.f), 64 | 512);
#endif
#if defined(KIGUMI_OP_MATH_POW)
    case OP_MATH_POW: return rt_float(pow(a0->u.f, a1->u.f), 64 | 512);
#endif
    // int_fn reads the op name after the key's last '.', so a leading dot
    // is enough; reusing it keeps the wrapping/rotate math in one place.
#if defined(KIGUMI_OP_WRAPPING_ADD)
    case OP_WRAPPING_ADD: return int_fn(".wrappingAdd", a0, a1);
#endif
#if defined(KIGUMI_OP_WRAPPING_SUB)
    case OP_WRAPPING_SUB: return int_fn(".wrappingSub", a0, a1);
#endif
#if defined(KIGUMI_OP_WRAPPING_MUL)
    case OP_WRAPPING_MUL: return int_fn(".wrappingMul", a0, a1);
#endif
#if defined(KIGUMI_OP_WRAPPING_SHL)
    case OP_WRAPPING_SHL: return int_fn(".wrappingShiftLeft", a0, a1);
#endif
#if defined(KIGUMI_OP_WRAPPING_SHR)
    case OP_WRAPPING_SHR: return int_fn(".wrappingShiftRight", a0, a1);
#endif
#if defined(KIGUMI_OP_ROTATE_LEFT)
    case OP_ROTATE_LEFT: return int_fn(".rotateLeft", a0, a1);
#endif
#if defined(KIGUMI_OP_ROTATE_RIGHT)
    case OP_ROTATE_RIGHT: return int_fn(".rotateRight", a0, a1);
#endif
#if defined(KIGUMI_OP_CHAR_FROM_INT)
    case OP_CHAR_FROM_INT: {
        int64_t v = a0->u.i;
        if (v < 0 || v > 0x10FFFF || (v >= 0xD800 && v <= 0xDFFF)) return rt_none();
        return rt_some(rt_char((uint32_t)v));
    }
#endif
#if defined(KIGUMI_OP_UCELL_NEW)
    case OP_UCELL_NEW: return rt_opaque("UnsafeCell", rt_cell(rt_retain(a0)));
#endif
#if defined(KIGUMI_OP_UCELL_GET)
    case OP_UCELL_GET: return rt_copy(((val)a0->u.op.data)->u.cell.v);
#endif
#if defined(KIGUMI_OP_UCELL_SET)
    case OP_UCELL_SET: {
        val cell = (val)a0->u.op.data;
        val old = cell->u.cell.v;
        cell->u.cell.v = rt_retain(a1);
        rt_release(old);
        return rt_unit();
    }
#endif
#if defined(KIGUMI_OP_UCELL_ATOMIC_LOAD)
    case OP_UCELL_ATOMIC_LOAD: return ucell_atomic('l', a0, NULL, NULL);
#endif
#if defined(KIGUMI_OP_UCELL_ATOMIC_STORE)
    case OP_UCELL_ATOMIC_STORE: return ucell_atomic('s', a0, a1, NULL);
#endif
#if defined(KIGUMI_OP_UCELL_ATOMIC_EXCHANGE)
    case OP_UCELL_ATOMIC_EXCHANGE: return ucell_atomic('x', a0, a1, NULL);
#endif
#if defined(KIGUMI_OP_UCELL_ATOMIC_COMPARE_EXCHANGE)
    case OP_UCELL_ATOMIC_COMPARE_EXCHANGE: return ucell_atomic('c', a0, a1, a2);
#endif
#if defined(KIGUMI_OP_UCELL_ATOMIC_ADD)
    case OP_UCELL_ATOMIC_ADD: return ucell_atomic('+', a0, a1, NULL);
#endif
#if defined(KIGUMI_OP_UCELL_ATOMIC_SUB)
    case OP_UCELL_ATOMIC_SUB: return ucell_atomic('-', a0, a1, NULL);
#endif
#if defined(KIGUMI_OP_UCELL_ATOMIC_AND)
    case OP_UCELL_ATOMIC_AND: return ucell_atomic('&', a0, a1, NULL);
#endif
#if defined(KIGUMI_OP_UCELL_ATOMIC_OR)
    case OP_UCELL_ATOMIC_OR: return ucell_atomic('|', a0, a1, NULL);
#endif
#if defined(KIGUMI_OP_UCELL_ATOMIC_XOR)
    case OP_UCELL_ATOMIC_XOR: return ucell_atomic('^', a0, a1, NULL);
#endif
#if defined(KIGUMI_OP_UNINIT_NEW)
    case OP_UNINIT_NEW: return rt_opaque("Uninit", rt_cell(NULL));
#endif
#if defined(KIGUMI_OP_UNINIT_INITIALIZE)
    case OP_UNINIT_INITIALIZE: {
        ((val)a0->u.op.data)->u.cell.v = rt_retain(a1);
        return rt_unit();
    }
#endif
#if defined(KIGUMI_OP_UNINIT_BORROW)
    case OP_UNINIT_BORROW: return rt_retain(uninit_filled(a0));
#endif
#if defined(KIGUMI_OP_UNINIT_COPY)
    case OP_UNINIT_COPY: return rt_copy(uninit_filled(a0));
#endif
#if defined(KIGUMI_OP_UNINIT_BORROW_MUT)
    case OP_UNINIT_BORROW_MUT: return rt_retain((val)a0->u.op.data);
#endif
#if defined(KIGUMI_OP_UNINIT_TAKE)
    case OP_UNINIT_TAKE: {
        val cell = (val)a0->u.op.data;
        val v = uninit_filled(a0);
        cell->u.cell.v = NULL;
        return v;
    }
#endif
#if defined(KIGUMI_OP_UNINIT_DROP)
    case OP_UNINIT_DROP: {
        val cell = (val)a0->u.op.data;
        val old = cell->u.cell.v;
        cell->u.cell.v = NULL;
        rt_release(old);
        return rt_unit();
    }
#endif
#if defined(KIGUMI_OP_BLOCKING_POOL_NEW)
    case OP_BLOCKING_POOL_NEW:
#if defined(KIGUMI_FREESTANDING) || !defined(KIGUMI_CAP_TASK_BLOCKING_POOL)
        rt_panic("this build has no task.blocking-pool capability");
        return NULL;
#else
        return cap_blocking_pool_new(a0);
#endif
#endif
#if defined(KIGUMI_OP_BLOCKING_POOL_RUN)
    case OP_BLOCKING_POOL_RUN:
#if defined(KIGUMI_FREESTANDING) || !defined(KIGUMI_CAP_TASK_BLOCKING_POOL)
        rt_panic("this build has no task.blocking-pool capability");
        return NULL;
#else
        return cap_blocking_pool_run(a0, a1);
#endif
#endif
    }
    rt_panic("rt_std_id: bad op");
    return NULL;
}
