// Package ui wraps the interactive fuzzy finder so the rest of the program does
// not depend on it directly. When stdin is not a TTY (CI, coding agents) the
// helpers return a clear error instead of hanging, nudging callers towards the
// non-interactive flags.
package ui

import (
	"errors"
	"fmt"
	"os"

	"github.com/ktr0731/go-fuzzyfinder"
	"golang.org/x/term"
)

// ErrAborted is returned when the user cancels the selection (Esc / Ctrl-C).
var ErrAborted = errors.New("selection aborted")

// ErrNoTTY is returned when an interactive prompt is needed but stdin is not a
// terminal.
var ErrNoTTY = errors.New("interactive selection requires a terminal; use the non-interactive flags instead")

func interactive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// Select shows a single-choice fuzzy finder over items, using label(item) for
// the displayed text. It returns the chosen index.
func Select[T any](items []T, prompt string, label func(T) string) (int, error) {
	if !interactive() {
		return 0, ErrNoTTY
	}
	idx, err := fuzzyfinder.Find(items,
		func(i int) string { return label(items[i]) },
		fuzzyfinder.WithPromptString(prompt+" "),
	)
	if err != nil {
		if errors.Is(err, fuzzyfinder.ErrAbort) {
			return 0, ErrAborted
		}
		return 0, err
	}
	return idx, nil
}

// SelectMulti shows a multi-choice fuzzy finder (Tab to toggle) and returns the
// chosen indices.
func SelectMulti[T any](items []T, prompt string, label func(T) string) ([]int, error) {
	if !interactive() {
		return nil, ErrNoTTY
	}
	idxs, err := fuzzyfinder.FindMulti(items,
		func(i int) string { return label(items[i]) },
		fuzzyfinder.WithPromptString(prompt+" "),
	)
	if err != nil {
		if errors.Is(err, fuzzyfinder.ErrAbort) {
			return nil, ErrAborted
		}
		return nil, err
	}
	return idxs, nil
}

// Confirm asks a yes/no question on a TTY, defaulting to def when not
// interactive.
func Confirm(prompt string, def bool) bool {
	if !interactive() {
		return def
	}
	suffix := "[y/N]"
	if def {
		suffix = "[Y/n]"
	}
	fmt.Fprintf(os.Stderr, "%s %s ", prompt, suffix)
	var answer string
	if _, err := fmt.Scanln(&answer); err != nil {
		return def
	}
	switch answer {
	case "y", "Y", "yes", "Yes":
		return true
	case "n", "N", "no", "No":
		return false
	default:
		return def
	}
}
