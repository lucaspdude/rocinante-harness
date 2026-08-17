package keystore

import "os"

// EnvProbe implements omp.ProviderProbe. It returns true when
// EITHER the keystore has a value for the given provider OR
// the matching env var is set in the api's environment. We
// check both because:
//   - The keystore is the new "configured via web form" path
//     and is the source of truth for what the api will inject
//     into spawned omp subprocesses.
//   - The env var is the legacy "configured via systemd
//     EnvironmentFile" path (which the installer seeds from
//     .env.local at install time).
// Either path makes the provider usable; both is fine too.
type EnvProbe struct {
	Store *Store
}

// IsConfigured implements omp.ProviderProbe.
func (p *EnvProbe) IsConfigured(name string) bool {
	if p.Store != nil {
		if v, err := p.Store.Get(ProviderName(name)); err == nil && v != "" {
			return true
		}
	}
	for _, p := range KnownProviders {
		if string(p) != name {
			continue
		}
		if os.Getenv(p.EnvVar()) != "" {
			return true
		}
	}
	return false
}
