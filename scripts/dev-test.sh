#!/usr/bin/env bash
# Local test runner for Kigumi contributors: the CI entry points without the matrix, and no sudo.
#
#   scripts/dev-test.sh env      resolved toolchain, optional tools, TMPDIR and `kigumi doctor`
#   scripts/dev-test.sh check    gofmt, `kigumi fmt --check`, go vet (CI's first three steps)
#   scripts/dev-test.sh quick    unit tests of every package except internal/toolchain/driver
#   scripts/dev-test.sh full     go test ./... (unit tests and the whole corpus; long)
#   scripts/dev-test.sh aot      the pinned-zig AOT corpus in internal/toolchain/driver
#   scripts/dev-test.sh ct       SecretSan and constant-time tests
#
# Options (before or after the command):
#   --tmpdir DIR    TMPDIR for every child process (default $KIG_TEST_TMPDIR, else
#                   $TMPDIR/kigumi-dev, else ~/.cache/kigumi-dev)
#   --shard I/N     KIGUMI_TEST_SHARD for the corpus loops (default: unsharded)
#   --timeout DUR   go test -timeout (default: quick 20m, full 45m, aot 120m, ct 30m)
#   --              everything after it goes to `go test` verbatim
#
# The runner fails closed: a missing required tool or a too-small TMPDIR is an error, and the AOT
# and ct suites always run against the zig pinned in ZIG_VERSION, never a zig that happens to be
# first on PATH. Optional host tools (qemu-arm, OVMF, the SGX SDK, pnpm) are reported by `env` and
# only gate the checks that need them.
set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd -- "$repo_root"

die() {
	printf 'dev-test: %s\n' "$*" >&2
	exit 1
}

# CI runners use umask 022. Corpus fixtures and the fmt tests create files with 0755/4755 and
# compare the mode back, so a caller's stricter umask (0077 is common) fails them for a reason
# unrelated to the change under test.
umask 022

note() {
	printf 'dev-test: %s\n' "$*" >&2
}

usage() {
	awk 'NR > 1 && /^#/ {sub(/^# ?/, ""); print; next} NR > 1 {exit}' "${BASH_SOURCE[0]}"
}

tmpdir=${KIG_TEST_TMPDIR:-}
shard=${KIG_TEST_SHARD:-}
timeout_opt=
command=
extra=()
while (($#)); do
	case $1 in
	-h | --help)
		usage
		exit 0
		;;
	--tmpdir)
		tmpdir=${2:?"--tmpdir needs a directory"}
		shift 2
		;;
	--tmpdir=*)
		tmpdir=${1#*=}
		shift
		;;
	--shard)
		shard=${2:?"--shard needs I/N"}
		shift 2
		;;
	--shard=*)
		shard=${1#*=}
		shift
		;;
	--timeout)
		timeout_opt=${2:?"--timeout needs a duration"}
		shift 2
		;;
	--timeout=*)
		timeout_opt=${1#*=}
		shift
		;;
	--)
		shift
		extra=("$@")
		break
		;;
	-*)
		die "unknown option: $1 (try --help)"
		;;
	*)
		if [ -n "$command" ]; then
			die "unexpected argument: $1 (put go test flags after --)"
		fi
		command=$1
		shift
		;;
	esac
done
[ -n "$command" ] || {
	usage
	exit 2
}

if [ -z "$tmpdir" ]; then
	if [ -n "${TMPDIR:-}" ]; then
		tmpdir=$TMPDIR/kigumi-dev
	else
		tmpdir=${HOME:?HOME is unset}/.cache/kigumi-dev
	fi
fi

need_go() {
	command -v go >/dev/null 2>&1 || die "missing required tool 'go'; install Go $(gomod_go_version)+"
	local want=${1:-$(gomod_go_version)}
	local got major minor want_major want_minor
	got=$(go env GOVERSION | sed 's/^go//')
	major=${got%%.*}
	minor=${got#*.}
	minor=${minor%%.*}
	want_major=${want%%.*}
	want_minor=${want#*.}
	if ((major < want_major || (major == want_major && minor < want_minor))); then
		die "go $got is older than the go.mod directive $want"
	fi
}

gomod_go_version() {
	awk '/^go /{print $2; exit}' go.mod
}

pinned_zig_version() {
	tr -d '[:space:]' <ZIG_VERSION
}

zig_store_dir() {
	printf '%s/%s\n' "${KIGUMI_ZIG_HOME:-${XDG_DATA_HOME:-$HOME/.local/share}/kigumi/zig}" "$(pinned_zig_version)"
}

# ensure_zig puts the zig pinned in ZIG_VERSION first on PATH, installing it into the managed
# store (the same store CI caches) when it is absent. Never uses sudo and never falls back to a
# different zig on PATH: AOT and constant-time tests report the compiler they lower with.
ensure_zig() {
	need_go
	local pinned dir got
	pinned=$(pinned_zig_version)
	[ -n "$pinned" ] || die "ZIG_VERSION is empty"
	dir=$(zig_store_dir)
	if [ ! -x "$dir/zig" ]; then
		note "pinned zig $pinned is not installed; running 'go run . zig install $pinned'"
		go run . zig install "$pinned" || die "installing zig $pinned failed (network needed once)"
	fi
	[ -x "$dir/zig" ] || die "pinned zig $pinned is missing from $dir"
	got=$("$dir/zig" version) || die "$dir/zig does not run"
	[ "$got" = "$pinned" ] || die "$dir/zig reports $got, want pinned $pinned"
	PATH=$dir:$PATH
	export PATH
	note "using pinned zig $got ($dir)"
}

min_free_mib() {
	case $command in
	check) echo 512 ;;
	quick) echo 1024 ;;
	ct) echo 1024 ;;
	full | aot) echo 4096 ;;
	*) echo 0 ;;
	esac
}

