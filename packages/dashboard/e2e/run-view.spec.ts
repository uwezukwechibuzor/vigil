import { test, expect } from "@playwright/test";

test.describe("Agent run trace view", () => {
  test("shows trace waterfall for a run", async ({ page }) => {
    await page.goto("/runs/run-demo-123");
    await expect(page.getByRole("heading", { level: 1 })).toContainText("Run");
    await expect(page.locator('[data-analytics-id="run-trace-waterfall"]')).toBeVisible();
    const spans = page.locator('[data-analytics-id="run-trace-waterfall"] > div');
    await expect(spans).toHaveCount(4);
  });
});
