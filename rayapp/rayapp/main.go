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

	// Build command flags (shared by build and build-all)
	buildFlags := flag.NewFlagSet("build", flag.ExitOnError)
	base := buildFlags.String("base", ".", "base directory")
	output := buildFlags.String("output", "_build", "output directory")
	buildFile := buildFlags.String("build", "BUILD.yaml", "build file")

	// Test command flags
	testFlags := flag.NewFlagSet("test", flag.ExitOnError)
	testBuildFile := testFlags.String("build", "BUILD.yaml", "build file")
	// workspaceName := testFlags.String("workspace-name", "", "workspace name (required)")
	// templateDir := testFlags.String("template-dir", "", "template directory (required)")
	// config := testFlags.String("config", "config.yml", "config file path (required)")

	switch os.Args[1] {
	case "build-all":
		buildFlags.Parse(os.Args[2:])
		if err := rayapp.BuildAll(*buildFile, *base, *output); err != nil {
			log.Fatal(err)
		}
	case "build":
		buildFlags.Parse(os.Args[2:])
		args := buildFlags.Args()
		if len(args) < 1 {
			log.Fatal("build requires a template name")
		}
		if err := rayapp.Build(*buildFile, args[0], *base, *output); err != nil {
			log.Fatal(err)
		}
	case "test":
		testFlags.Parse(os.Args[2:])
		args := testFlags.Args()
		if len(args) < 1 {
			log.Fatal("test requires <template-name> flag")
		}
		if err := rayapp.Test(args[0], *testBuildFile); err != nil {
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
	fmt.Println("  build-all              Build all templates")
	fmt.Println("  build <template-name>  Build a specific template")
	fmt.Println("  test                   Test templates")
	fmt.Println("  help                   Show this help message")
	fmt.Println()
	fmt.Println("Build flags (build, build-all):")
	fmt.Println("  --base string      Base directory (default \".\")")
	fmt.Println("  --output string    Output directory (default \"_build\")")
	fmt.Println("  --build string     Build file (default \"BUILD.yaml\")")
	fmt.Println()
	fmt.Println("Test flags:")
	fmt.Println("  --workspace string     Workspace name (required)")
	fmt.Println("  --template-dir string  Template directory (required)")
	fmt.Println("  --config string        Config file path (required)")
}