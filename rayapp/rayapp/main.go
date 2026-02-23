package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ray-project/rayci/rayapp"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	buildFlags := flag.NewFlagSet("build", flag.ExitOnError)
	base := buildFlags.String("base", ".", "base directory")
	output := buildFlags.String("output", "_build", "output directory")
	buildFile := buildFlags.String("build", "BUILD.yaml", "build file")

	testFlags := flag.NewFlagSet("test", flag.ExitOnError)
	testBuildFile := testFlags.String("build", "BUILD.yaml", "build file")

	probeFlags := flag.NewFlagSet("probe", flag.ExitOnError)

	switch os.Args[1] {
	case "build":
		buildFlags.Parse(os.Args[2:])
		args := buildFlags.Args()
		if len(args) < 1 {
			log.Fatal("build requires a template name or 'all'")
		}
		if args[0] == "all" {
			if err := rayapp.BuildAll(*buildFile, *base, *output); err != nil {
				log.Fatal(err)
			}
		} else {
			if err := rayapp.Build(*buildFile, args[0], *base, *output); err != nil {
				log.Fatal(err)
			}
		}
	case "test":
		testFlags.Parse(os.Args[2:])
		args := testFlags.Args()
		if len(args) < 1 {
			log.Fatal("test requires <template-name> or 'all'")
		}
		if args[0] == "all" {
			if err := rayapp.TestAll(*testBuildFile); err != nil {
				log.Fatal(err)
			}
		} else {
			if err := rayapp.Test(args[0], *testBuildFile); err != nil {
				log.Fatal(err)
			}
		}
	case "probe":
		probeFlags.Parse(os.Args[2:])
		args := probeFlags.Args()
		if len(args) < 1 {
			log.Fatal("probe requires <template-name>")
		}
		if err := rayapp.Probe(args[0]); err != nil {
			log.Fatal(err)
		}
	case "help":
		printUsage()
	default:
		log.Fatalf("unknown command: %s", os.Args[1])
	}
}

func printUsage() {
	fmt.Println("Usage: rayapp <command> [flags]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  build <template-name|all>  Build a template or all templates")
	fmt.Println("  test  <template-name|all>  Test a template or all templates")
	fmt.Println("  help                       Show this help message")
	fmt.Println()
	fmt.Println("Build flags (build):")
	fmt.Println("  --base string      Base directory (default \".\")")
	fmt.Println("  --output string    Output directory (default \"_build\")")
	fmt.Println("  --build string     Build file (default \"BUILD.yaml\")")
}
