package main

import (
	"fmt"
	"os"

	"github.com/ray-project/rayci/raycilint"
)

func main() {
	code, err := raycilint.Main(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}
