// Package dispatcher is the single seam through which every panel action
// flows. Initial backend emits a coily deep-link URL the PWA opens. When
// the coily web-ops MCP is ready, swap the backend without touching panels.
//
// Contract:
//
//   - Panels never speak directly to Hue, Sonos, Cast, coily, or any other
//     write surface. They call Dispatcher.Dispatch and render the Result.
//
//   - The Result.URL is opaque to the panel. The PWA opens it; what happens
//     on the other side is the backend's problem.
//
//   - Backends are swap-in: replace the Dispatcher value at server
//     construction time and every panel's write path moves to the new
//     backend.
//
// Tracked: https://github.com/coilysiren/personal-dashboard/issues/40
package dispatcher

import (
	"fmt"
	"net/url"
	"sort"
)

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
//
// Argument order in the URL is sorted alphabetically by key so the same
// (action, args) input always produces the same URL. Tests rely on this.
type DeepLink struct{}

func (DeepLink) Dispatch(action Action, args map[string]string) (Result, error) {
	if action == "" {
		return Result{}, fmt.Errorf("dispatcher: empty action")
	}
	u := "coily://" + string(action)
	if len(args) > 0 {
		keys := make([]string, 0, len(args))
		for k := range args {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		q := url.Values{}
		for _, k := range keys {
			q.Set(k, args[k])
		}
		u += "?" + q.Encode()
	}
	return Result{URL: u}, nil
}
