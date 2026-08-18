package catalog

// Cross-reference between models.dev catalog and the live provider
// list served by omp. Annotates each entry with `selectable: bool`
// (true when the model's provider is in omp's `get_login_providers`
// response as `available` and authenticated).

import "strings"

// LoginProvidersProvider is the seam from api — declared as an
// interface here to avoid an import cycle (catalog imports nothing
// from api; api imports catalog for the model types).
type LoginProvidersProvider interface {
	List() []LoginProviderInfo
}

// AnnotateSelectable marks each entry's `Selectable` based on the
// passed-in omp provider list.
func AnnotateSelectable(entries []ModelsDevEntry, providers []LoginProviderInfo) []ModelsDevEntry {
	available := make(map[string]bool)
	authenticated := make(map[string]bool)
	for _, p := range providers {
		available[strings.ToLower(p.ID)] = p.Available
		authenticated[strings.ToLower(p.ID)] = p.Authenticated
	}
	for i := range entries {
		pid := strings.ToLower(entries[i].Provider)
		ok, has := available[pid]
		entries[i].Selectable = has && ok && authenticated[pid]
	}
	return entries
}

// LoginProviderIsAvailable reports whether the given provider id
// is in the provider list as available + authenticated.
func LoginProviderIsAvailable(providers []LoginProviderInfo, id string) bool {
	for _, p := range providers {
		if strings.EqualFold(p.ID, id) {
			return p.Available && p.Authenticated
		}
	}
	return false
}
