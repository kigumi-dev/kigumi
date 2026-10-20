package mod

import (
	"sort"

	"github.com/spf13/cobra"

	"kigumi/internal/cli/shared"
	"kigumi/internal/cliutil"
	"kigumi/internal/driver"
	"kigumi/internal/errors"
)

// tidyCmd drops Requires (and their Replace/lock records) nothing imports; it never adds one, since an unknown import has no url to fetch.
func tidyCmd(root *string) *cobra.Command {
	return &cobra.Command{
		Use:   "tidy",
		Short: "Remove Requires that no package imports",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			mf, ok, err := driver.ReadModFile(*root)
			if err != nil {
				return errors.WrapErr(err, "read manifest")
			}
			if !ok {
				return &cliutil.UsageError{Err: errors.NewErrf("%s has no %s", *root, driver.ManifestName)}
			}
			m, err := driver.LoadModule(*root, driver.LoadOptions{Test: true, NoDeps: true, Compiler: shared.CompilerOptions()})
			if err != nil {
				return errors.WrapErr(err, "load module")
			}
			used := m.ImportedModules(mf)
			unused := map[string]bool{}
			for _, r := range mf.Requires {
				if !used[r.Name] {
					unused[r.Name] = true
				}
			}
			if len(unused) == 0 {
				cmd.Println("nothing to remove")
				return nil
			}
			// Read the lock before editing the manifest, not after.
			lock, err := driver.ReadLock(*root)
			if err != nil {
				return errors.WrapErr(err, "read lock")
			}
			for name := range unused {
				delete(lock.Entries, name)
			}
			if err := driver.RemoveRequires(*root, unused); err != nil {
				return errors.WrapErr(err, "edit manifest")
			}
			if err := driver.RemoveLocalReplaces(*root, unused); err != nil {
				return errors.WrapErr(err, "edit local overrides")
			}
			if err := driver.WriteLock(*root, lock); err != nil {
				return errors.WrapErr(err, "write lock")
			}
			for name := range unused {
				cmd.Printf("removed %s\n", name)
			}
			return nil
		},
	}
}

func sortedNames(lock driver.Lock) []string {
	names := make([]string, 0, len(lock.Entries))
	for n := range lock.Entries {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
