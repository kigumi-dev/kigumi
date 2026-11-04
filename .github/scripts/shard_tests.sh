#!/usr/bin/env bash
# Print a `go test -run` expression for one shard of a package's top-level tests.
#
# usage: shard_tests.sh <index> <count> <package>
#
# Corpus tests are split by KIGUMI_TEST_SHARD inside the test binary, so they run in every
# shard (each takes its slice). The few checks that count the whole corpus run only in
# shard 0. Every other top-level test is assigned to one shard by a hash of its name, which
# spreads the hundreds of small native builds that would otherwise serialize on one runner.
set -euo pipefail

index=${1:?shard index}
count=${2:?shard count}
pkg=${3:?package}

# Corpus loops that honour KIGUMI_TEST_SHARD (see internal/devkit/fixture.Shard).
fixture_tests='TestAotCorpus|TestRunCorpusEngines|TestSemDriverFixtures|TestTextAcceleratorsMatchKigumiBodies|TestArrayAcceleratorsMatchKigumiBodies'
# Checks that read the whole corpus; they cannot be split, so shard 0 runs them.
whole_tests='TestRCCallCounts|TestCorpusOwnershipBeforeVerifier|TestRunCorpusOnVMLive|TestManifestFresh'

regex=$fixture_tests
if [ "$index" -eq 0 ]; then
  regex+="|$whole_tests"
fi
while read -r test; do
  case $test in
    $fixture_tests | $whole_tests) continue ;;
  esac
  hash=$(printf '%s' "$test" | sha256sum | cut -c1-7)
  if [ $((16#$hash % count)) -eq "$index" ]; then
    regex+="|$test"
  fi
done < <(go test -list '^Test' "$pkg" | grep '^Test')

printf '^(%s)$\n' "$regex"
