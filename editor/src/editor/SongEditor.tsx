import { useEffect, useRef, useState } from "react";
import { EditorState } from "prosemirror-state";
import { EditorView } from "prosemirror-view";
import { undo, redo, history } from "prosemirror-history";
import { keymap } from "prosemirror-keymap";
import { baseKeymap, toggleMark } from "prosemirror-commands";
import { schema } from "./schema";
import { blocksToDocJSON, docNodeToBlocks } from "./convert";
import { chordMarkerNodeView, lineNodeView } from "./nodeViews";
import { findEnclosingLineDepth, insertChordAtPos, insertChordAtSelection } from "./commands";
import { formatToolbarPlugin } from "./formatToolbarPlugin";
import { openContextMenu } from "./ContextMenu";
import type { Document as TranscriptionDocument } from "./blocks";

interface SongEditorProps {
  songID: string;
}

type SaveStatus = "idle" | "loading" | "saving" | "saved" | "error";

export function SongEditor({ songID }: SongEditorProps) {
  const mountRef = useRef<HTMLDivElement | null>(null);
  const viewRef = useRef<EditorView | null>(null);
  const [status, setStatus] = useState<SaveStatus>("loading");

  useEffect(() => {
    let cancelled = false;

    fetch(`/songs/${songID}/editor-content`)
      .then((res) => {
        if (!res.ok) throw new Error(`GET editor-content: ${res.status}`);
        return res.json() as Promise<TranscriptionDocument>;
      })
      .then((doc) => {
        if (cancelled || !mountRef.current) return;
        const state = EditorState.create({
          doc: schema.nodeFromJSON(blocksToDocJSON(doc.blocks)),
          plugins: [
            history(),
            keymap({
              "Mod-z": undo,
              "Mod-y": redo,
              "Mod-k": (_state, _dispatch, editorView) => {
                if (editorView) insertChordAtSelection(editorView);
                return true;
              },
              "Mod-b": toggleMark(schema.marks.strong),
              "Mod-i": toggleMark(schema.marks.em),
              "Mod-u": toggleMark(schema.marks.underline),
            }),
            keymap(baseKeymap),
            formatToolbarPlugin(),
          ],
        });
        const view = new EditorView(mountRef.current, {
          state,
          nodeViews: {
            chordMarker: (node, v, getPos) => chordMarkerNodeView(node, v, getPos),
            line: (node, v, getPos) => lineNodeView(node, v, getPos),
          },
        });
        view.dom.addEventListener("contextmenu", (e) => {
          const target = e.target as HTMLElement;
          if (target.closest(".chord-marker")) return; // handled by the marker's own node view
          e.preventDefault();
          const coords = view.posAtCoords({ left: e.clientX, top: e.clientY });
          if (!coords) return;
          const $pos = view.state.doc.resolve(coords.pos);
          if (findEnclosingLineDepth($pos) === null) return;
          openContextMenu(e.clientX, e.clientY, [
            { label: "Insert chord here", onSelect: () => insertChordAtPos(view, coords.pos) },
          ]);
        });
        viewRef.current = view;
        setStatus("idle");
      })
      .catch(() => {
        if (!cancelled) setStatus("error");
      });

    return () => {
      cancelled = true;
      viewRef.current?.destroy();
      viewRef.current = null;
    };
  }, [songID]);

  const save = () => {
    const view = viewRef.current;
    if (!view) return;
    setStatus("saving");
    const blocks = docNodeToBlocks(view.state.doc);
    fetch(`/songs/${songID}/editor-content`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ blocks }),
    })
      .then((res) => {
        if (!res.ok) throw new Error(`POST editor-content: ${res.status}`);
        setStatus("saved");
      })
      .catch(() => setStatus("error"));
  };

  const addChord = () => {
    const view = viewRef.current;
    if (view) insertChordAtSelection(view);
  };

  return (
    <div className="song-editor">
      <div className="song-editor-toolbar">
        <button type="button" onClick={save} disabled={status === "loading" || status === "saving"}>
          Save
        </button>
        <button type="button" onClick={addChord} disabled={status === "loading"} title="Insert a chord at the cursor (Mod-K)">
          + Chord
        </button>
        <span className="song-editor-status">
          {status === "loading" && "Loading…"}
          {status === "saving" && "Saving…"}
          {status === "saved" && "Saved"}
          {status === "error" && "Something went wrong — see console."}
        </span>
      </div>
      <div ref={mountRef} className="song-editor-surface" />
    </div>
  );
}
