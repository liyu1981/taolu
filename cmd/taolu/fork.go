package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/yli/taolu/pkg/vault"
)

func runFork(args []string) {
	fs := flag.NewFlagSet("fork", flag.ExitOnError)
	newGroup := fs.String("group", "", "new group folder (defaults to source group)")
	message := fs.String("message", "", "commit message")
	user := fs.String("user", "admin", "author to record")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, `Usage: taolu fork <source> <new-name> [options]

Fork a taolu: clone its SKILL.md, ACTION.md, and files/ assets into a new
name, recording a .fork provenance marker so the fork's history shows the
copied upstream lineage followed by independent saves. The source is untouched.

Options:
  --group <group>    new group folder (defaults to source group)
  --message <msg>    commit message
  --user <name>      author to record (default: admin)
  -h, --help         show this help message

Examples:
  taolu fork @local/backend/go-api-server  my-custom-api
  taolu fork go-api-server  my-api --group=frontend`)
	}
	fs.Parse(args)
	if fs.NArg() < 2 {
		fs.Usage()
		os.Exit(1)
	}
	source := fs.Arg(0)
	newName := fs.Arg(1)

	r, p, err := vault.OpenVault("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer r.Close()

	ref, err := vault.ParseTaoluRefWithConfig(r, source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	f, err := vault.ForkTaolu(r, ref, newName, *newGroup, *message, *user)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	newRef := vault.TaoluRef{Domain: f.Source.Domain, Group: f.Source.Group, Name: newName}
	if *newGroup != "" {
		newRef.Group = *newGroup
	}

	hist, err := vault.SkillHistory(r, newRef.Path())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading history: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("forked %s -> %s\n", f.Source.String(), newRef.String())
	fmt.Printf("forked from: %s (version %s)\n", f.Source.String(), f.Version)
	fmt.Printf("history: %d version(s)\n", len(hist))
	fmt.Printf("vault: %s\n", p)
}

func runForkInfo(args []string) {
	fs := flag.NewFlagSet("fork-info", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, `Usage: taolu fork-info <name>

Show fork provenance for a taolu: the source taolu and version it was
forked from, or a note that it is not a fork.

Options:
  -h, --help   show this help message`)
	}
	fs.Parse(args)
	if fs.NArg() < 1 {
		fs.Usage()
		os.Exit(1)
	}
	name := fs.Arg(0)

	r, _, err := vault.OpenVault("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer r.Close()

	ref, err := vault.ParseTaoluRefWithConfig(r, name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	sp, err := vault.FindSkillPathByRefResolved(r, ref)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if sp == "" {
		fmt.Fprintf(os.Stderr, "taolu %q not found in vault\n", ref.String())
		os.Exit(1)
	}

	f, err := vault.ReadForkInfo(r, ref)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if f == nil {
		fmt.Printf("%s is not a fork\n", ref.String())
		return
	}
	fmt.Printf("%s\nforked from: %s\nsource version: %s\nsource uuid: %s\n",
		ref.String(), f.Source.String(), f.Version, vault.ShortUUID(f.SourceUUID))
	// Show the fork's full history for context.
	hist, err := vault.SkillHistory(r, sp)
	if err == nil && len(hist) > 0 {
		fmt.Printf("\nhistory:\n")
		for _, v := range hist {
			fmt.Printf("  %s  %s  %s  %s  %s\n", v.Label, vault.ShortUUID(v.UUID),
				v.Date.Format("2006-01-02 15:04:05"), v.User, v.Message)
		}
	}
}
