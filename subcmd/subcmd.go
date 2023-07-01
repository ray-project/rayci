// Package subcmd provides simple utilities to implement subcommands.
package subcmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/ray-project/rayci/errutil"
)

// Env contains the environment variables that the command runs.
// We use this instead of os.Environ() so that we can capture the environment
// variables, and makes it easier to test.
type Env struct {
	Env map[string]string
}

// Get gets the value of the environment variable. Return empty string if the
// variable is not set.
func (e *Env) Get(key string) string { return e.Env[key] }

// Lookup gets the value of the environment variable. Returns false if the
// variable is not set.
func (e *Env) Lookup(key string) (string, bool) {
	v, ok := e.Env[key]
	return v, ok
}

// Subcmd is a generic subcommand entry.
type Subcmd struct {
	Name string // Name of the sub command.
	Help string // Single line help string.

	// Run is the function that runs the command.
	Run func(name string, args []string, env *Env) error
}

func printHelp(subs []*Subcmd) {
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
func RunMain(env *Env, subs []*Subcmd, args []string) error {
	if args == nil {
		args = os.Args
	}

	if env == nil {
		m := make(map[string]string)
		for _, e := range os.Environ() {
			k, v, ok := strings.Cut(e, "=")
			if !ok {
				m[k] = ""
			}
			m[k] = v
		}
		env = &Env{Env: m}
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
