import { test, expect } from "@playwright/test";

test.describe("Alert rule creation", () => {
  test("creates an alert rule", async ({ page }) => {
    await page.goto("/alert-rules/new");
    await expect(page.getByRole("heading", { name: "New alert rule" })).toBeVisible();

    await page.getByTestId("rule-name").fill("RPC latency spike");
    await page.getByTestId("rule-severity").selectOption("high");
    await page.getByTestId("save-rule").click();

    await expect(page.getByTestId("rule-saved")).toBeVisible();
    await expect(page.getByTestId("rule-saved")).toContainText("RPC latency spike");
  });
});
