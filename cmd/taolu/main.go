// Command taolu is a versioned practice library for AI agents.
//
// Usage:
//
//	taolu                     show help
//	taolu serve               start MCP server + web UI
//	taolu serve --with=stdio  start in stdio mode
//	taolu install             install slash commands (interactive TUI)
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	switch os.Args[1] {
	case "serve":
		runServe(os.Args[2:])
	case "install":
		runInstall(os.Args[2:])
	case "init":
		runInit(os.Args[2:])
	case "migrate":
		runMigrate(os.Args[2:])
	case "fork":
		runFork(os.Args[2:])
	case "fork-info":
		runForkInfo(os.Args[2:])
	case "version":
		runVersion()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: taolu <command> [options]

Commands:
  serve      Start the MCP server and/or web UI
  init       Create or open the practice vault
  install    Install taolu slash commands for an agent tool
  migrate    Migrate 2-layer taolus to 3-layer domain format
  fork       Fork a taolu into a new name (clones content + history)
  fork-info  Show fork provenance for a taolu
  version    Show the build version
  help       Show this help message

Run 'taolu <command> --help' for command-specific options.`)
}
