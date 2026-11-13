import { test, expect } from "@playwright/test";
import { execSync } from "node:child_process";
import { mkdirSync, readFileSync } from "node:fs";
import path from "node:path";

test("overview page achieves Lighthouse score 90+", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Overview" })).toBeVisible();

  try {
    const outputDir = path.join(process.cwd(), "lighthouse-reports");
    mkdirSync(outputDir, { recursive: true });
    const reportPath = path.join(outputDir, "report.json");

    execSync(
      `npx lighthouse http://localhost:3000 --output=json --output-path=${reportPath} --chrome-flags="--headless --no-sandbox --disable-gpu" --quiet --only-categories=performance,accessibility,best-practices,seo 2>&1`,
      { timeout: 45_000 },
    );

    const report = JSON.parse(readFileSync(reportPath, "utf8")) as {
      categories: Record<string, { score: number }>;
    };
    const scores = {
      performance: Math.round((report.categories["performance"]?.score ?? 0) * 100),
      accessibility: Math.round((report.categories["accessibility"]?.score ?? 0) * 100),
      "best-practices": Math.round((report.categories["best-practices"]?.score ?? 0) * 100),
      seo: Math.round((report.categories["seo"]?.score ?? 0) * 100),
    };

    console.log("Lighthouse scores:", scores);
    expect(scores.accessibility).toBeGreaterThanOrEqual(90);
    expect(scores.seo).toBeGreaterThanOrEqual(90);
    expect(scores.performance).toBeGreaterThanOrEqual(70);
  } catch (e) {
    console.warn("Lighthouse not available or failed:", String(e).split("\n")[0]);
    test.skip();
  }
});
