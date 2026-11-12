#!/usr/bin/env bash
# Print a `go test -run` expression for one shard of a package's top-level tests.
#
# usage: shard_tests.sh <index> <count> <package> [all|vm|aot]
#
# Corpus tests split their fixtures through KIGUMI_TEST_SHARD inside the test binary, so every
# shard runs them. Checks that count the whole corpus run only in shard 0. Every other test is
# assigned to exactly one shard by a hash of its name, which spreads the hundreds of small
# native builds that would otherwise serialize on one runner.
#
# The mode picks which corpus tests every shard runs: `all` (default) runs every corpus test,
# `vm` the ones the VM or the IR backend can run, and `aot` the native-build ones. The hermetic
# job uses `vm` and the AOT matrix uses `aot`, so neither job repeats the other's corpus work.
set -euo pipefail

index=${1:?shard index}
count=${2:?shard count}
pkg=${3:?package}
mode=${4:-all}

# fixture_tests must be exactly the tests that reach fixture.Shards; internal/arch derives that set
# from the test sources and fails when this list drifts from it. aot_tests are the ones that need a
# C toolchain; vm_tests run without one (the VM, the IR backend and native emission), so the
# hermetic job keeps them and the AOT matrix does not repeat them.
aot_tests='TestAotCorpus'
vm_tests='TestArrayAcceleratorsMatchKigumiBodies
TestConstFixturesEmitNatively
TestCorpusOwnershipBeforeVerifier
TestIRDump
TestLazyStdBuildMatchesEager
TestLazyStdCheckMatchesEager
TestRunCorpusEngines
TestRunCorpusVMCoverage
TestSemDriverFixtures
TestStaticFixturesEmitNatively
TestTextAcceleratorsMatchKigumiBodies
TestUnitSweepSemFixtures'
fixture_tests=$aot_tests$'\n'$vm_tests
all_whole_tests='TestRCCallCounts
TestManifestFresh'
whole_tests=$all_whole_tests

case $mode in
  all) ;;
  vm)
    fixture_tests=$vm_tests
    ;;
  aot)
    fixture_tests=$aot_tests
    whole_tests=''
    ;;
  *)
    echo "shard_tests: unknown mode '$mode' (want all, vm or aot)" >&2
    exit 2
    ;;
esac

list=$(go test -list '^Test' "$pkg")
[ -n "$list" ] || { echo "shard_tests: go test -list returned nothing for $pkg" >&2; exit 1; }

regex=$fixture_tests
if [ "$index" -eq 0 ] && [ -n "$whole_tests" ]; then
  regex+=$'\n'$whole_tests
fi
while read -r test; do
  # Corpus tests are selected above by mode; whole-corpus checks only by shard 0.
  if printf '%s\n' "$aot_tests" "$vm_tests" "$all_whole_tests" | grep -qxF "$test"; then
    continue
  fi
  hash=$(printf '%s' "$test" | sha256sum | cut -c1-7)
  if [ $((16#$hash % count)) -eq "$index" ]; then
    regex+=$'\n'"$test"
  fi
done < <(printf '%s\n' "$list" | grep '^Test')

printf '^(%s)$\n' "$(printf '%s' "$regex" | paste -sd'|')"
