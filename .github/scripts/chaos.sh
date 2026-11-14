#!/usr/bin/env bash
# Run one plan of the nightly chaos sweeps, locally or in CI.
#
# usage: chaos.sh [options]
#
#   --list              print the planned entries as "<suite> <shard> <count> <test> <toolchain>"
#   --plan              print the same plan as a JSON array (the CI matrix consumes this)
#   --print-seed        print the resolved base seed and nothing else
#   --dry-run           print each entry's go test command instead of running it
#   --suite S           only the entries of suite S (mir, sem, alloc, alloc-native)
#   --shard I           only shard I of the plan
#   --shards N          shard count (default 4)
#   --seed S            base seed; unset derives one from GITHUB_RUN_ID or the clock
#   --seconds N         KIGUMI_MIR_FUZZ_SECONDS for the mir suite (default 600)
#   --count N           KIGUMI_MIR_FUZZ_COUNT for the mir suite (default 4)
#                       (a floor: the mir test also runs until --seconds elapses)
#   --timeout N         go test -timeout in seconds (default 1800)
#   --out DIR           log directory (default chaos-logs)
#   --replay SPEC       run one recorded finding instead of a plan: SPEC is <base>:<seed>
#
# The default plan is the seeded MIR sweep plus the two allocation fault-injection sweeps.
# The sem suite (TestUnitSweepSemFixtures) is opt-in through --suite sem: it is deterministic
# and already gated by every CI push, and its mutation sweep costs far more than a nightly
# shard budget allows. It stays available for a focused, unattended run.
#
# Suites are deterministic per seed, so a run is replayable from its seed. The mir suite draws a
# new FROM seed per (base seed, shard); Ctrl-C or a failing go test never leaves a shard's log
# without the seed that produced it. Every default suite runs against the VM/IR path only, so
# the runner needs no C toolchain: alloc-native is the exception and is the one entry with
# toolchain zig.
#
# A shard's log ends with its replay block. For a mir finding the test itself prints the exact
# `KIGUMI_MIR_FUZZ_REPLAY=<fixture>:<seed>` line to paste into the replay command.
set -euo pipefail

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$root"

suites=(mir alloc alloc-native)
known_suites=(mir sem alloc alloc-native)
shards=4
base_seed=""
seconds=600
mutants=4
timeout=1800
out=chaos-logs
action=run
filter_suite=""
filter_shard=""
replay_spec=""

usage() {
  sed -n '2,29p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

die() {
  echo "chaos.sh: $*" >&2
  exit 2
}

suite_test() {
  case $1 in
    mir) echo TestMutatedMIR ;;
    sem) echo TestUnitSweepSemFixtures ;;
    alloc) echo TestFaultInjectionSweepVM ;;
    alloc-native) echo TestFaultInjectionSweepNative ;;
    *) die "unknown suite '$1' (want one of: ${known_suites[*]})" ;;
  esac
}

suite_toolchain() {
  case $1 in
    alloc-native) echo zig ;;
    *) echo go ;;
  esac
}

# sharded suites slice their corpus through fixture.Shards/ShardPaths, so every shard runs a
# disjoint slice. alloc* compile one embedded program and ignore KIGUMI_TEST_SHARD, so they run
# once, in shard 0, instead of paying the same work in every shard.
sharded_suite() {
  case $1 in
    mir | sem) return 0 ;;
    *) return 1 ;;
  esac
}

require_uint() { # name value
  case $2 in
    '' | *[!0-9]*) die "$1 wants a non-negative integer, got '$2'" ;;
  esac
}

while [ $# -gt 0 ]; do
  case $1 in
    --list | --plan | --print-seed | --dry-run)
      action=${1#--}
      ;;
    --suite | --shard | --shards | --seed | --seconds | --count | --timeout | --out | --replay)
      opt=$1
      [ $# -ge 2 ] || die "$opt wants a value"
      case $opt in
        --suite) filter_suite=$2 ;;
        --shard) filter_shard=$2 ;;
        --shards) shards=$2 ;;
        --seed) base_seed=$2 ;;
        --seconds) seconds=$2 ;;
        --count) mutants=$2 ;;
        --timeout) timeout=$2 ;;
        --out) out=$2 ;;
        --replay) replay_spec=$2 ;;
      esac
      shift
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      die "unknown argument '$1' (try --help)"
      ;;
  esac
  shift
