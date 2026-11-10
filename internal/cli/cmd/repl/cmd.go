// Package repl implements `kigumi repl` / kgsh.
package repl

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"

	"github.com/spf13/cobra"

	"kigumi/internal/backend/vm"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/toolchain/driver"
	toolrepl "kigumi/internal/tooling/repl"
)

type options struct {
	forceInteractive bool
	stdinCells       bool
	command          string
	login            bool
	noRC             bool
	noHistory        bool
}

// New returns the repl command.
func New() *cobra.Command {
	var opts options
	cmd := &cobra.Command{
		Use:                   "repl [flags]",
		DisableFlagsInUseLine: true,
		Short:                 "Evaluate kigumi source interactively or as kgsh",
		Args:                  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd, opts)
		},
	}
	cmd.Flags().BoolVarP(&opts.forceInteractive, "interactive", "i", false, "treat input as interactive even when stdin is not a tty")
	cmd.Flags().BoolVarP(&opts.stdinCells, "stdin", "s", false, "read cells from stdin without a prompt")
	cmd.Flags().StringVarP(&opts.command, "command", "c", "", "evaluate one kigumi source string and exit")
	cmd.Flags().BoolVarP(&opts.login, "login", "l", false, "read /etc/kigumirc and ~/.kigumirc")
	cmd.Flags().BoolVar(&opts.noRC, "no-rc", false, "do not read rc files")
	cmd.Flags().BoolVar(&opts.noHistory, "no-history", false, "do not read or write history")
	return cmd
}

