package main

import (
	"fmt"
	"os"

	"github.com/ray-project/rayci/prcheck"
)

func main() {
	code, err := prcheck.Main(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}
