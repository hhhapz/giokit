//go:build linux && !android

package desktop

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// This file is the file chooser fallback for desktops without the
// FileChooser portal: the zenity or kdialog helper programs.

// commandResult is how a helper program ended.
type commandResult struct {
	stdout, stderr []byte
	code           int // exit code
}

// commands is the seam between the fallback and the processes it starts,
// so tests can stand in for them. The zero value runs real processes.
type commands struct {
	// look finds a program like exec.LookPath.
	look func(name string) (string, error)
	// run runs path with args and returns how it ended. It returns an
	// error only when the program could not run to an exit code (it could
	// not start, or ctx killed it).
	run func(ctx context.Context, path string, args []string) (commandResult, error)
	// getenv reads the environment like os.Getenv.
	getenv func(key string) string
}

// orDefault fills c's unset functions with the real ones.
func (c commands) orDefault() commands {
	if c.look == nil {
		c.look = exec.LookPath
	}
	if c.run == nil {
		c.run = runCommand
	}
	if c.getenv == nil {
		c.getenv = os.Getenv
	}
	return c
}

// runCommand is commands.run for real processes. Ending ctx kills the
// process.
func runCommand(ctx context.Context, path string, args []string) (commandResult, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ee := (*exec.ExitError)(nil); errors.As(err, &ee) && ctx.Err() == nil && ee.Exited() {
		err = nil
	}
	if err != nil {
		return commandResult{}, err
	}
	return commandResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), code: cmd.ProcessState.ExitCode()}, nil
}

// helper is a file chooser program and how to drive it.
type helper struct {
	name string
	open func(FileOptions) []string
	save func(SaveOptions) []string
	// sep separates the paths of a multiple selection. Linux file names
	// may hold any byte but '/' and NUL, so "\n" is ambiguous: with it,
	// helperPaths checks that every path exists.
	sep string
}

// Both helpers exit with 1 when the user cancels.
const helperCancelled = 1

var (
	// zenity takes any separator: "//" never occurs in the clean absolute
	// paths it prints. kdialog only separates with newlines.
	zenity  = helper{name: "zenity", open: zenityOpenArgs, save: zenitySaveArgs, sep: zenitySep}
	kdialog = helper{name: "kdialog", open: kdialogOpenArgs, save: kdialogSaveArgs, sep: "\n"}
)

// helpers returns the helpers to try, in order: zenity then kdialog, or
// kdialog first on KDE Plasma.
func (c commands) helpers() []helper {
	for _, d := range strings.Split(c.getenv("XDG_CURRENT_DESKTOP"), ":") {
		if strings.EqualFold(d, "KDE") {
			return []helper{kdialog, zenity}
		}
	}
	return []helper{zenity, kdialog}
}

// choose runs the first installed helper with the arguments args builds
// for it, and returns the paths it printed: one, or several when multiple.
// portalErr is why the portal could not be used; it is wrapped with
// ErrUnsupported when no helper is installed.
func (c commands) choose(ctx context.Context, portalErr error, multiple bool, args func(helper) []string) ([]string, error) {
	c = c.orDefault()
	for _, h := range c.helpers() {
		path, err := c.look(h.name)
		if err != nil {
			continue
		}
		res, err := c.run(ctx, path, args(h))
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if err != nil {
			return nil, fmt.Errorf("desktop: %s: %w", h.name, err)
		}
		return helperPaths(h, res, multiple)
	}
	return nil, fmt.Errorf("desktop: no file chooser: zenity and kdialog are not installed either: %w: %w", ErrUnsupported, portalErr)
}

// helperPaths interprets how a helper ended: the paths it printed,
// ErrCancelled, or an error. Output ends in one newline. A single path is
// everything before it, newlines included; several are split on h.sep, and
// each must be absolute (and, split on newlines, exist), or the output is
// refused as ambiguous rather than guessed at.
func helperPaths(h helper, res commandResult, multiple bool) ([]string, error) {
	switch res.code {
	case 0:
	case helperCancelled:
		return nil, ErrCancelled
	default:
		return nil, fmt.Errorf("desktop: %s exited with status %d: %s", h.name, res.code, strings.TrimSpace(string(res.stderr)))
	}
	out := strings.TrimSuffix(string(res.stdout), "\n")
	if out == "" {
		return nil, fmt.Errorf("desktop: %s printed no path", h.name)
	}
	if !multiple {
		return []string{out}, nil
	}
	ps := strings.Split(out, h.sep)
	for _, p := range ps {
		if !filepath.IsAbs(p) {
			return nil, fmt.Errorf("desktop: %s: ambiguous output %q: a chosen name may hold the separator", h.name, out)
		}
		if h.sep == "\n" {
			if _, err := os.Lstat(p); err != nil {
				return nil, fmt.Errorf("desktop: %s: ambiguous output: %q does not exist; a chosen name may hold a newline", h.name, p)
			}
		}
	}
	return ps, nil
}

