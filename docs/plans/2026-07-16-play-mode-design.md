# Play mode — design

## Goal

Add a "Play" mode to song show pages: a book/ereader-style fullscreen view of
the same transcription, paginated into swipeable/keyboard/arrow-button
screens instead of one long scroll.

## Route & handler

- `GET /songs/{idOrSlug}/play`, public (same access as `/songs/{idOrSlug}`).
- `internal/web/song_play.go`, `SongPlayHandler(q)`: resolves song
  (id-or-slug, numeric→slug redirect) same as `SongShowHandler`. If the song
  has no current version yet, redirect to the show page — nothing to
  paginate.
- Renders via a new `PagePlay(title, description string, body ...g.Node)` in
  `layout.go`: full HTML5 doc (font preload, reset.css, style.css) but no
  site header/sidebar/`.container` — body renders full-bleed.
- Content reuses `renderTranscriptionHTML(omitDuplicateHeaderLines(blocks,
  song))` — the same function the show page calls — so any future change to
  chord/lyric rendering applies to both views.
- Show page (`song_show.go`) gets a "Play" icon-button link next to the
  existing "Edit" affordance, pointing at `songShowHref(song) + "/play"`.

## Pagination mechanics

CSS multi-column does the actual page-splitting — no JS measures where
content breaks:

- `.play-columns` (title/byline/key + transcription blocks, all one flow):
  `column-width: var(--page-w); height: var(--page-h); column-fill: auto`.
- `.play-scroller` wraps it: `overflow-x: auto; overflow-y: hidden`, so
  swipe/trackpad scroll works natively — no touch-event JS.
- `break-inside: avoid` on `.chord-line`/`.text-line`/`.section-header` stops
  a line splitting mid-line across a page boundary; `break-after: avoid` on
  `.section-header` keeps a header from stranding at a column's bottom.

Generated columns aren't addressable DOM elements, so native
`scroll-snap-align` can't target them. Snapping is done in JS instead: a
`scrollend` listener rounds `scrollLeft` to the nearest multiple of
page-width and smooth-scrolls there. This is the one piece of JS beyond
"fires on resize" — still no page-splitting logic, no library, no build
step.

`--page-w`/`--page-h` CSS vars are set from the scroller's own
`ResizeObserver` (handles mobile URL-bar collapse/orientation better than
`window.resize`), backed by `100dvh` in CSS so layout is correct even before
JS runs on first paint.

## Navigation

- `.play-prev`/`.play-next`: fixed low-opacity arrow buttons, left/right
  edges. `scrollBy({left: ±clientWidth, behavior: 'smooth'})` — browser
  clamps at start/end, no boundary math.
- Keyboard: `ArrowLeft`/`ArrowRight` → same scrollBy. `Escape` → navigate to
  show page (via `data-show-href` on the root).
- `.play-close` (×, top corner) → same show-page navigation.
- No page-count indicator or disabled boundary states — YAGNI.

## New/changed files

- `internal/web/song_play.go` (new) — handler + content builder.
- `internal/web/layout.go` — add `PagePlay`.
- `internal/web/assets.go` — add `PlayCSS`/`PlayJS` fields + loading.
- `internal/web/router.go` — register route.
- `internal/web/song_show.go` — add Play button.
- `static/css/play.css` (new) — fullscreen/pagination/nav-button styles.
- `static/js/play.js` (new) — ResizeObserver sizing, scrollend snap,
  buttons/keyboard/close nav. Plain JS, no build step.
