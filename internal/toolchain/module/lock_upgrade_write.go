package module

import (
	"fmt"
	"sort"

	"kigumi/internal/toolchain/version"
)

// LockFormatSupport registers a format/resolver pair and the earliest
// toolchain that can read it. A writer must not save a format newer than the
// project's declared minimum, and must never rewrite an existing lock into an
// older format.
type LockFormatSupport struct {
	Format       int
	Resolver     string
	MinToolchain string
}

// LockWriteDecision is the format a writer may persist, independent of the
// format an in-memory migration used.
type LockWriteDecision struct {
	Format       int
	Resolver     string
	MinToolchain string
}

// ChooseLockWriteFormat selects the newest registered format readable by
// declaredMinToolchain. When no minimum is declared, an existing lock keeps
// its current format; a new lock uses the newest format and reports the
// toolchain it needs. A declared minimum that would force a downgrade is a
// contradiction, not a reason to rewrite the lock.
func ChooseLockWriteFormat(currentFormat int, currentResolver, declaredMinToolchain string, supported []LockFormatSupport) (LockWriteDecision, error) {
	if len(supported) == 0 {
		return LockWriteDecision{}, fmt.Errorf("no lock formats are registered")
	}
	byFormat := make(map[int]LockFormatSupport, len(supported))
	for _, s := range supported {
		if s.Format <= 0 {
			return LockWriteDecision{}, fmt.Errorf("lock format %d must be positive", s.Format)
		}
		if s.Resolver != LockResolverOrderedV1 {
			return LockWriteDecision{}, fmt.Errorf("%w: format %d names %q", ErrLockUnknownResolver, s.Format, s.Resolver)
		}
		if s.MinToolchain == "" {
			return LockWriteDecision{}, fmt.Errorf("lock format %d has no minimum toolchain", s.Format)
		}
		if _, dup := byFormat[s.Format]; dup {
			return LockWriteDecision{}, fmt.Errorf("lock format %d is registered twice", s.Format)
		}
		if _, err := version.AtLeast(s.MinToolchain, s.MinToolchain); err != nil {
			return LockWriteDecision{}, fmt.Errorf("lock format %d minimum toolchain: %v", s.Format, err)
		}
		byFormat[s.Format] = s
	}
	if currentResolver != LockResolverOrderedV1 {
		return LockWriteDecision{}, fmt.Errorf("%w: current resolver %q", ErrLockUnknownResolver, currentResolver)
	}
	current, hasCurrent := LockFormatSupport{}, currentFormat != 0
	if hasCurrent {
		var ok bool
		current, ok = byFormat[currentFormat]
		if !ok {
			return LockWriteDecision{}, fmt.Errorf("current lock format %d is not registered", currentFormat)
		}
		if currentResolver != current.Resolver {
			return LockWriteDecision{}, fmt.Errorf("%w: current resolver %q is not %q", ErrLockUnknownResolver, currentResolver, current.Resolver)
		}
	}
	if declaredMinToolchain == "" {
		if hasCurrent {
			return LockWriteDecision{Format: current.Format, Resolver: current.Resolver, MinToolchain: current.MinToolchain}, nil
		}
		best := supported[0]
		for _, s := range supported[1:] {
			if s.Format > best.Format {
				best = s
			}
		}
		return LockWriteDecision{Format: best.Format, Resolver: best.Resolver, MinToolchain: best.MinToolchain}, nil
	}
	if _, err := version.AtLeast(declaredMinToolchain, declaredMinToolchain); err != nil {
		return LockWriteDecision{}, fmt.Errorf("declared minimum toolchain: %v", err)
	}
	formats := make([]int, 0, len(byFormat))
	for format := range byFormat {
		formats = append(formats, format)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(formats)))
	for _, format := range formats {
		s := byFormat[format]
		ok, err := version.AtLeast(declaredMinToolchain, s.MinToolchain)
		if err != nil {
			return LockWriteDecision{}, err
		}
		if !ok {
			continue
		}
		if s.Format < currentFormat {
			return LockWriteDecision{}, fmt.Errorf("declared minimum toolchain %s can only read lock format %d, older than the existing format %d; refusing to downgrade", declaredMinToolchain, s.Format, currentFormat)
		}
		return LockWriteDecision{Format: s.Format, Resolver: s.Resolver, MinToolchain: s.MinToolchain}, nil
	}
	return LockWriteDecision{}, fmt.Errorf("declared minimum toolchain %s cannot read any registered lock format", declaredMinToolchain)
}
