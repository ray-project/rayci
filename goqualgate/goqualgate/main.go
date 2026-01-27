package main

import (
	"fmt"
	"os"

	"github.com/ray-project/rayci/goqualgate"
)

func main() {
	code, err := goqualgate.Main(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}
