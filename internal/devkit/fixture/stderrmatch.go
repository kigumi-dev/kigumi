package fixture

import (
	"fmt"
	"regexp"
	"strings"
)

// CheckStderr applies a fixture's stderr policy; a nil match means exact
// comparison, so an absent or empty "-- stderr --" section demands empty output.
func CheckStderr(match *StderrMatch, expected, got string) error {
	if match == nil {
		wantTrim, gotTrim := strings.TrimRight(expected, "\n"), strings.TrimRight(got, "\n")
		if wantTrim != gotTrim {
			return fmt.Errorf("stderr mismatch\n--- got ---\n%s--- want ---\n%s", got, expected)
		}
		return nil
	}
	switch match.Type {
	case "ignore":
		return nil
	case "regex":
		re, err := regexp.Compile(match.Pattern)
		if err != nil {
			return fmt.Errorf("bad stderr regex %q: %w", match.Pattern, err)
		}
		if !re.MatchString(got) {
			return fmt.Errorf("stderr %q does not match /%s/", got, match.Pattern)
		}
		return nil
	default:
		return fmt.Errorf("unknown stderr matcher type %q", match.Type)
	}
}
