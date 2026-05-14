# PWA shell conventions

Phone is the design center. Desktop is the same layout, wider.

## Layout primitives

All primitives live in `static/app.css`. Panels (#43-#48) reach for these instead of inventing per-panel layout:

- `.stack` - vertical column, gap between children.
- `.row` - horizontal row that wraps on narrow screens.
- `.card` - panel container, rounded corner + subtle border.
- `.tap` - tap target, minimum 44px high.
- `.tap.accent` - emphasized action.
- `.muted` - secondary text.
- `.redactable` - blurred by default. Lifted by `.revealed` on the route root (wired in #41).

If a panel needs something none of these cover, the new class belongs in `app.css` so other panels can reuse it.

## Templates

`templates/base.html.tmpl` defines the root layout. Panel templates start with `{{template "base" .}}` and override the `title` and `main` blocks.

## Static assets

Vendored, not CDN-loaded, so the PWA works offline once installed:

- `htmx.min.js` - htmx 2.0.4. Drives all panel interactions.
- `app.css` - layout primitives.
- `sw.js` - service worker. Shell-cache only. Panel data (`/panels/*`, `/api/*`) always hits the network.
- `manifest.webmanifest` - PWA manifest.
- `icon.svg` - app icon, scalable, also serves as apple-touch-icon.

## Adding a panel

When implementing a panel from #43-#48:

1. Drop the template under `templates/panels/<name>.html.tmpl`.
2. Register a `/panels/<name>` route on the mux.
3. Render through htmx (the panel root requests its content async to keep page navigation snappy).
4. Wrap any field that may carry private data in `<span class="redactable">`.

## Reveal model

Per-page granularity, per-session persistence ([#41](https://github.com/coilysiren/personal-dashboard/issues/41)). The route root carries `.revealed` when the session has revealed that route. Server controls the class; client never toggles redaction directly.
