import { useEffect, useRef, useState } from "react";
import { EditorState } from "prosemirror-state";
import { EditorView } from "prosemirror-view";
import { undo, redo, history } from "prosemirror-history";
import { keymap } from "prosemirror-keymap";
import { baseKeymap } from "prosemirror-commands";
import { schema } from "./schema";
import { blocksToDocJSON, docNodeToBlocks } from "./convert";
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
          plugins: [history(), keymap({ "Mod-z": undo, "Mod-y": redo }), keymap(baseKeymap)],
        });
        viewRef.current = new EditorView(mountRef.current, { state });
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

  return (
    <div className="song-editor">
      <div className="song-editor-toolbar">
        <button type="button" onClick={save} disabled={status === "loading" || status === "saving"}>
          Save
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