prepare_tmp() {
	mkdir -p -- "$tmpdir" || die "cannot create TMPDIR $tmpdir"
	tmpdir=$(cd -- "$tmpdir" && pwd)
	local want=${KIG_TEST_MIN_FREE_MIB:-$(min_free_mib)}
	if ((want > 0)); then
		command -v df >/dev/null 2>&1 || die "missing required tool 'df'"
		local avail
		avail=$(df -Pk -- "$tmpdir" | awk 'NR==2 {print int($4/1024)}')
		((avail >= want)) ||
			die "TMPDIR $tmpdir has ${avail} MiB free but $command needs ${want} MiB; set --tmpdir or KIG_TEST_TMPDIR"
	fi
	TMPDIR=$tmpdir
	export TMPDIR
	note "TMPDIR $TMPDIR"
}

test_timeout() {
	if [ -n "$timeout_opt" ]; then
		printf '%s\n' "$timeout_opt"
		return
	fi
	case $command in
	quick) echo 20m ;;
	full) echo 45m ;;
	aot) echo 120m ;;
	ct) echo 30m ;;
	*) echo 10m ;;
	esac
}

build_kigumi() {
	mkdir -p -- "$tmpdir/bin"
	local bin=$tmpdir/bin/kigumi
	go build -trimpath -o "$bin" . || die "go build failed"
	printf '%s\n' "$bin"
}

