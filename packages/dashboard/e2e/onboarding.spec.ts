import { test, expect } from "@playwright/test";

test.describe("Onboarding flow", () => {
  test("completes all 4 steps", async ({ page }) => {
    await page.goto("/onboarding");
    await expect(page.getByRole("heading", { name: "Get started" })).toBeVisible();

    // Step 1: Create project
    await page.getByTestId("project-name").fill("My test project");
    await page.click('[data-analytics-id="onboarding-create-project"]');

    // Step 2: Add endpoint
    await expect(page.getByRole("heading", { name: "Add endpoints" })).toBeVisible();
    await page.click('[data-analytics-id="onboarding-add-endpoint"]');

    // Step 3: Copy API key
    await expect(page.getByTestId("api-key")).toBeVisible();
    await page.click('[data-analytics-id="onboarding-copy-key"]');

    // Step 4: Done button visible
    await expect(page.getByRole("button", { name: /Go to dashboard/i })).toBeVisible();
  });
});
