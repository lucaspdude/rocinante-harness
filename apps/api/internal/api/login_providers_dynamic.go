package api

// Dynamic omp-based LoginProvidersProvider.
//
// Shells out to `omp --mode rpc-ui` once, sends
// {"type":"get_login_providers"} on stdin, parses the single
// JSONL response on stdout, decodes into the same
// catalog.LoginProviderInfo shape as the static catalog.
//
// Falls back to the provided fallback when omp is unavailable or
// the spawn fails (timeout, missing binary, non-zero exit).
//
// Per `docs/mvp/phase-1-functionality/04-analysis.md` §1.4, omp's
// `get_login_providers` returns {id, name, available, authenticated}
// (4 fields) — not the canonical harness shape. We cross-reference
// with the keystore EnvProbe + the `available` boolean to populate
// the canonical fields.

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/lucaspdude/rocinante-harness/apps/api/internal/catalog"
)

// ompLoginProviders is a dynamic provider that talks to a real
// omp child once per list call. Cache lives outside (LoginProvidersCache).
type ompLoginProviders struct {
	bin    string
	args   []string // default: ["--mode", "rpc-ui"]
	probe  ProviderProbe
	fallback LoginProvidersProvider
	timeout time.Duration // default 5s
}

// NewOMPLoginProviders builds a dynamic provider.
// bin is the path to the omp binary ("" falls through).
// fallback is consulted when the spawn fails.
func NewOMPLoginProviders(bin string, probe ProviderProbe, fallback LoginProvidersProvider) LoginProvidersProvider {
	if probe == nil {
		probe = nullProbe{}
	}
	return &ompLoginProviders{
		bin:      bin,
		args:     []string{"--mode", "rpc-ui"},
		probe:    probe,
		fallback: fallback,
		timeout:  5 * time.Second,
	}
}

// ompProviderRaw is the upstream omp `get_login_providers` row.
type ompProviderRaw struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Available     bool   `json:"available"`
	Authenticated bool   `json:"authenticated"`
	EnvVar        string `json:"env_var,omitempty"`
}

// List shells out to omp for the live provider list. On any
// failure (omp missing, timeout, malformed JSON), it returns the
// fallback catalog so the UI never goes blank.
func (o *ompLoginProviders) List() []catalog.LoginProviderInfo {
	if o.bin == "" {
		return o.fallback.List()
	}
	ctx, cancel := context.WithTimeout(context.Background(), o.timeout)
	defer cancel()
	cmd := execCommandContextDynamic(ctx, o.bin, o.args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return o.fallback.List()
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return o.fallback.List()
	}

	if err := cmd.Start(); err != nil {
		return o.fallback.List()
	}

	// Send the JSONL request: one line, terminated with a newline.
	if _, err := io.WriteString(stdin, `{"type":"get_login_providers"}` + "\n"); err != nil {
		_ = stdin.Close()
		return o.fallback.List()
	}
	_ = stdin.Close()

	var rawList []ompProviderRaw
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Bytes()
		var resp struct {
			Type    string           `json:"type"`
			Result  any               `json:"result,omitempty"`
			List    []ompProviderRaw `json:"list,omitempty"`
			Error   string           `json:"error,omitempty"`
		}
		if err := json.Unmarshal(line, &resp); err != nil {
			continue
		}
		if resp.Type != "response" && resp.Type != "get_login_providers_response" {
			continue
		}
		if resp.Error != "" {
			return o.fallback.List()
		}
		// Some omp versions nest under result.list, others at top
		// level. Pick whichever is populated.
		if len(resp.List) > 0 {
			rawList = resp.List
			break
		}
		if resultMap, ok := resp.Result.(map[string]any); ok {
			if listAny, ok := resultMap["list"]; ok {
				if raw, ok := listAny.([]any); ok {
					for _, e := range raw {
						b, _ := json.Marshal(e)
						var p ompProviderRaw
						if err := json.Unmarshal(b, &p); err == nil {
							rawList = append(rawList, p)
						}
					}
				}
			}
		}
		break
	}
	if err := cmd.Wait(); err != nil {
		// Non-zero exit: fall back.
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			// log only when not a timeout
		}
		if len(rawList) == 0 {
			return o.fallback.List()
		}
	}

	if len(rawList) == 0 {
		return o.fallback.List()
	}

	out := make([]catalog.LoginProviderInfo, 0, len(rawList))
	for _, p := range rawList {
		envVars := []string{}
		if p.EnvVar != "" {
			envVars = append(envVars, p.EnvVar)
		}
		if len(envVars) == 0 {
			// omp may omit env_var for keyless / oauth-only
			// providers. Best-effort: pull from the keystore
			// catalogue so paste-key still works.
			if ev, ok := knownEnvVarFor(p.ID); ok {
				envVars = []string{ev}
			}
		}
		out = append(out, catalog.LoginProviderInfo{
			ID:            p.ID,
			Name:          chooseName(p.ID, p.Name),
			Available:     p.Available,
			Authenticated: p.Authenticated || o.probe.IsConfigured(p.ID),
			EnvVars:       envVars,
			SupportsLogin: true,
			Keyless:       len(envVars) == 0,
			HelpURL:       helpURLFor(p.ID),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// execCommandContextDynamic is the seam so tests can substitute
// exec.CommandContext. Defaults to osExec.
var execCommandContextDynamic = func(ctx context.Context, name string, args ...string) cmdIface {
	return defaultCmdFactory(ctx, name, args...)
}

// chooseName picks the human-readable name for an omp provider,
// falling back to the literal id if name is empty.
func chooseName(id, name string) string {
	if name != "" {
		return name
	}
	return id
}

// knownEnvVarFor returns the env var name for canonical paste-key
// providers. Used as a fallback when omp omits env_var in its
// get_login_providers response.
func knownEnvVarFor(id string) (string, bool) {
	switch id {
	case "anthropic":
		return "ANTHROPIC_API_KEY", true
	case "openai":
		return "OPENAI_API_KEY", true
	case "gemini", "google-gemini-cli":
		return "GEMINI_API_KEY", true
	case "openrouter":
		return "OPENROUTER_API_KEY", true
	case "minimax", "minimax-tokenplan":
		return "MINIMAX_API_KEY", true
	}
	return "", false
}

// helpURLFor returns the docs URL for canonical providers (used by
// the panel's "Get key" link).
func helpURLFor(id string) string {
	switch id {
	case "anthropic":
		return "https://console.anthropic.com/settings/keys"
	case "openai":
		return "https://platform.openai.com/api-keys"
	case "gemini", "google-gemini-cli":
		return "https://aistudio.google.com/apikey"
	case "openrouter":
		return "https://openrouter.ai/settings/keys"
	case "minimax", "minimax-tokenplan":
		return "https://minimax.io/dashboard"
	}
	return ""
}

// Quiet for unused imports.
var (
	_ = sync.Mutex{}
	_ = fmt.Sprint("")
)
