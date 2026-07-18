import { test, expect } from "@playwright/test";

// The transpose stepper (static/js/transpose.js) only exists once JS runs
// against server-rendered .chord spans — nothing to assert from a plain
// HTTP response, so this is exactly the Playwright case per
// docs/testing-strategy.md. Uses the seeded e2e-test-song's real chords
// (Cm, G/B, Eb, Fm, Ebm, Db, Ab — see cmd/e2eseed) rather than synthetic
// ones, and no Key line, so spelling falls back to the sharps-by-default
// convention.
test.describe("transpose", () => {
  test("shifts every chord by a semitone and updates the key readout", async ({ page }) => {
    await page.goto("/songs/e2e-test-song");
    await expect(page.locator(".chord").first()).toHaveText("Cm");

    await page.click(".transpose-up");

    await expect(page.locator(".chord").first()).toHaveText("Dbm");
    await expect(page.locator(".transpose-key")).toHaveText("+1");
  });

  test("round-trips back to the exact original spelling at net-zero", async ({ page }) => {
    await page.goto("/songs/e2e-test-song");
    const before = await page.locator(".chord").allTextContents();

    await page.click(".transpose-up");
    await page.click(".transpose-up");
    await page.click(".transpose-down");
    await page.click(".transpose-down");

    const after = await page.locator(".chord").allTextContents();
    expect(after).toEqual(before);
    await expect(page.locator(".transpose-key")).toHaveText("0");
  });

  test("also transposes chords in Play mode", async ({ page }) => {
    await page.goto("/songs/e2e-test-song/play");
    await expect(page.locator(".chord").first()).toHaveText("Cm");

    await page.click(".transpose-down");

    await expect(page.locator(".chord").first()).toHaveText("Bm");
  });

  test("reflects the semitone offset in the URL and reapplies it on direct load", async ({ page }) => {
    await page.goto("/songs/e2e-test-song");

    await page.click(".transpose-up");
    await page.click(".transpose-up");
    await expect(page).toHaveURL(/\?t=2$/);

    // A fresh load of that same URL (e.g. a bookmarked/shared link)
    // should reapply the offset immediately, not reset to the original.
    await page.goto(page.url());
    await expect(page.locator(".chord").first()).toHaveText("Dm");
    await expect(page.locator(".transpose-key")).toHaveText("+2");
  });

  test("carries the transposition from the show page into Play mode and back", async ({ page }) => {
    await page.goto("/songs/e2e-test-song");
    await page.click(".transpose-up");
    await page.click(".transpose-up");

    await expect(page.locator(".play-affordance a")).toHaveAttribute("href", /\?t=2$/);

    await page.click(".play-affordance a");
    await expect(page).toHaveURL(/\/play\?t=2$/);
    await expect(page.locator(".chord").first()).toHaveText("Dm");

    await page.click(".play-close");
    await expect(page).toHaveURL(/\/songs\/e2e-test-song\?t=2$/);
    await expect(page.locator(".chord").first()).toHaveText("Dm");
  });
});