shard_parts() {
	local spec=${shard:-0/1} i n
	i=${spec%%/*}
	n=${spec##*/}
	[[ $spec == */* && $i =~ ^[0-9]+$ && $n =~ ^[1-9][0-9]*$ ]] || die "bad --shard '$spec' (want I/N)"
	((i < n)) || die "--shard index $i is not below the count $n"
	printf '%s %s\n' "$i" "$n"
}

# cmd_env reports what the suite would resolve; it fails only when Go itself is unusable, because
# the optional tools each gate one check.
cmd_env() {
	need_go
	local pinned store path zig_version
	pinned=$(pinned_zig_version)
	store=$(zig_store_dir)
	printf 'repo        %s\n' "$repo_root"
	printf 'commit      %s\n' "$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
	printf 'go.mod      go %s\n' "$(gomod_go_version)"
	printf 'go          %s (%s)\n' "$(go version | awk '{print $3}')" "$(command -v go)"
	printf 'host        %s/%s, %s CPUs\n' "$(go env GOOS)" "$(go env GOARCH)" "$(nproc 2>/dev/null || echo '?')"
	printf 'pinned zig  %s\n' "$pinned"
	if [ -x "$store/zig" ]; then
		printf 'zig store   %s [ok] %s\n' "$store" "$("$store/zig" version)"
	else
		printf 'zig store   %s [MISSING] (aot/ct run "go run . zig install %s")\n' "$store" "$pinned"
	fi
	if path=$(command -v zig 2>/dev/null); then
		zig_version=$("$path" version 2>/dev/null || echo unknown)
		printf 'zig PATH    %s (%s)%s\n' "$path" "$zig_version" \
			"$([ "$zig_version" = "$pinned" ] && echo ' [ok]' || echo ' [ignored: aot/ct prepend the pinned store]')"
	else
		printf 'zig PATH    [absent]\n'
	fi
	local cc
	for cc in clang zig cc gcc; do
		if path=$(command -v "$cc" 2>/dev/null); then
			break
		fi
		path=
	done
	printf 'c compiler  %s\n' "${path:-[MISSING]}"
	printf 'qemu        system-x86_64 %s, arm %s (cross-run, aot)\n' \
		"$([ -n "$(command -v qemu-system-x86_64 2>/dev/null)" ] && echo ok || echo missing)" \
		"$([ -n "$(command -v qemu-arm 2>/dev/null)" ] && echo ok || echo missing)"
	local ovmf=
	for f in /usr/share/ovmf/OVMF.fd /usr/share/OVMF/OVMF_CODE*.fd; do
		[ -f "$f" ] && { ovmf=$f; break; }
	done
	printf 'ovmf        %s (uefi-boot)\n' "${ovmf:-missing}"
	printf 'openssl     %s\n' "$(command -v openssl 2>/dev/null || echo missing)"
	printf 'sgx sdk     sdk %s, enclave_common %s (informational; std/tee tests do not need it)\n' \
		"$([ -d /opt/intel/sgxsdk ] && echo ok || echo missing)" \
		"$([ -e /usr/lib/x86_64-linux-gnu/libsgx_enclave_common.so.1 ] && echo ok || echo missing)"
	printf 'pnpm        %s (editor checks)\n' "$(command -v pnpm 2>/dev/null || echo missing)"
	if mkdir -p -- "$tmpdir" 2>/dev/null; then
		tmpdir=$(cd -- "$tmpdir" && pwd)
		printf 'TMPDIR      %s (%s MiB free, %s needed by full/aot)\n' "$tmpdir" \
			"$(df -Pk -- "$tmpdir" | awk 'NR==2 {print int($4/1024)}')" \
			"${KIG_TEST_MIN_FREE_MIB:-4096}"
	else
		printf 'TMPDIR      %s [cannot create]\n' "$tmpdir"
	fi
	go env GOCACHE >/dev/null 2>&1 || die "go env GOCACHE failed"
	printf 'gocache     %s\n' "$(go env GOCACHE)"
	printf -- '--- kigumi doctor ---\n'
	go run . doctor || die "kigumi doctor failed"
}

cmd_check() {
	need_go
	prepare_tmp
	local unformatted bin
	unformatted=$(gofmt -l .) || die "gofmt failed"
	[ -z "$unformatted" ] || die "gofmt -l lists files that need formatting:"$'\n'"$unformatted"
	bin=$(build_kigumi)
	"$bin" fmt --check std testdata examples || die "kigumi fmt --check failed"
	go vet ./... || die "go vet failed"
	note "check: gofmt, kigumi fmt --check and go vet are clean"
}

cmd_quick() {
	need_go
	prepare_tmp
	local pkgs count
	pkgs=$(go list ./... | grep -v '^kigumi/internal/toolchain/driver$') || true
	[ -n "$pkgs" ] || die "go list returned no packages"
	count=$(printf '%s\n' "$pkgs" | wc -l)
	note "quick: $count packages, hermetic (KIGUMI_TEST_NO_CC=1); the driver corpus is 'aot'/'full'"
	# shellcheck disable=SC2086 # one package per line, split on purpose
	KIGUMI_TEST_NO_CC=1 go test -timeout "$(test_timeout)" $pkgs ${extra[@]+"${extra[@]}"} ||
		die "quick tests failed"
}

cmd_full() {
	need_go
	prepare_tmp
	ensure_zig
	local spec i n
	if [ -n "$shard" ]; then
		spec=$(shard_parts)
		i=${spec% *}
		n=${spec#* }
		KIGUMI_TEST_SHARD=$i/$n
		export KIGUMI_TEST_SHARD
		shard=$i/$n
	fi
	note "full: go test ./... (timeout $(test_timeout)${shard:+, shard $shard})"
	go test -timeout "$(test_timeout)" ./... ${extra[@]+"${extra[@]}"} || die "full tests failed"
}

cmd_aot() {
	need_go
	prepare_tmp
	ensure_zig
	local spec i n sel
	spec=$(shard_parts)
	i=${spec% *}
	n=${spec#* }
	if [ -n "$shard" ]; then
		KIGUMI_TEST_SHARD=$i/$n
		export KIGUMI_TEST_SHARD
	fi
	sel=$(bash .github/scripts/shard_tests.sh "$i" "$n" kigumi/internal/toolchain/driver aot) ||
		die "shard_tests.sh failed"
	note "aot: pinned zig corpus, shard $i/$n${KIGUMI_CFLAGS:+ (KIGUMI_CFLAGS=$KIGUMI_CFLAGS)}"
	go test -timeout "$(test_timeout)" -skip '^(TestLazyStd|TestMutatedMIR)' -run "$sel" \
		./internal/toolchain/driver/ ${extra[@]+"${extra[@]}"} || die "aot tests failed"
	if command -v qemu-arm >/dev/null 2>&1; then
		go test -timeout 10m -run '^TestTarget32CrossBuildRuns$' -count=1 \
			./internal/toolchain/driver/ || die "32-bit cross-run test failed"
	else
		note "skipping TestTarget32CrossBuildRuns: qemu-arm is not on PATH"
	fi
}

cmd_ct() {
	need_go
	prepare_tmp
	ensure_zig
	if [ "${KIGUMI_SECRET_SAN_CORPUS:-}" = 1 ]; then
		note "ct: KIGUMI_SECRET_SAN_CORPUS=1, every corpus fixture is rerun under --secret-san"
	fi
	note "ct: SecretSan and constant-time tests"
	go test -timeout "$(test_timeout)" -run 'Secret|ConstantTime' \
		./internal/backend/vm ./internal/toolchain/driver ${extra[@]+"${extra[@]}"} || die "ct tests failed"
}

case $command in
env) cmd_env ;;
check) cmd_check ;;
quick) cmd_quick ;;
full) cmd_full ;;
aot) cmd_aot ;;
ct) cmd_ct ;;
*) die "unknown command '$command' (want env, check, quick, full, aot or ct)" ;;
esac
