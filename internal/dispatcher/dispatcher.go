// Package dispatcher is the single seam through which every panel action
// flows. Initial backend emits a coily deep-link URL the PWA opens. When
// the coily web-ops MCP is ready, swap the backend without touching panels.
//
// Tracked: https://github.com/coilysiren/personal-dashboard/issues/40
package dispatcher

import "fmt"

// Action names the operation a panel wants to perform. Verbs are
// dotted: "hue.toggle", "sonos.volume.up", "coily.allowlist.propose".
type Action string

// Result is what panels render after a dispatch. URL points the PWA at
// the coily CLI invocation (or, later, an MCP-mediated write).
type Result struct {
	URL string
}

// Dispatcher routes panel actions to a backend.
type Dispatcher interface {
	Dispatch(action Action, args map[string]string) (Result, error)
}

// DeepLink is the initial backend. Every action becomes a coily:// URL the
// PWA opens. Read-only at launch means panels render the link, user taps it,
// coily on the user's machine handles the actual write.
type DeepLink struct{}

func (DeepLink) Dispatch(action Action, args map[string]string) (Result, error) {
	url := fmt.Sprintf("coily://%s", action)
	if len(args) > 0 {
		url += "?"
		first := true
		for k, v := range args {
			if !first {
				url += "&"
			}
			url += fmt.Sprintf("%s=%s", k, v)
			first = false
		}
	}
	return Result{URL: url}, nil
}
