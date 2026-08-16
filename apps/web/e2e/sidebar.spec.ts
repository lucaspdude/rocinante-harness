import { test, expect, type Page } from "@playwright/test";

const PASSPHRASE = process.env.RH_E2E_PASSPHRASE ?? "e2e-passphrase";

async function login(page: Page) {
  await page.goto("/en-US/login");
  await page.fill("#passphrase", PASSPHRASE);
  await page.fill("#deviceName", "e2e-sidebar");
  await page.click('button[type="submit"]');
  await page.waitForURL(/\/(en-US|pt-BR)$/);
}

test.describe("sidebar", () => {
  test("sidebar renders with title", async ({ page }) => {
    await login(page);
    await expect(page.locator("aside h2")).toBeVisible();
  });

  test("creates a new session and lists it", async ({ page }) => {
    await login(page);
    await page.click("aside button[title='New session']");
    await page.waitForURL(/\/agent\//);
    await expect(page.locator("aside li[data-active='true']")).toBeVisible();
  });

  test("deletes a session from the sidebar", async ({ page }) => {
    await login(page);
    await page.click("aside button[title='New session']");
    await page.waitForURL(/\/agent\//);
    const before = await page.locator("aside li[data-active]").count();
    await page.click("aside li[data-active] button[aria-label='delete']");
    await page.waitForFunction(
      (n) => document.querySelectorAll("aside li[data-active]").length < n,
      before
    );
  });
});
