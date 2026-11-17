//go:build ignore

package main

// 64bdd553 cannot run std/shell in its VM; this pin is the first tested origin commit.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const bootstrapPin = "2f7fdd1e63660bdcf421218a564908fe9b8d2d40"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "bootstrap:", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	repo := os.Getenv("KIGUMI_BOOTSTRAP_REPO")
	if repo == "" {
		repo, err = output("git", "-C", root, "remote", "get-url", "origin")
		if err != nil {
			return fmt.Errorf("find origin: %w", err)
		}
	}
	tmp, err := os.MkdirTemp("", "kigumi-bootstrap-*")
	if err != nil {
		return err
	}
	if os.Getenv("KIGUMI_BOOTSTRAP_KEEP") != "1" {
		defer os.RemoveAll(tmp)
	}

	clone := filepath.Join(tmp, "kigumi")
	if err := runCommand("", "git", "clone", "--quiet", repo, clone); err != nil {
		return err
	}
	if err := runCommand(clone, "git", "checkout", "--quiet", bootstrapPin); err != nil {
		return err
	}
	compiler := filepath.Join(tmp, "kigumi-bin")
	if err := runCommand(clone, "go", "build", "-o", compiler, "."); err != nil {
		return err
	}
	args := append([]string{"run", "--std", filepath.Join(root, "std"), filepath.Join(root, "main.kg")}, os.Args[1:]...)
	return runCommandEnv(root, []string{"KIGUMI_COMPILER=" + compiler}, compiler, args...)
}

func mergeEnv(base, extra []string) []string {
	keys := make(map[string]bool, len(extra))
	for _, kv := range extra {
		keys[strings.SplitN(kv, "=", 2)[0]] = true
	}
	out := make([]string, 0, len(base)+len(extra))
	for _, kv := range base {
		if keys[strings.SplitN(kv, "=", 2)[0]] {
			continue
		}
		out = append(out, kv)
	}
	return append(out, extra...)
}

func output(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	b, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func runCommand(dir, name string, args ...string) error {
	return runCommandEnv(dir, nil, name, args...)
}

func runCommandEnv(dir string, env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = mergeEnv(os.Environ(), env)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
