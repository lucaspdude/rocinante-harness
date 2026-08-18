package catalog

// LoginProviderInfo is the wire form for the omp provider list
// mirrored by /api/v1/login/providers and /api/v1/meta. Lives in
// the catalog package so that other packages can depend on the
// type without creating an import cycle with the api package.
//
// Per PR-01 advisory 4 (2026-08-17) and the cross-PR §1.7 of
// 04-analysis.md, the model is *capabilities*, not a single
// "auth" string. omp providers vary along three orthogonal axes:
//   - EnvVars    : which API key env vars the provider reads.
//   - SupportsLogin: whether /login has an OAuth/device-code flow.
//   - Keyless    : whether the provider requires no auth at all
//                  (e.g. local engines like ollama, lmstudio).
type LoginProviderInfo struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Available     bool     `json:"available"`
	Authenticated bool     `json:"authenticated"`
	EnvVars       []string `json:"env_vars,omitempty"`
	SupportsLogin bool     `json:"supports_login"`
	Keyless       bool     `json:"keyless,omitempty"`
	HelpURL       string   `json:"help_url,omitempty"`
}
