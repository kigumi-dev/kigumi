#!/usr/bin/env bash
# Print a `go test -run` expression for one shard of a package's top-level tests.
#
# usage: shard_tests.sh <index> <count> <package>
#
# Corpus tests split their fixtures through KIGUMI_TEST_SHARD inside the test binary, so every
# shard runs them. Checks that count the whole corpus run only in shard 0. Every other test is
# assigned to exactly one shard by a hash of its name, which spreads the hundreds of small
# native builds that would otherwise serialize on one runner.
set -euo pipefail

index=${1:?shard index}
count=${2:?shard count}
pkg=${3:?package}

# Keep these lists in sync with the tests that call fixture.Shard; a name must exist in the package.
fixture_tests='TestAotCorpus
TestRunCorpusEngines
TestSemDriverFixtures
TestTextAcceleratorsMatchKigumiBodies
TestArrayAcceleratorsMatchKigumiBodies'
whole_tests='TestRCCallCounts
TestCorpusOwnershipBeforeVerifier
TestManifestFresh'

list=$(go test -list '^Test' "$pkg")
[ -n "$list" ] || { echo "shard_tests: go test -list returned nothing for $pkg" >&2; exit 1; }

regex=$fixture_tests
if [ "$index" -eq 0 ]; then
  regex+=$'\n'$whole_tests
fi
while read -r test; do
  if printf '%s\n' "$fixture_tests" | grep -qxF "$test"; then
    continue
  fi
  if printf '%s\n' "$whole_tests" | grep -qxF "$test"; then
    continue
  fi
  hash=$(printf '%s' "$test" | sha256sum | cut -c1-7)
  if [ $((16#$hash % count)) -eq "$index" ]; then
    regex+=$'\n'"$test"
  fi
done < <(printf '%s\n' "$list" | grep '^Test')

printf '^(%s)$\n' "$(printf '%s' "$regex" | paste -sd'|')"