done

for pair in "shards:$shards" "seconds:$seconds" "count:$mutants" "timeout:$timeout"; do
  require_uint "${pair%%:*}" "${pair#*:}"
done
[ -z "$base_seed" ] || require_uint seed "$base_seed"
[ -z "$filter_shard" ] || require_uint shard "$filter_shard"
[ "$shards" -ge 1 ] || die "--shards wants at least 1, got $shards"
[ "$timeout" -ge 1 ] || die "--timeout wants at least 1 second, got $timeout"

if [ -n "$replay_spec" ]; then
  # The test parses <fixture>:<seed> itself and prints the mutant recipe plus the failure.
  exec env KIGUMI_MIR_FUZZ_REPLAY="$replay_spec" \
    go test -count=1 -timeout "${timeout}s" -run '^TestMutatedMIR$' -v ./internal/toolchain/driver
fi

if [ -n "$filter_shard" ]; then
  [ "$filter_shard" -ge "$shards" ] && die "--shard $filter_shard is out of range for --shards $shards"
  if [ -n "$filter_suite" ] && [ "$filter_shard" != 0 ] && ! sharded_suite "$filter_suite"; then
    die "suite $filter_suite runs once in shard 0, not shard $filter_shard"
  fi
fi

if [ -n "$filter_suite" ]; then
  found=0
  for suite in "${known_suites[@]}"; do [ "$suite" = "$filter_suite" ] && found=1; done
  [ "$found" = 1 ] || die "unknown --suite '$filter_suite' (want one of: ${known_suites[*]})"
fi

if [ -z "$base_seed" ]; then
  # One seed per run, stable within it: GITHUB_RUN_ID in CI, the clock locally. Either source is
  # hashed so a seed is a small number that fits KIGUMI_MIR_FUZZ_FROM and every shell.
  if [ -n "${GITHUB_RUN_ID:-}" ]; then origin="run:${GITHUB_RUN_ID}"; else origin="local:$(date -u +%s)"; fi
  base_seed=$((16#$(printf '%s' "$origin" | sha256sum | cut -c1-8)))
fi
[ "$base_seed" -gt 0 ] || base_seed=1

derive_seed() { # suite shard -> per-entry seed
  printf '%d' "$((16#$(printf '%s:%s:%s' "$base_seed" "$1" "$2" | sha256sum | cut -c1-8)))"
}

in_scope() { # suite shard
  [ -z "$filter_suite" ] || [ "$filter_suite" = "$1" ] || return 1
  [ -z "$filter_shard" ] || [ "$filter_shard" = "$2" ] || return 1
  return 0
}

plan_entries() {
  local suite shard list=("${suites[@]}")
  if [ -n "$filter_suite" ]; then list=("$filter_suite"); fi
  for suite in "${list[@]}"; do
    if sharded_suite "$suite"; then
      for ((shard = 0; shard < shards; shard++)); do
        in_scope "$suite" "$shard" && printf '%s %s\n' "$suite" "$shard"
      done
    else
      in_scope "$suite" 0 && printf '%s %s\n' "$suite" 0
    fi
  done
}

mapfile -t entries < <(plan_entries)
[ "${#entries[@]}" -gt 0 ] || die "no chaos entry matches suite='$filter_suite' shard='$filter_shard' (shards=$shards)"

entry_env() { # suite shard -> "VAR=VALUE" words
  printf 'KIGUMI_TEST_SHARD=%s/%s\n' "$2" "$shards"
  if [ "$1" = mir ]; then
    printf 'KIGUMI_MIR_FUZZ_COUNT=%s\n' "$mutants"
    printf 'KIGUMI_MIR_FUZZ_SECONDS=%s\n' "$seconds"
    printf 'KIGUMI_MIR_FUZZ_FROM=%s\n' "$(derive_seed "$1" "$2")"
  fi
}

entry_cmd() { # suite shard
  local test
  test=$(suite_test "$1")
  printf "go test -count=1 -v -timeout %ss -run '^%s\$' %s" "$timeout" "$test" ./internal/toolchain/driver
}

replay_block() { # suite shard
  local suite=$1 shard=$2 test seed
  test=$(suite_test "$suite")
  seed=$(derive_seed "$suite" "$shard")
  echo "# chaos entry: suite=$suite shard=$shard/$shards test=$test toolchain=$(suite_toolchain "$suite")"
  echo "# base seed: $base_seed  entry seed: $seed"
  echo "# replay (from the repository root):"
  echo "#   $(entry_env "$suite" "$shard" | paste -sd' ' -) $(entry_cmd "$suite" "$shard")"
  if [ "$suite" = mir ]; then
    echo "# a finding prints its own KIGUMI_MIR_FUZZ_REPLAY=<fixture>:<seed> line; paste it:"
    echo "#   KIGUMI_MIR_FUZZ_REPLAY=<fixture>:<seed> go test -count=1 -run '^TestMutatedMIR\$' -v ./internal/toolchain/driver"
  fi
}

case $action in
  list)
    printf '%-12s %-5s %-5s %-34s %s\n' suite shard count test toolchain
    for entry in "${entries[@]}"; do
      read -r suite shard <<<"$entry"
      printf '%-12s %-5s %-5s %-34s %s\n' "$suite" "$shard" "$shards" "$(suite_test "$suite")" "$(suite_toolchain "$suite")"
    done
    exit 0
    ;;
  plan)
    printf '['
    first=1
    for entry in "${entries[@]}"; do
      read -r suite shard <<<"$entry"
      [ "$first" = 1 ] || printf ','
      first=0
      printf '{"suite":"%s","shard":%s,"count":%s,"seed":%s,"test":"%s","toolchain":"%s","seconds":%s,"mutants":%s,"timeout":%s}' \
        "$suite" "$shard" "$shards" "$(derive_seed "$suite" "$shard")" "$(suite_test "$suite")" "$(suite_toolchain "$suite")" \
        "$seconds" "$mutants" "$timeout"
    done
    printf ']\n'
    exit 0
    ;;
  print-seed)
    printf '%s\n' "$base_seed"
    exit 0
    ;;
  dry-run)
    for entry in "${entries[@]}"; do
      read -r suite shard <<<"$entry"
      replay_block "$suite" "$shard" | sed -n 's/^#   //p' | head -n 1
    done
    exit 0
    ;;