func run(cmd *cobra.Command, opts options) error {
	ioh := session.NewIO(cmd)
	s := session.Read(cmd)
	tgt, err := s.Target()
	if err != nil {
		return &exit.UsageError{Err: err}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	root := cwd
	compiler := session.CompilerOptions()
	compiler.Cwd = root
	r, err := driver.NewRepl(root, driver.ReplOptions{
		Load: driver.LoadOptions{StdRoot: s.StdRoot(), Target: tgt, Compiler: compiler, Policy: s.Policy(), NoLocal: s.NoLocal},
		VM:   vm.Options{Args: []string{"kgsh"}, Cwd: root, Stdin: os.Stdin},
		Out:  ioh.Out,
		Err:  ioh.Err,
	})
	if err != nil {
		return err
	}
	defer r.Close()

	if !opts.noRC && (opts.login || isInteractive(os.Stdin, opts.forceInteractive)) {
		readRC(r, ioh, opts.login)
	}
	if opts.command != "" {
		if inc, reason := syntax.Incomplete([]byte(opts.command)); inc {
			return fmt.Errorf("repl: incomplete command: %s", reason)
		}
		res, err := r.Eval(opts.command, true)
		if err != nil {
			return err
		}
		printResult(ioh, res, true)
		if res.Exit {
			return exit.Code(res.ExitCode)
		}
		if res.Status != 0 {
			return exit.Code(res.Status)
		}
		return nil
	}
	if opts.stdinCells || !isInteractive(os.Stdin, opts.forceInteractive) {
		return runStdin(r, ioh, opts)
	}
	return runInteractive(r, ioh, opts)
}

func isInteractive(f *os.File, force bool) bool {
	if force {
		return true
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func runStdin(r *driver.Repl, ioh session.IO, opts options) error {
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var pending []string
	flush := func() error {
		if len(pending) == 0 {
			return nil
		}
		src := strings.Join(pending, "\n")
		pending = nil
		res, err := r.Eval(src, false)
		if err != nil {
			return err
		}
		if res.Incomplete {
			return fmt.Errorf("repl: incomplete input: %s", res.Message)
		}
		if res.Message != "" {
			fmt.Fprintln(ioh.Err, res.Message)
		}
		if res.Exit {
			return exit.Code(res.ExitCode)
		}
		if res.Status != 0 {
			return exit.Code(res.Status)
		}
		return nil
	}
	for sc.Scan() {
		line := sc.Text()
		if len(pending) == 0 {
			if handled, err := shellLine(line, ioh, nil); handled {
				if err != nil {
					fmt.Fprintln(ioh.Err, err)
					return exit.Code(1)
				}
				continue
			}
		}
		pending = append(pending, line)
		if inc, _ := syntax.Incomplete([]byte(strings.Join(pending, "\n"))); inc {
			continue
		}
		if err := flush(); err != nil {
			return err
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return flush()
}

func runInteractive(r *driver.Repl, ioh session.IO, opts options) error {
	histPath := ""
	if !opts.noHistory {
		histPath = toolrepl.DefaultHistoryPath()
	}
	hist, err := toolrepl.NewHistory(histPath)
	if err != nil {
		return err
	}
	ed, err := toolrepl.NewEditor(os.Stdin, ioh.Out, hist)
	if err != nil {
		return err
	}
	defer ed.Close()
	if ed.IsTerminal() {
		ed.SetCompleter(r.Completions)
	}
	sigc := make(chan os.Signal, 4)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGWINCH)
	stop := make(chan struct{})
	var evaluating atomic.Bool
	go func() {
		for {
			select {
			case <-stop:
				return
			case sig := <-sigc:
				switch sig {
				case os.Interrupt:
					if evaluating.Load() {
						r.Cancel()
					}
				case syscall.SIGHUP:
					ed.Close()
					os.Exit(0)
				case syscall.SIGTERM:
					ed.Close()
					os.Exit(1)
				}
			}
		}
	}()
	defer func() {
		signal.Stop(sigc)
		close(stop)
	}()
	var pending []string
	for {
		prompt := "kgsh> "
		if len(pending) > 0 {
			prompt = "   ...> "
		}
		line, err := ed.ReadLine(prompt)
		if errors.Is(err, toolrepl.ErrInterrupted) {
			pending = nil
			continue
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if len(pending) == 0 {
			if handled, err := shellLine(line, ioh, ed); handled {
				if err != nil {
					fmt.Fprintln(ioh.Err, err)
				}
				continue
			}
		}
		pending = append(pending, line)
		src := strings.Join(pending, "\n")
		if inc, _ := syntax.Incomplete([]byte(src)); inc {
			continue
		}
		evaluating.Store(true)
		res, err := r.Eval(src, true)
		evaluating.Store(false)
		if err != nil {
			fmt.Fprintln(ioh.Err, err)
			pending = nil
			continue
		}
		if res.Incomplete {
			continue
		}
		pending = nil
		printResult(ioh, res, true)
		if res.Exit {
			return exit.Code(res.ExitCode)
		}
	}
}

func printResult(ioh session.IO, res driver.EvalResult, display bool) {
	if res.Message != "" {
		fmt.Fprintln(ioh.Err, res.Message)
	}
	if display && res.Display != "" {
		fmt.Fprintln(ioh.Out, res.Display)
	}
}

func readRC(r *driver.Repl, ioh session.IO, login bool) {
	var files []string
	if login {
		files = append(files, "/etc/kigumirc")
	}
	if home, err := os.UserHomeDir(); err == nil {
		files = append(files, filepath.Join(home, ".kigumirc"))
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(strings.NewReader(string(data)))
		var pending []string
		for sc.Scan() {
			pending = append(pending, sc.Text())
			if inc, _ := syntax.Incomplete([]byte(strings.Join(pending, "\n"))); inc {
				continue
			}
			src := strings.Join(pending, "\n")
			pending = nil
			res, err := r.Eval(src, false)
			if err != nil {
				fmt.Fprintln(ioh.Err, err)
				continue
			}
			if res.Message != "" {
				fmt.Fprintln(ioh.Err, res.Message)
			}
			if res.Exit {
				return
			}
		}
	}
}

// shellLine handles the few commands a login shell provides outside the
// language. cd/export/unset/exit/pwd are translated by the driver; which and
// exec need process tools and stay here.
func shellLine(line string, ioh session.IO, ed *toolrepl.Editor) (bool, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return false, nil
	}
	switch fields[0] {
	case "which":
		if len(fields) != 2 {
			return true, errors.New("which: expected one name")
		}
		p, err := exec.LookPath(fields[1])
		if err != nil {
			return true, err
		}
		fmt.Fprintln(ioh.Out, p)
		return true, nil
	case "exec":
		if len(fields) < 2 {
			return true, errors.New("exec: expected a program")
		}
		p, err := exec.LookPath(fields[1])
		if err != nil {
			return true, err
		}
		if ed != nil {
			ed.Close()
		}
		return true, syscall.Exec(p, fields[1:], os.Environ())
	}
	return false, nil
}
