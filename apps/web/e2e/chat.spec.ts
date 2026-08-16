import { test, expect, type Page } from "@playwright/test";

const PASSPHRASE = process.env.RH_E2E_PASSPHRASE ?? "e2e-passphrase";

async function login(page: Page) {
  await page.goto("/en-US/login");
  await page.fill("#passphrase", PASSPHRASE);
  await page.fill("#deviceName", "e2e-chat");
  await page.click('button[type="submit"]');
  await page.waitForURL(/\/(en-US|pt-BR)$/);
}

async function openFirstSession(page: Page) {
  await page.click("aside button[title='New session']");
  await page.waitForURL(/\/agent\//);
}

test.describe("chat", () => {
  test("login → new session → composer is visible", async ({ page }) => {
    await login(page);
    await openFirstSession(page);

    await expect(page.locator("textarea")).toBeVisible();
    await expect(page.getByRole("button", { name: /Send|Enviar/ })).toBeVisible();
  });

  test("send → message appears in the log", async ({ page }) => {
    await login(page);
    await openFirstSession(page);

    const textarea = page.locator("textarea");
    await textarea.fill("hello world");
    await page.getByRole("button", { name: /Send|Enviar/ }).click();

    await expect(page.locator("[data-testid='message'][data-role='user']"))
      .toContainText("hello world");
  });

  test("stop button appears while streaming", async ({ page }) => {
    await login(page);
    await openFirstSession(page);

    const textarea = page.locator("textarea");
    await textarea.fill("long prompt");
    await page.getByRole("button", { name: /Send|Enviar/ }).click();

    await expect(page.getByRole("button", { name: /Stop|Parar/ }))
      .toBeVisible({ timeout: 5_000 });
  });
});
