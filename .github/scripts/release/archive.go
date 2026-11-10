package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// archivePath is one entry to write. The content is read from disk so a
// symlink is stored as a link, never followed.
type archivePath struct {
	name string // slash-separated path inside the archive, before the prefix
	disk string // absolute or root-relative path to read
}

// runArchive writes a byte-for-byte reproducible tar.gz: entries are sorted,
// ownership is zeroed, timestamps are the Unix epoch and gzip carries no name
// or mtime. Reproducibility is the point of the source archive, so nothing in
// the header may come from the build machine.
func runArchive(args []string) error {
	fs := newFlagSet("archive")
	root := fs.String("root", ".", "directory that paths are relative to")
	out := fs.String("out", "", "output tar.gz")
	prefix := fs.String("prefix", "", "directory prefix inside the archive")
	fromGit := fs.Bool("git", false, "include exactly the files tracked by git")
	all := fs.Bool("all", false, "include every regular file and symlink under root")
	if err := parse(fs, args); err != nil {
		return err
	}
	if *out == "" {
		return fmt.Errorf("archive: --out is required")
	}
	if *fromGit && *all {
		return fmt.Errorf("archive: --git and --all are mutually exclusive")
	}
	if !*fromGit && !*all && fs.NArg() == 0 {
		return fmt.Errorf("archive: give --git, --all or file paths")
	}
	entries, err := collectEntries(*root, *fromGit, *all, fs.Args())
	if err != nil {
		return err
	}
	return writeArchive(*out, *prefix, entries)
}

func collectEntries(root string, fromGit, all bool, paths []string) ([]archivePath, error) {
	var names []string
	switch {
	case fromGit:
		list, err := gitTrackedFiles(root)
		if err != nil {
			return nil, err
		}
		names = list
	case all:
		err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if p == root {
				return nil
			}
			if d.IsDir() {
				if d.Name() == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			names = append(names, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			return nil, err
		}
	default:
		for _, p := range paths {
			clean := path.Clean(filepath.ToSlash(p))
			if clean == ".." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) {
				return nil, fmt.Errorf("archive: path %q escapes the root", p)
			}
			names = append(names, clean)
		}
	}
	sort.Strings(names)
	seen := map[string]bool{}
	entries := make([]archivePath, 0, len(names))
	for _, name := range names {
		if name == "" || name == "." || seen[name] {
			continue
		}
		seen[name] = true
		entries = append(entries, archivePath{name: name, disk: filepath.Join(root, filepath.FromSlash(name))})
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("archive: no files selected")
	}
	return entries, nil
}

func gitTrackedFiles(root string) ([]string, error) {
	cmd := exec.Command("git", "-C", root, "ls-files", "-z", "--cached")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	var names []string
	for _, name := range strings.Split(string(out), "\x00") {
		if name == "" {
			continue
		}
		names = append(names, filepath.ToSlash(name))
	}
	return names, nil
}

func writeArchive(out, prefix string, entries []archivePath) error {
	if dir := filepath.Dir(out); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	zw, err := gzip.NewWriterLevel(f, gzip.BestCompression)
	if err != nil {
		return err
	}
	zw.Header.ModTime = time.Unix(0, 0)
	zw.Header.OS = 255
	tw := tar.NewWriter(zw)
	for _, entry := range entries {
		if err := writeEntry(tw, prefix, entry); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return zw.Close()
}

func writeEntry(tw *tar.Writer, prefix string, entry archivePath) error {
	info, err := os.Lstat(entry.disk)
	if err != nil {
		return fmt.Errorf("archive: %s: %w", entry.name, err)
	}
	name := entry.name
	if prefix != "" {
		name = path.Join(prefix, name)
	}
	hdr := &tar.Header{
		Name:    name,
		Mode:    0o644,
		ModTime: time.Unix(0, 0),
		Uid:     0,
		Gid:     0,
		Format:  tar.FormatPAX,
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(entry.disk)
		if err != nil {
			return fmt.Errorf("archive: %s: %w", entry.name, err)
		}
		hdr.Typeflag = tar.TypeSymlink
		hdr.Linkname = target
		hdr.Mode = 0o777
		hdr.Size = 0
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		return nil
	case info.Mode().IsRegular():
		if info.Mode()&0o111 != 0 {
			hdr.Mode = 0o755
		}
		hdr.Typeflag = tar.TypeReg
		hdr.Size = info.Size()
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		src, err := os.Open(entry.disk)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tw, src)
		closeErr := src.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	default:
		return fmt.Errorf("archive: %s is neither a regular file nor a symlink", entry.name)
	}
}