esac

mkdir -p "$out"
failed=0
declare -a rows=() details=()
for entry in "${entries[@]}"; do
  read -r suite shard <<<"$entry"
  dir="$out/$suite-shard$shard"
  mkdir -p "$dir"
  replay_block "$suite" "$shard" >"$dir/replay.txt"
  mapfile -t envs < <(entry_env "$suite" "$shard")
  # The command is assembled once so --dry-run and the real run cannot drift.
  cmd=$(entry_cmd "$suite" "$shard")
  echo "chaos: $suite shard $shard/$shards: $(entry_env "$suite" "$shard" | paste -sd' ' -) $cmd"
  if bash -c "env $(printf '%q ' "${envs[@]}") $cmd" 2>&1 | tee "$dir/test.log"; then
    status=PASS
  else
    status=FAIL
    failed=1
  fi
  echo "$status $suite shard $shard/$shards (seed $(derive_seed "$suite" "$shard"))"
  # shellcheck disable=SC2016  # the backticks are markdown, not expansions
  rows+=("$(printf '| %s | %s/%s | %s | `%s` |' "$suite" "$shard" "$shards" "$status" "$dir/replay.txt")")
  # shellcheck disable=SC2016
  details+=("$(printf '### %s shard %s/%s: %s\n\n```\n%s\n```\n' "$suite" "$shard" "$shards" "$status" "$(cat "$dir/replay.txt")")")
done

summary="$out/summary.md"
{
  echo '## nightly chaos'
  echo
  echo "base seed: \`$base_seed\` — shards: $shards, mir seconds: $seconds, mir mutants/base: $mutants"
  echo
  echo '| suite | shard | status | replay |'
  echo '|---|---|---|---|'
  printf '%s\n' "${rows[@]}"
  echo
  printf '%s\n' "${details[@]}"
} >"$summary"

if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
  cat "$summary" >>"$GITHUB_STEP_SUMMARY"
fi
echo
cat "$summary"
[ "$failed" = 0 ] || exit 1
