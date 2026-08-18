package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/yli/taolu/pkg/vault"
)

func runMigrate(args []string) {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	domain := fs.String("domain", vault.DomainPrefix, "target domain for migration (e.g., @local, @liyu1981)")
	user := fs.String("user", "admin", "author to record for the migration commit")
	message := fs.String("message", "", "commit message; defaults to 'migrate taolus to <domain> domain'")
	path := fs.String("path", "", "vault repository path; defaults to TAOLU_REPO or ~/.taolu/vault.fossil")
	dryRun := fs.Bool("dry-run", false, "show what would be migrated without making changes")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, `Usage: taolu migrate [options]

Migrate all existing 2-layer taolus to a 3-layer format under the specified domain.
This is a one-time operation that preserves version history via origin markers.

Options:`)
		fs.PrintDefaults()
		fmt.Fprintln(os.Stderr, `
Examples:
  taolu migrate                      # Migrate to @local domain
  taolu migrate -domain @liyu1981    # Migrate to @liyu1981 domain
  taolu migrate -dry-run             # Show what would be migrated`)
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	r, p, err := vault.OpenVault(*path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer r.Close()

	// Check for legacy taolus
	legacy, err := vault.ListLegacyTaolus(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing legacy taolus: %v\n", err)
		os.Exit(1)
	}

	if len(legacy) == 0 {
		fmt.Println("No legacy 2-layer taolus found. Nothing to migrate.")
		return
	}

	fmt.Printf("Found %d legacy 2-layer taolu(s) to migrate to domain %s:\n", len(legacy), *domain)
	for _, ref := range legacy {
		fmt.Printf("  %s/%s/%s\n", vault.DomainPrefix, ref.Group, ref.Name)
	}

	if *dryRun {
		fmt.Println("\nDry run: no changes made.")
		return
	}

	// Perform migration
	count, err := vault.MigrateToDomain(r, *domain, *user, *message)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error migrating: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nSuccessfully migrated %d taolu(s) to domain %s\n", count, *domain)
	fmt.Printf("Vault: %s\n", p)
}
