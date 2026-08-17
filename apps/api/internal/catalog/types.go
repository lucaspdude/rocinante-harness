package catalog

// LoginProviderInfo is the wire form for the omp provider list
// mirrored by /api/v1/login/providers and /api/v1/meta. Lives in
// the catalog package so that other packages can depend on the
// type without creating an import cycle with the api package.
type LoginProviderInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Auth          string `json:"auth"` // "oauth" | "paste-key" | "keyless"
	Available     bool   `json:"available"`
	Authenticated bool   `json:"authenticated"`
	EnvVar        string `json:"env_var,omitempty"`
	HelpURL       string `json:"help_url,omitempty"`
}
