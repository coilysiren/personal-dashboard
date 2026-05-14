# Dispatcher

Single seam for every panel write. Tracked: [#40](https://github.com/coilysiren/personal-dashboard/issues/40).

## Why

The daemon is read-only at launch. Every panel action eventually needs to write somewhere: toggle a Hue light, propose a coily allowlist gap, mark an inbox item read. Those writes can hit a dozen different surfaces (Hue API, Sonos, coily CLI, vault filesystem, ElevenLabs).

If panels speak directly to each surface, three things break:

1. Audit trail fragments across protocols.
2. Adding the coily web-ops MCP later means a per-panel rewrite.
3. The Tailscale-only posture has no central choke point.

The dispatcher is that choke point.

## Contract

- Panels never call Hue / Sonos / Cast / coily / vault writes directly.
- Panels call `Dispatcher.Dispatch(action, args)` and render the returned `Result`.
- `Result.URL` is opaque. The PWA opens it. What happens on the other side is the backend's problem.
- Backends are swap-in. Replace the `Dispatcher` value at server construction; every panel's write path moves with it.

## Backends

### DeepLink (initial)

Every action becomes a `coily://<action>?<args>` URL the PWA opens. The user's local coily handles the actual write through its existing audit-logged path. Zero new write surface in the daemon.

### Future: coily web-ops MCP

When the in-progress coily web-ops MCP ships, a new backend replaces `DeepLink`. The daemon authenticates to the MCP, calls the appropriate tool, and returns a `Result` to the panel. Panels do not change.

### Future: stub for tests

A `stubBackend` lives in `dispatcher_test.go` showing the minimum surface to satisfy `Dispatcher`. Panel tests can use the same pattern.

## Action naming

Dotted verbs, lowercase: `hue.toggle`, `sonos.volume.up`, `coily.allowlist.propose`. The leading segment names the target surface. The dispatcher does not enforce a namespace; it is a convention so reading the audit log later is bearable.

## Argument ordering

`DeepLink` sorts argument keys alphabetically before encoding so the same `(action, args)` always produces the same URL. Panels can rely on URL stability for template-side memoization.
