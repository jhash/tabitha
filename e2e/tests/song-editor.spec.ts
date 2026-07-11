import { readFileSync } from "fs";
import path from "path";
import { test, expect, type Page } from "@playwright/test";

const songID = readFileSync(path.join(__dirname, "..", ".song-id"), "utf8").trim();

async function login(page: Page) {
  await page.goto("/dev-login");
}

async function openEditor(page: Page) {
  await login(page);
  await page.goto(`/songs/${songID}/edit`);
  await page.waitForSelector(".song-editor-surface .line");
}

test.describe("song editor", () => {
  test("loads the seeded song's real content", async ({ page }) => {
    await openEditor(page);
    await expect(page.locator(".line-content").first()).toContainText("E2E Test Song");
  });

  test("no chord label overlaps another line's actual text", async ({ page }) => {
    await openEditor(page);

    // Compares each label's rect against other lines' real glyph rects
    // (via Range.getClientRects(), not getBoundingClientRect() on the
    // .line/.section-header container) — a label is deliberately allowed
    // to render outside its own line's box (see index.css), so comparing
    // against container boxes produces false positives; only overlapping
    // another line's rendered text is a real bug.
    const overlaps = await page.evaluate(() => {
      const found: { label: string | null; blockText: string }[] = [];
      const labels = [...document.querySelectorAll(".chord-marker-label")];
      const textBlocks = [...document.querySelectorAll(".line-content, .section-header")];
      for (const label of labels) {
        const lr = label.getBoundingClientRect();
        const ownLine = label.closest(".line, .section-header");
        for (const block of textBlocks) {
          if (block.closest(".line, .section-header") === ownLine) continue;
          const range = document.createRange();
          range.selectNodeContents(block);
          for (const rect of range.getClientRects()) {
            if (rect.width === 0 || rect.height === 0) continue;
            const xOverlap = lr.left < rect.right && lr.right > rect.left;
            const yOverlap = lr.top < rect.bottom && lr.bottom > rect.top;
            if (xOverlap && yOverlap) {
              found.push({ label: label.textContent, blockText: block.textContent?.slice(0, 25) ?? "" });
            }
          }
        }
      }
      return found;
    });

    expect(overlaps).toEqual([]);
  });

  test("gap between two separate chorded lines matches the gap between wrapped rows of one", async ({ page }) => {
    await openEditor(page);

    // CHORUS 1 in the seed data (real lyrics from the catalog) wraps
    // within the editor's content width, so its own two rows give the
    // in-wrap rhythm to compare against the gap to the next chorded line.
    const { withinWrapGap, acrossLineGap } = await page.evaluate(() => {
      const chordLines = [...document.querySelectorAll(".line.has-chords")];
      // Chord markers interrupt mid-word (see cmd/e2eseed), so textContent
      // has no space around them — match on substrings that survive that.
      const wrapping = chordLines.find((l) => (l.textContent ?? "").includes("You can't tell me"));
      const next = chordLines.find((l) => (l.textContent ?? "").includes("You know it's"));
      const rowTops = [...new Set(
        [...wrapping!.querySelectorAll(".chord-marker-label")].map((l) => Math.round(l.getBoundingClientRect().top)),
      )].sort((a, b) => a - b);
      const nextTop = Math.round(next!.querySelector(".chord-marker-label")!.getBoundingClientRect().top);
      return {
        withinWrapGap: rowTops[1] - rowTops[0],
        acrossLineGap: nextTop - rowTops[rowTops.length - 1],
      };
    });

    // Sub-pixel rounding can differ by a hair; anything more is a real gap.
    expect(Math.abs(acrossLineGap - withinWrapGap)).toBeLessThanOrEqual(1);
  });

  test("a chord label sits directly above the word it belongs to", async ({ page }) => {
    await openEditor(page);

    // Seeded tokens: ... {chord: Fm} {text: "right here"} — Fm sits right
    // before "right", not "something" (that one's preceded by Eb).
    const { markerX, wordX } = await page.evaluate(() => {
      const markers = [...document.querySelectorAll(".chord-marker")];
      const fm = markers.find((m) => m.textContent?.trim() === "Fm");
      const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
      let node: Node | null;
      let wordRect: DOMRect | null = null;
      while ((node = walker.nextNode())) {
        const idx = node.textContent?.indexOf("right here") ?? -1;
        if (idx >= 0) {
          const range = document.createRange();
          range.setStart(node, idx);
          range.setEnd(node, idx + "right here".length);
          wordRect = range.getBoundingClientRect();
          break;
        }
      }
      return { markerX: fm!.getBoundingClientRect().x, wordX: wordRect!.x };
    });

    expect(Math.abs(markerX - wordX)).toBeLessThan(1);
  });

  test("clicking a chord marker opens the floating action bar", async ({ page }) => {
    await openEditor(page);

    // The chordMarker itself is deliberately zero-width (see index.css) so
    // it takes no space in the lyric line — its label child is what's
    // actually visible/clickable-sized, so that's what Playwright needs to
    // interact with rather than the zero-size parent.
    const marker = page.locator(".chord-marker").first();
    const label = marker.locator(".chord-marker-label");
    await label.click();

    const bar = page.locator(".chord-action-bar");
    await expect(bar).toBeVisible();
  });

  test("the action bar's change button opens the picker and changing the chord updates the label", async ({ page }) => {
    await openEditor(page);

    const marker = page.locator(".chord-marker").first();
    const label = marker.locator(".chord-marker-label");
    await label.click();

    const bar = page.locator(".chord-action-bar");
    await bar.locator(".chord-action-chord").click();

    const picker = page.locator(".chord-picker");
    await expect(picker).toBeVisible();
    const input = picker.locator(".chord-picker-input");
    await input.fill("Dm7");
    await input.press("Enter");

    await expect(picker).toBeHidden();
    await expect(label).toHaveText("Dm7");
  });

  test("the action bar's delete button removes the chord", async ({ page }) => {
    await openEditor(page);

    // Seed data has two "Cm" markers (start of each chorded line) — delete
    // the first and confirm exactly one remains, rather than asserting
    // zero (which would pass even if the wrong one were deleted).
    const cmLabels = page.locator(".chord-marker-label", { hasText: "Cm" });
    await expect(cmLabels).toHaveCount(2);
    await cmLabels.first().click();

    const bar = page.locator(".chord-action-bar");
    await bar.locator(".chord-action-delete").click();

    await expect(cmLabels).toHaveCount(1);
  });

  test("the action bar's move-right button moves the chord one character at a time", async ({ page }) => {
    await openEditor(page);

    // Seeded first line: {Cm} "Oh baby, baby, how " {G/B} ... — clicking
    // move-right 3 times (one per character of "Oh ") should land Cm just
    // before "baby,", not jump the whole word in a single click.
    const marker = page.locator(".chord-marker").filter({ hasText: "Cm" }).first();
    await marker.locator(".chord-marker-label").click();
    const moveRight = page.locator(".chord-action-bar .chord-action-move").filter({ hasText: "▶" });
    for (let i = 0; i < 3; i++) {
      await moveRight.click();
      // The action bar re-opens on the moved marker after each click.
      await expect(page.locator(".chord-action-bar")).toBeVisible();
    }

    const { markerX, wordX } = await page.evaluate(() => {
      const markers = [...document.querySelectorAll(".chord-marker")];
      const cm = markers.find((m) => m.textContent?.trim() === "Cm");
      const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
      let node: Node | null;
      let wordRect: DOMRect | null = null;
      while ((node = walker.nextNode())) {
        const idx = node.textContent?.indexOf("baby,") ?? -1;
        if (idx >= 0) {
          const range = document.createRange();
          range.setStart(node, idx);
          range.setEnd(node, idx + "baby,".length);
          wordRect = range.getBoundingClientRect();
          break;
        }
      }
      return { markerX: cm!.getBoundingClientRect().x, wordX: wordRect!.x };
    });

    expect(Math.abs(markerX - wordX)).toBeLessThan(1);
  });

  test("the action bar's move-left button moves one character at a time, not blocked at the front of a word", async ({ page }) => {
    await openEditor(page);

    // Regression: seeded CHORUS 1 has {Text:"Don't "},{Chord:"Ebm"},
    // {Text:"tell me..."} — Ebm sits at the very front of "tell", right
    // after "Don't ". Moving it left used to be a no-op there (the
    // trailing space on "Don't " was mistaken for an empty word after
    // it). 6 single-character clicks should walk it all the way past
    // "Don't " to the very front of the line.
    const marker = page.locator(".chord-marker").filter({ hasText: "Ebm" }).first();
    await marker.locator(".chord-marker-label").click();
    const moveLeft = page.locator(".chord-action-bar .chord-action-move").filter({ hasText: "◀" });
    for (let i = 0; i < 6; i++) {
      await moveLeft.click();
      await expect(page.locator(".chord-action-bar")).toBeVisible();
    }
    // Nothing left to move past — the button click is a no-op, bar stays.
    await moveLeft.click();

    const { markerX, wordX } = await page.evaluate(() => {
      const markers = [...document.querySelectorAll(".chord-marker")];
      const ebm = markers.find((m) => m.textContent?.trim() === "Ebm");
      const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
      let node: Node | null;
      let wordRect: DOMRect | null = null;
      while ((node = walker.nextNode())) {
        const idx = node.textContent?.indexOf("Don't") ?? -1;
        if (idx >= 0) {
          const range = document.createRange();
          range.setStart(node, idx);
          range.setEnd(node, idx + "Don't".length);
          wordRect = range.getBoundingClientRect();
          break;
        }
      }
      return { markerX: ebm!.getBoundingClientRect().x, wordX: wordRect!.x };
    });

    expect(Math.abs(markerX - wordX)).toBeLessThan(1);
  });

  test("the floating action bar stays open and visible while picking a new chord", async ({ page }) => {
    await openEditor(page);

    const marker = page.locator(".chord-marker").first();
    await marker.locator(".chord-marker-label").click();
    const bar = page.locator(".chord-action-bar");
    await bar.locator(".chord-action-chord").click();

    const picker = page.locator(".chord-picker");
    await expect(picker).toBeVisible();
    // The bar must still be visible (not covered/closed) while the picker
    // is open, and the two popups must not overlap each other.
    await expect(bar).toBeVisible();
    const [barBox, pickerBox] = await Promise.all([bar.boundingBox(), picker.boundingBox()]);
    const overlaps =
      barBox!.y < pickerBox!.y + pickerBox!.height &&
      barBox!.y + barBox!.height > pickerBox!.y &&
      barBox!.x < pickerBox!.x + pickerBox!.width &&
      barBox!.x + barBox!.width > pickerBox!.x;
    expect(overlaps).toBe(false);
  });

  test("inserting a chord via the toolbar button opens the picker at the cursor", async ({ page }) => {
    await openEditor(page);

    const lyricText = page.locator(".line-content").filter({ hasText: "Short line" });
    await lyricText.click();
    await page.getByRole("button", { name: "+ Chord" }).click();

    const picker = page.locator(".chord-picker");
    await expect(picker).toBeVisible();
    await picker.locator(".chord-picker-input").fill("Em");
    await picker.locator(".chord-picker-input").press("Enter");

    await expect(page.locator(".chord-marker-label", { hasText: "Em" })).toBeVisible();
  });

  test("lyric text is natively editable independent of chord markers", async ({ page }) => {
    await openEditor(page);

    const shortLine = page.locator(".line-content").filter({ hasText: "Short line" });
    await shortLine.click();
    await page.keyboard.press("End");
    await page.keyboard.type(" EDITED");

    await expect(shortLine).toContainText("Short line EDITED");
    // The chord marker before it is untouched by the text edit.
    await expect(page.locator(".chord-marker-label").filter({ hasText: "Cm" }).first()).toBeVisible();
  });

  test("save persists edits and they reload correctly", async ({ page }) => {
    await openEditor(page);

    const shortLine = page.locator(".line-content").filter({ hasText: "Short line" });
    await shortLine.click();
    await page.keyboard.press("End");
    await page.keyboard.type(" SAVED-ROUNDTRIP");
    await page.getByRole("button", { name: "Save" }).click();
    await expect(page.locator(".song-editor-status")).toHaveText("Saved");

    await page.reload();
    await page.waitForSelector(".song-editor-surface .line");
    await expect(page.locator(".line-content").filter({ hasText: "SAVED-ROUNDTRIP" })).toBeVisible();
  });

  async function selectTextRange(page: Page, text: string) {
    const rect = await page.evaluate((needle) => {
      const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
      let node: Node | null;
      while ((node = walker.nextNode())) {
        const idx = node.textContent?.indexOf(needle) ?? -1;
        if (idx >= 0) {
          const range = document.createRange();
          range.setStart(node, idx);
          range.setEnd(node, idx + needle.length);
          const r = range.getBoundingClientRect();
          return { left: r.left, top: r.top + r.height / 2, right: r.right };
        }
      }
      return null;
    }, text);
    if (!rect) throw new Error(`text not found: ${text}`);
    await page.mouse.move(rect.left, rect.top);
    await page.mouse.down();
    await page.mouse.move(rect.right, rect.top, { steps: 5 });
    await page.mouse.up();
  }

  test("selecting lyric text shows the floating format toolbar, and Bold wraps it", async ({ page }) => {
    await openEditor(page);

    await selectTextRange(page, "Short line");
    const toolbar = page.locator(".format-toolbar");
    await expect(toolbar).toBeVisible();

    await toolbar.locator(".format-toolbar-btn", { hasText: "B" }).click();
    const shortLine = page.locator(".line-content").filter({ hasText: "Short line" });
    await expect(shortLine.locator("strong")).toHaveText("Short line");
  });

  test("bold formatting persists through save and reload", async ({ page }) => {
    await openEditor(page);

    await selectTextRange(page, "Short line");
    await page.locator(".format-toolbar-btn", { hasText: "B" }).click();
    await page.getByRole("button", { name: "Save" }).click();
    await expect(page.locator(".song-editor-status")).toHaveText("Saved");

    await page.reload();
    await page.waitForSelector(".song-editor-surface .line");
    const shortLine = page.locator(".line-content").filter({ hasText: "Short line" });
    await expect(shortLine.locator("strong")).toHaveText("Short line");
  });

  test("the format toolbar's H button toggles the current line into a section header and back", async ({ page }) => {
    await openEditor(page);

    const titleLine = page.locator(".line-content").filter({ hasText: "E2E Test Song" });
    await titleLine.click();
    await page.keyboard.press("Home");
    await page.keyboard.down("Shift");
    await page.keyboard.press("ArrowRight");
    await page.keyboard.up("Shift");

    const toolbar = page.locator(".format-toolbar");
    await expect(toolbar).toBeVisible();
    await toolbar.locator(".format-toolbar-btn", { hasText: "H" }).click();

    const header = page.locator(".section-header").filter({ hasText: "E2E Test Song" });
    await expect(header).toBeVisible();

    // Toggle back: select inside the now-header line and click H again.
    await header.click();
    await page.keyboard.press("Home");
    await page.keyboard.down("Shift");
    await page.keyboard.press("ArrowRight");
    await page.keyboard.up("Shift");
    await expect(toolbar).toBeVisible();
    await toolbar.locator(".format-toolbar-btn", { hasText: "H" }).click();

    await expect(page.locator(".section-header").filter({ hasText: "E2E Test Song" })).toHaveCount(0);
    await expect(page.locator(".line-content").filter({ hasText: "E2E Test Song" })).toBeVisible();
  });

  test("the format toolbar refuses to convert a chorded line into a section header", async ({ page }) => {
    await openEditor(page);

    await selectTextRange(page, "Short line");
    const toolbar = page.locator(".format-toolbar");
    await expect(toolbar).toBeVisible();
    await toolbar.locator(".format-toolbar-btn", { hasText: "H" }).click();

    // The line still has a chord marker, so it can't become a section
    // header — nothing should change.
    await expect(page.locator(".section-header").filter({ hasText: "Short line" })).toHaveCount(0);
    await expect(page.locator(".chord-marker-label").filter({ hasText: "Cm" }).first()).toBeVisible();
  });
});
