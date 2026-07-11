const VIEWPORT_MARGIN = 8;

// Shared by ChordPicker/ContextMenu/ChordActionBar: clamps a fixed-position
// popup to stay fully inside the viewport, flipping above its anchor when
// there's not enough room below, and re-clamps on resize/scroll so it
// doesn't drift off-screen if the window changes size while open. Callers
// pass a getter rather than a static rect so the anchor's live position is
// used on every reposition (relevant when the anchor itself can scroll).
export interface PopupPositioner {
  reposition: () => void;
  cleanup: () => void;
}

export function positionPopup(
  root: HTMLElement,
  getAnchorRect: () => DOMRect,
  opts?: { preferAbove?: boolean },
): PopupPositioner {
  function reposition() {
    const anchor = getAnchorRect();
    const width = root.offsetWidth;
    const height = root.offsetHeight;

    const below = anchor.bottom + 4;
    const above = anchor.top - height - 4;

    let top: number;
    if (opts?.preferAbove) {
      top = above >= 0 ? above : below;
    } else {
      top = below + height > window.innerHeight - VIEWPORT_MARGIN && above >= 0 ? above : below;
    }
    top = Math.max(VIEWPORT_MARGIN, Math.min(top, window.innerHeight - height - VIEWPORT_MARGIN));

    const left = Math.max(
      VIEWPORT_MARGIN,
      Math.min(anchor.left, window.innerWidth - width - VIEWPORT_MARGIN),
    );

    root.style.top = `${top}px`;
    root.style.left = `${left}px`;
  }

  reposition();
  window.addEventListener("resize", reposition);
  window.addEventListener("scroll", reposition, true);

  return {
    reposition,
    cleanup: () => {
      window.removeEventListener("resize", reposition);
      window.removeEventListener("scroll", reposition, true);
    },
  };
}
