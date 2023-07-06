package subcmd

import (
	"testing"

	"errors"
	"fmt"
)

var (
	errRaining        = errors.New("it seems rainging")
	errShouldFeelSad  = errors.New("it should be raining")
	errWrongGreetings = errors.New("wrong greetings")
)

const keySkyColor = "SKY_COLOR"

func testHello(name string, args []string, env *Env) error {
	if name != "hello" {
		return fmt.Errorf("you say %q?", name)
	}

	if env.Get(keySkyColor) != "blue" {
		return errRaining
	}

	if len(args) != 1 || args[0] != "foo" {
		return errWrongGreetings
	}
	return nil
}

func testBye(name string, args []string, env *Env) error {
	if env.Get(keySkyColor) == "blue" {
		return errShouldFeelSad
	}
	if len(args) != 1 || args[0] != "bar" {
		return errWrongGreetings
	}
	return nil
}

func TestRunMain(t *testing.T) {
	cmdHello := &Subcmd{
		Name: "hello",
		Help: "say hello",
		Run:  testHello,
	}

	cmdBye := &Subcmd{
		Name: "bye",
		Help: "say good bye",
		Run:  testBye,
	}

	cmds := []*Subcmd{cmdHello, cmdBye}

	const bin = "t" // Program binary path; it is always ignored.

	for _, test := range []struct {
		skyColor string
		args     []string
		errWant  error
	}{{
		skyColor: "blue",
		args:     []string{bin, "hello", "foo"},
	}, {
		skyColor: "gray",
		args:     []string{bin, "bye", "bar"},
	}, {
		skyColor: "blue",
		args:     []string{bin, "hello"},
		errWant:  errWrongGreetings,
	}, {
		skyColor: "blue",
		args:     []string{bin, "hello", "bar"},
		errWant:  errWrongGreetings,
	}, {
		skyColor: "blue",
		args:     []string{bin, "bye", "bar"},
		errWant:  errShouldFeelSad,
	}, {
		args:    []string{},
		errWant: ErrInvalidFormat,
	}, {
		args:    []string{bin},
		errWant: ErrInvalidFormat,
	}, {
		args: []string{bin, "help"},
	}, {
		args:    []string{bin, "wada"},
		errWant: ErrUknownCommand,
	}} {
		env := &Env{Env: map[string]string{keySkyColor: test.skyColor}}
		errGot := RunMain(env, cmds, test.args)
		if test.errWant == nil && errGot != nil {
			t.Errorf("run %q got error %s, env=%+v", test.args, errGot, env)
		} else if test.errWant != nil {
			if errGot == nil {
				t.Errorf(
					"run %q got no error, want %q, env=%+v",
					test.args, errGot, env,
				)
			} else if !errors.Is(errGot, test.errWant) {
				t.Errorf(
					"run %q got %q, want %q, env=%+v",
					test.args, errGot, test.errWant, env,
				)
			}
		}
	}
}
