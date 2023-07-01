// Package subcmd provides simple utilities to implement subcommands.
package subcmd

import (
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/ray-project/rayci/errutil"
)

// Subcmd is a generic subcommand entry.
type Subcmd[E any] struct {
	Name string // Name of the sub command.
	Help string // Single line help string.

	// Run is the function that runs the command.
	Run func(name string, args []string, env E) error
}

func printHelp[E any](subs []*Subcmd[E]) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	for _, s := range subs {
		fmt.Fprintf(w, "%s\t%s\n", s.Name, s.Help)
	}
	w.Flush()
}

var (
	ErrInvalidFormat = errors.New("invalid format")
	ErrUknownCommand = errors.New("unknown subcommand")
)

// RunMain runs the list of subcommands with the given environment and args.
// if args is nil, then os.Args is used.
func RunMain[E any](env E, subs []*Subcmd[E], args []string) error {
	if args == nil {
		args = os.Args
	}

	if len(args) == 0 {
		return errutil.Wrap(ErrInvalidFormat, "no args found")
	}
	if len(args) <= 1 {
		printHelp(subs)
		return errutil.Wrap(ErrInvalidFormat, "need a subcommand")
	}

	sub := args[1]
	for _, cmd := range subs {
		if cmd.Name == sub {
			return cmd.Run(sub, args[2:], env)
		}
	}

	if sub == "help" {
		printHelp(subs)
		return nil
	}

	printHelp(subs)
	return fmt.Errorf("%w: %q", ErrUknownCommand, sub)
}
