// Length of the codepoint at s[i] under utf8_valid's rules, or 0 when invalid.
static int utf8_decode_one(const char* s, int64_t n, int64_t i, uint32_t* cp_out) {
    unsigned char c = s[i];
    if (c < 0x80) {
        *cp_out = c;
        return 1;
    }
    int len;
    uint32_t min, cp;
    if ((c & 0xE0) == 0xC0) {
        len = 2;
        min = 0x80;
        cp = c & 0x1F;
    } else if ((c & 0xF0) == 0xE0) {
        len = 3;
        min = 0x800;
        cp = c & 0x0F;
    } else if ((c & 0xF8) == 0xF0) {
        len = 4;
        min = 0x10000;
        cp = c & 0x07;
    } else {
        return 0;
    }
    if (i + len > n) return 0;
    for (int k = 1; k < len; k++) {
        unsigned char cc = s[i + k];
        if ((cc & 0xC0) != 0x80) return 0;
        cp = (cp << 6) | (cc & 0x3F);
    }
    if (cp < min || cp > 0x10FFFF || (cp >= 0xD800 && cp <= 0xDFFF)) return 0;
    *cp_out = cp;
    return len;
}
// Each byte that starts no valid sequence becomes its own U+FFFD, as Go's DecodeRuneInString does.
static val utf8_lossy_val(const char* s, int64_t n) {
    buf b = {0};
    int64_t i = 0;
    while (i < n) {
        uint32_t cp;
        int len = utf8_decode_one(s, n, i, &cp);
        if (len == 0) {
            rt_bput(&b, "\xEF\xBF\xBD", 3);
            i++;
        } else {
            rt_bput(&b, s + i, (size_t)len);
            i += len;
        }
    }
    if (!b.p) {
        b.p = calloc(1, 1);
        if (!b.p) rt_panic("out of memory");
    }
    return rt_str_take(b.p, b.len);
}
static val cap_ffi_bytes_from(val a0, val a1) {
    return bytes_val((const char*)a0->u.p, a1->u.i);
}
static val cap_ffi_bytes_ptr(val a0) {
    return rt_ptr((void*)a0->u.s.data);
}
static val cap_ffi_string(val a0) {
    const char* s = a0->u.p ? (const char*)a0->u.p : "";
    return strdup_val(s, strlen(s));
}
static val cap_ffi_string_lossy(val a0) {
    const char* s = a0->u.p ? (const char*)a0->u.p : "";
    return utf8_lossy_val(s, (int64_t)strlen(s));
}
static val cap_ffi_string_checked(val a0) {
    const char* s = a0->u.p ? (const char*)a0->u.p : "";
    int64_t len = (int64_t)strlen(s);
    if (!rt_utf8_valid(s, len)) return rt_none();
    return rt_some(strdup_val(s, len));
}
