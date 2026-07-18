# Vendored front-end assets

Everything under `static/` that isn't ours is self-hosted rather than
CDN-linked (no FOUT, works offline, no third-party request on every page
load). To upgrade any of these, re-fetch and replace — nothing is built from
source in this repo.

| File | Source | Version | Notes |
|---|---|---|---|
| `static/css/reset.css` | [thoughtbot/roux](https://github.com/thoughtbot/roux)'s reset layer, which is [necolas/normalize.css](https://github.com/necolas/normalize.css) v8.0.1 verbatim | v8.0.1 | MIT licensed |
| `static/fonts/Lora-Variable.woff2` | [google/fonts](https://github.com/google/fonts) `ofl/lora/Lora[wght].ttf`, converted to WOFF2 with `woff2_compress` (`brew install woff2`) | — | SIL OFL 1.1, see `static/fonts/OFL.txt`. Variable font, weight axis 400–700 covers regular *and* bold in one file. |
| `static/fonts/Lora-Italic-Variable.woff2` | same repo, `ofl/lora/Lora-Italic[wght].ttf` | — | Same license/axis notes as above, italic + bold-italic in one file. |
| `static/js/htmx.min.js` | [htmx.org](https://htmx.org) via unpkg | 2.0.10 | BSD 2-Clause |
| `static/js/vendor/sqljs/sql-wasm.js`, `sql-wasm.wasm` | [sql.js](https://github.com/sql-js/sql.js) (`npm pack sql.js`, `dist/`) | 1.13.0 | MIT, see `static/js/vendor/sqljs/LICENSE`. Loaded lazily by `static/sw.js` only when serving an offline navigation for a song not already in the HTTP cache — never fetched on a normal page load. |

Font conversion, if re-doing it:

```sh
brew install woff2
curl -o Lora-Variable.ttf 'https://raw.githubusercontent.com/google/fonts/main/ofl/lora/Lora%5Bwght%5D.ttf'
curl -o Lora-Italic-Variable.ttf 'https://raw.githubusercontent.com/google/fonts/main/ofl/lora/Lora-Italic%5Bwght%5D.ttf'
woff2_compress Lora-Variable.ttf
woff2_compress Lora-Italic-Variable.ttf
rm Lora-Variable.ttf Lora-Italic-Variable.ttf  # keep only the woff2 output
```
