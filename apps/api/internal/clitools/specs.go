package clitools

import "regexp"

// CLIS is the registry of supported providers. Each entry's
// regexes match the device-code prompt that the CLI prints
// before the user opens the URL in a browser.
//
//	az login --use-device-code prints:
//	  "To sign in, use a web browser to open the page
//	   https://microsoft.com/devicelogin and enter the code XXXXXX
//	   to authenticate."
//	gh auth login --no-browser prints:
//	  "! First copy your one-time code: XXXX-XXXX
//	   Press Enter to open the browser or press ctrl-c to cancel."
//
// The latter blocks waiting for <Enter> on stdin before
// printing the URL+code; LoginAck="\n" handles that.
var CLIS = map[string]Spec{
	"az": {
		ID:          "az",
		DisplayName: "Azure CLI",
		HelpText:    "Azure CLI gives the agent access to your Azure resources.",
		Install: map[string][]string{
			"mac": {"brew", "install", "azure-cli"},
		},
		VerifyInstall: []string{"az", "--version"},
		VerifyAuth:    []string{"az", "account", "show", "--output", "none"},
		AccountQuery:  []string{"az", "account", "show", "--query", "user.name", "-o", "tsv"},
		LoginCmd:      []string{"az", "login", "--use-device-code"},
		LoginStream:   "stderr",
		LoginURLRegex: regexp.MustCompile(`(https://microsoft.com/devicelogin)`),
		LoginCodeRegex: regexp.MustCompile(`(?:enter the code\s+)([A-Z0-9]+)`),
		LoginAck:      "",
		LoginTimeoutSeconds: 900,
	},
	"gh": {
		ID:          "gh",
		DisplayName: "GitHub CLI",
		HelpText:    "GitHub CLI lets the agent open PRs, fetch issues, and call the GitHub API.",
		Install: map[string][]string{
			"mac": {"brew", "install", "gh"},
		},
		VerifyInstall: []string{"gh", "--version"},
		VerifyAuth:    []string{"gh", "auth", "status"},
		AccountQuery:  []string{"gh", "api", "user", "--jq", ".login"},
		LoginCmd:      []string{"gh", "auth", "login", "--no-browser", "--git-protocol", "ssh"},
		LoginStream:   "stdout",
		LoginURLRegex: regexp.MustCompile(`(https://github.com/login/device)`),
		LoginCodeRegex: regexp.MustCompile(`(?:one-time code:\s*)([A-Z0-9-]+)`),
		LoginAck:      "\n",
		LoginTimeoutSeconds: 900,
	},
}

// GetSpec returns the spec for id, or false if unknown.
func GetSpec(id string) (Spec, bool) {
	s, ok := CLIS[id]
	return s, ok
}

// List returns the sorted list of supported provider ids.
// Stable order so the Settings panel renders the same rows
// across reloads (and so the API can ship a canonical list).
func List() []string {
	return []string{"az", "gh"}
}