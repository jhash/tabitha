# tabitha song editor

ProseMirror-based rich editor for a song's chord-over-lyric transcription,
mounted at `/songs/{id}/edit` in the main Go app. See the root
[`README.md`](../README.md#song-editor-songsidedit) for the full picture
(build wiring, Docker, the Block/ProseMirror schema mapping).

```sh
npm install
npm run dev      # http://localhost:5173, standalone against index.html's data-song-id="1"
npm run build    # emits ../static/js/editor.js + ../static/css/editor.css
npm test         # vitest — Block<->ProseMirror-doc conversion tests
```
