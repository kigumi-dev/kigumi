#include "rt.h"
#if !defined(KIGUMI_BARE)
#include <errno.h>
#ifdef _WIN32
#include <sys/stat.h>
typedef struct _stat64 rt_native_stat;
#define rt_stat _stat64
#else
#include <sys/stat.h>
typedef struct stat rt_native_stat;
#define rt_stat stat
#endif

typedef struct {
    int32_t kind;
    int64_t size;
    uint32_t mode;
    int64_t mtime_ms;
} rt_fs_meta;

int32_t rt_fs_stat(const char* path, rt_fs_meta* out) {
    rt_native_stat st;
    if (rt_stat(path, &st) != 0) return -errno;
#ifdef _WIN32
    out->kind = (st.st_mode & _S_IFMT) == _S_IFREG ? 0 : (st.st_mode & _S_IFMT) == _S_IFDIR ? 1 : 2;
#else
    out->kind = S_ISREG(st.st_mode) ? 0 : S_ISDIR(st.st_mode) ? 1 : 2;
#endif
    out->size = out->kind == 0 ? (int64_t)st.st_size : 0;
    out->mode = (uint32_t)(st.st_mode & 0777);
    out->mtime_ms = (int64_t)st.st_mtime * 1000;
    return 0;
}
#endif
