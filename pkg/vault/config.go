package vault

import (
	"encoding/json"
	"fmt"

	libfossil "github.com/danmestas/go-libfossil"
)

// GetUserDomain returns the user's default domain from the vault config.
// Returns empty string if not configured.
func GetUserDomain(r *libfossil.Repo) (string, error) {
	val, err := r.Config("user-domain")
	if err != nil {
		return "", nil // Config key not found is not an error
	}
	if val == "" {
		return "", nil
	}
	if !ValidDomain(val) {
		return "", fmt.Errorf("invalid user-domain %q in vault config", val)
	}
	return val, nil
}

// SetUserDomain sets the user's default domain in the vault config.
func SetUserDomain(r *libfossil.Repo, domain string) error {
	if !ValidDomain(domain) {
		return fmt.Errorf("invalid domain %q: must start with @ and be a valid slug", domain)
	}
	return r.SetConfig("user-domain", domain)
}

// GetDomainAliases returns the domain aliases from the vault config.
// Aliases map short names to full domains, e.g., {"@me": "@liyu1981"}.
func GetDomainAliases(r *libfossil.Repo) (map[string]string, error) {
	val, err := r.Config("domain-aliases")
	if err != nil || val == "" {
		return nil, nil
	}
	var aliases map[string]string
	if err := json.Unmarshal([]byte(val), &aliases); err != nil {
		return nil, fmt.Errorf("invalid domain-aliases JSON: %w", err)
	}
	return aliases, nil
}

// SetDomainAliases sets the domain aliases in the vault config.
func SetDomainAliases(r *libfossil.Repo, aliases map[string]string) error {
	// Validate all keys and values
	for key, val := range aliases {
		if !ValidDomain(key) {
			return fmt.Errorf("invalid alias key %q: must start with @ and be a valid slug", key)
		}
		if !ValidDomain(val) {
			return fmt.Errorf("invalid alias value %q for key %q: must start with @ and be a valid slug", val, key)
		}
	}
	data, err := json.Marshal(aliases)
	if err != nil {
		return fmt.Errorf("marshal domain aliases: %w", err)
	}
	return r.SetConfig("domain-aliases", string(data))
}

// ResolveDomainAlias resolves a domain alias to its full domain.
// If the domain is not an alias, it is returned as-is.
func ResolveDomainAlias(domain string, aliases map[string]string) string {
	if aliases == nil {
		return domain
	}
	if full, ok := aliases[domain]; ok {
		return full
	}
	return domain
}

// ParseTaoluRefWithConfig parses a taolu reference and resolves it using vault config.
// It handles domain aliases and the user's default domain.
func ParseTaoluRefWithConfig(r *libfossil.Repo, ref string) (TaoluRef, error) {
	parsed, err := ParseTaoluRef(ref)
	if err != nil {
		return TaoluRef{}, err
	}

	// Get user domain and aliases
	userDomain, _ := GetUserDomain(r)
	aliases, _ := GetDomainAliases(r)

	// Resolve domain alias if present
	if parsed.Domain != "" {
		parsed.Domain = ResolveDomainAlias(parsed.Domain, aliases)
	}

	// Resolve empty domain
	resolved := ResolveTaoluRef(parsed, userDomain)
	return resolved, nil
}