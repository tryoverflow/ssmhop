package picker

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/tryoverflow/ssmhop/internal/target"
)

const fzfBinary = "fzf"

var errFzfUnusable = errors.New("fzf unusable")

func fzfPick(items []target.Target, opts Options) (int, error) {
	path, err := exec.LookPath(fzfBinary)
	if err != nil {
		return 0, errFzfUnusable
	}

	cmd := exec.Command(path, fzfArgs(opts)...)
	cmd.Stdin = strings.NewReader(fzfInput(items))
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			switch ee.ExitCode() {
			case 1:
				return 0, errors.New("no target matched the filter")
			case 130:

				return 0, ErrAborted
			default:

				return 0, errFzfUnusable
			}
		}
		return 0, errFzfUnusable
	}

	return parseFzfSelection(string(out), len(items))
}

func fzfArgs(opts Options) []string {
	return []string{
		"--layout=reverse",
		"--header-first",
		"--header=" + opts.header(),
		"--prompt=? " + opts.prompt(),
		"--query=" + opts.Query,

		"--delimiter=\t",
		"--with-nth=2..",
		"--select-1",
		"--no-multi",
	}
}

func fzfInput(items []target.Target) string {
	var b strings.Builder
	for i, t := range items {
		fmt.Fprintf(&b, "%d\t%s\n", i, t.Label())
	}
	return b.String()
}

func parseFzfSelection(out string, n int) (int, error) {
	line := strings.TrimRight(out, "\r\n")
	if line == "" {
		return 0, ErrAborted
	}
	field, _, ok := strings.Cut(line, "\t")
	if !ok {
		return 0, fmt.Errorf("unexpected fzf output %q", line)
	}
	idx, err := strconv.Atoi(field)
	if err != nil {
		return 0, fmt.Errorf("unexpected fzf index %q", field)
	}
	if idx < 0 || idx >= n {
		return 0, fmt.Errorf("fzf index %d out of range", idx)
	}
	return idx, nil
}
