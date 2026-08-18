package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/yli/taolu/pkg/vault"
)

func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	user := fs.String("user", "admin", "user recorded for seeded commits")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, `Usage: taolu init [options]

Create or open the practice vault, seed the taolu-authoring guide, and
migrate any legacy practices/ tree.

Options:
  --user <name>   user recorded for seeded commits (default: admin)
  -h, --help      show this help message`)
	}
	fs.Parse(args)

	r, p, err := vault.EnsureVault("", *user)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer r.Close()

	projectCode, _ := r.Config("project-code")
	taolus, _ := vault.ListTaolu(r)
	groups := vault.UniqueGroups(taolus)

	fmt.Printf("vault ready: %s\n", p)
	fmt.Printf("project-code: %s\n", projectCode)
	fmt.Printf("taolus: %d (in %d groups)\n", len(taolus), len(groups))
}
