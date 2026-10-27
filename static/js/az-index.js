(() => {
  if (window.azIndexBound) return;
  window.azIndexBound = true;

  let bar = null;
  let rect = null;
  let letters = [];
  let pendingIndex = -1;
  let shownIndex = -1;
  let frame = 0;

  function indexAt(clientY) {
    const i = Math.floor(((clientY - rect.top) / rect.height) * letters.length);
    return Math.max(0, Math.min(letters.length - 1, i));
  }

  function jumpToPending() {
    frame = 0;
    if (pendingIndex === shownIndex) return;
    letters[shownIndex]?.classList.remove("az-current");
    shownIndex = pendingIndex;
    const letter = letters[shownIndex];
    letter.classList.add("az-current");
    const href = letter.getAttribute("href");
    const target = href && document.getElementById(href.slice(1));
    if (target) target.scrollIntoView({ block: "start", behavior: "instant" });
  }

  function scheduleJump(clientY) {
    pendingIndex = indexAt(clientY);
    if (!frame) frame = requestAnimationFrame(jumpToPending);
  }

  function endDrag() {
    if (!bar) return;
    if (frame) {
      cancelAnimationFrame(frame);
      jumpToPending();
    }
    letters[shownIndex]?.classList.remove("az-current");
    bar.classList.remove("az-dragging");
    bar = null;
  }

  document.addEventListener("pointerdown", (e) => {
    const found = e.target.closest?.(".az-index");
    if (!found || e.button > 0) return;
    e.preventDefault();
    bar = found;
    rect = bar.getBoundingClientRect();
    letters = Array.from(bar.querySelectorAll(".az-letter"));
    shownIndex = -1;
    bar.setPointerCapture(e.pointerId);
    bar.classList.add("az-dragging");
    scheduleJump(e.clientY);
  });

  document.addEventListener("pointermove", (e) => {
    if (bar) scheduleJump(e.clientY);
  }, { passive: true });

  document.addEventListener("pointerup", endDrag);
  document.addEventListener("pointercancel", endDrag);

  document.addEventListener("click", (e) => {
    if (e.detail > 0 && e.target.closest?.(".az-index")) e.preventDefault();
  });
})();
