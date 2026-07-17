// Play mode: CSS multi-column pagination handles the actual page layout
// (see play.css) — this script only sizes the pages to the viewport and
// wires up navigation. It does no page-splitting math of its own.

const root = document.getElementById("play-root");
const scroller = document.getElementById("play-scroller");

// ResizeObserver's callback fires once immediately on observe() with the
// element's current size, so this also covers sending the initial frame
// size — no separate "on load" call needed.
let lastPageWidth = scroller.clientWidth;
new ResizeObserver((entries) => {
  const { inlineSize, blockSize } = entries[0].contentBoxSize
    ? entries[0].contentBoxSize[0]
    : { inlineSize: entries[0].contentRect.width, blockSize: entries[0].contentRect.height };
  // Snapshot which page scrollLeft currently sits on using the OLD page
  // width before resizing — otherwise a resize (mobile address-bar
  // collapse, rotation, a manual window resize) changes the column pitch
  // out from under the old scrollLeft, stranding the view mid-page: part
  // of the previous page's tail plus blank space, not a clean page.
  const pageIndex = lastPageWidth ? Math.round(scroller.scrollLeft / lastPageWidth) : 0;
  document.documentElement.style.setProperty("--page-w", `${inlineSize}px`);
  document.documentElement.style.setProperty("--page-h", `${blockSize}px`);
  lastPageWidth = inlineSize;
  scroller.scrollTo({ left: pageIndex * inlineSize, behavior: "instant" });
}).observe(scroller);

function goToAdjacentPage(direction) {
  scroller.scrollBy({ left: direction * scroller.clientWidth, behavior: "smooth" });
}

function closePlayMode() {
  window.location.href = root.dataset.showHref;
}

// A page turn, not a scroll: native touch scrolling has momentum, so a fast
// flick can sail past several pages — there's no way to cap that once the
// browser's own scroll physics owns the gesture. Instead we take over the
// gesture ourselves (play.css sets touch-action: none on the scroller so
// the browser doesn't also try to pan it) — drag moves the content 1:1
// with the finger for visual feedback, and release is a binary decision
// (past the threshold or not), so a swipe can only ever land on the
// adjacent page or snap back, no matter how far or fast it was.
let dragStartX = null;
let dragStartScrollLeft = 0;

scroller.addEventListener("pointerdown", (e) => {
  if (e.pointerType === "mouse") return; // mouse users get buttons/keyboard, not drag
  dragStartX = e.clientX;
  dragStartScrollLeft = scroller.scrollLeft;
  scroller.setPointerCapture(e.pointerId);
});

scroller.addEventListener("pointermove", (e) => {
  if (dragStartX === null) return;
  scroller.scrollLeft = dragStartScrollLeft - (e.clientX - dragStartX);
});

function endDrag(e) {
  if (dragStartX === null) return;
  const delta = e.clientX - dragStartX;
  dragStartX = null;
  const pageWidth = scroller.clientWidth;
  const threshold = pageWidth * 0.2;
  let pageIndex = Math.round(dragStartScrollLeft / pageWidth);
  if (delta < -threshold) pageIndex += 1;
  else if (delta > threshold) pageIndex -= 1;
  scroller.scrollTo({ left: pageIndex * pageWidth, behavior: "smooth" });
}

scroller.addEventListener("pointerup", endDrag);
scroller.addEventListener("pointercancel", endDrag);

document.querySelector(".play-prev").addEventListener("click", () => goToAdjacentPage(-1));
document.querySelector(".play-next").addEventListener("click", () => goToAdjacentPage(1));
document.querySelector(".play-close").addEventListener("click", closePlayMode);

document.addEventListener("keydown", (e) => {
  if (e.key === "ArrowLeft") goToAdjacentPage(-1);
  else if (e.key === "ArrowRight") goToAdjacentPage(1);
  else if (e.key === "Escape") closePlayMode();
});