// startPath is the path a save chooser starts at: name inside folder,
// either alone, or "" for neither. A folder alone ends in a separator, so
// that zenity opens it rather than proposing its name.
func startPath(folder, name string) string {
	switch {
	case folder != "" && name != "":
		return filepath.Join(folder, name)
	case folder != "":
		return strings.TrimSuffix(folder, "/") + "/"
	default:
		return name
	}
}

// filterPatterns is f's patterns separated by spaces, or "*" for none.
func filterPatterns(f Filter) string {
	if len(f.Patterns) == 0 {
		return "*"
	}
	return strings.Join(f.Patterns, " ")
}

// zenityFilters returns one --file-filter argument per filter, in
// zenity's "NAME | PATTERN PATTERN" form. zenity splits on '|', so the
// name may not contain one.
func zenityFilters(filters []Filter) []string {
	var args []string
	for _, f := range filters {
		pats := filterPatterns(f)
		name := strings.ReplaceAll(f.Name, "|", " ")
		if name == "" {
			name = pats
		}
		args = append(args, "--file-filter="+name+" | "+pats)
	}
	return args
}

// zenitySep separates the paths zenity prints for a multiple selection.
const zenitySep = "//"

// zenityOpenArgs is zenity's command line for OpenFile.
func zenityOpenArgs(o FileOptions) []string {
	args := []string{"--file-selection", "--separator=" + zenitySep}
	if o.Title != "" {
		args = append(args, "--title="+o.Title)
	}
	if o.Multiple {
		args = append(args, "--multiple")
	}
	if o.Directory {
		return append(args, "--directory")
	}
	return append(args, zenityFilters(o.Filters)...)
}

// zenitySaveArgs is zenity's command line for SaveFile. It asks before
// replacing a file (zenity 4 always does and ignores the flag).
func zenitySaveArgs(o SaveOptions) []string {
	args := []string{"--file-selection", "--save", "--confirm-overwrite"}
	if o.Title != "" {
		args = append(args, "--title="+o.Title)
	}
	if p := startPath(o.Folder, o.Name); p != "" {
		args = append(args, "--filename="+p)
	}
	return append(args, zenityFilters(o.Filters)...)
}

// kdialogFilter is the filter argument for filters in KDE's
// "PATTERNS|NAME" form, one filter per line, or "" for none.
func kdialogFilter(filters []Filter) string {
	lines := make([]string, 0, len(filters))
	for _, f := range filters {
		pats := filterPatterns(f)
		name := strings.ReplaceAll(f.Name, "\n", " ")
		if name == "" {
			name = pats
		}
		lines = append(lines, pats+"|"+name)
	}
	return strings.Join(lines, "\n")
}

// kdialogTitle is kdialog's --title option for title, if set.
func kdialogTitle(title string) []string {
	if title == "" {
		return nil
	}
	return []string{"--title", title}
}

// kdialogOpenArgs is kdialog's command line for OpenFile. It starts in the
// working directory; kdialog chooses one folder at a time, so Multiple
// does not apply to Directory.
func kdialogOpenArgs(o FileOptions) []string {
	args := kdialogTitle(o.Title)
	if o.Directory {
		return append(args, "--getexistingdirectory", ".")
	}
	if o.Multiple {
		args = append(args, "--multiple", "--separate-output")
	}
	args = append(args, "--getopenfilename", ".")
	if f := kdialogFilter(o.Filters); f != "" {
		args = append(args, f)
	}
	return args
}

// kdialogSaveArgs is kdialog's command line for SaveFile.
func kdialogSaveArgs(o SaveOptions) []string {
	start := startPath(o.Folder, o.Name)
	if start == "" {
		start = "."
	}
	args := append(kdialogTitle(o.Title), "--getsavefilename", start)
	if f := kdialogFilter(o.Filters); f != "" {
		args = append(args, f)
	}
	return args
}
