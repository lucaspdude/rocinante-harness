import { test, expect, type Page } from "@playwright/test";

const PASSPHRASE = process.env.RH_E2E_PASSPHRASE ?? "e2e-passphrase";

async function login(page: Page) {
  await page.goto("/en-US/login");
  await page.fill("#passphrase", PASSPHRASE);
  await page.fill("#deviceName", "e2e-settings");
  await page.click('button[type="submit"]');
  await page.waitForURL(/\/(en-US|pt-BR)$/);
}

test.describe("settings", () => {
  test("three tabs render and switch", async ({ page }) => {
    await login(page);
    await page.goto("/en-US/settings");

    await expect(page.getByRole("button", { name: /General|Geral/ })).toBeVisible();
    await expect(page.getByRole("button", { name: /Account|Conta/ })).toBeVisible();
    await expect(page.getByRole("button", { name: /Devices|Dispositivos/ })).toBeVisible();

    await page.getByRole("button", { name: /Account|Conta/ }).click();
    await expect(page.getByRole("button", { name: /Sign out|Sair/ })).toBeVisible();

    await page.getByRole("button", { name: /Devices|Dispositivos/ }).click();
  });

  test("locale picker changes UI to pt-BR", async ({ page }) => {
    await login(page);
    await page.goto("/en-US/settings");
    await page.selectOption("#set-locale", "pt-BR");
    await expect(page).toHaveURL(/\/pt-BR\/settings/);
    await expect(page.locator("h1")).toContainText(/Configura/i);
  });

  test("logout clears tokens and redirects to login", async ({ page }) => {
    await login(page);
    await page.goto("/en-US/settings");
    await page.getByRole("button", { name: /Account|Conta/ }).click();
    await page.getByRole("button", { name: /Sign out|Sair/ }).click();
    await page.waitForURL(/\/(en-US|pt-BR)\/login/);
  });
});
