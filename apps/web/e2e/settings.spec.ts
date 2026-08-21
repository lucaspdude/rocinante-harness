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
  test("modal opens with four rail sections and switches content", async ({
    page,
  }) => {
    await login(page);
    await page.goto("/en-US/settings");

    // Modal is open + shows all four rail items.
    await expect(page.getByTestId("rh-settings-modal")).toBeVisible();
    await expect(page.getByTestId("rh-settings-rail-general")).toBeVisible();
    await expect(page.getByTestId("rh-settings-rail-providers")).toBeVisible();
    await expect(page.getByTestId("rh-settings-rail-account")).toBeVisible();
    await expect(page.getByTestId("rh-settings-rail-developer")).toBeVisible();

    // Account section renders the sign-out button (the Devices list
    // is nested inside the Account section in PR-07).
    await page.getByTestId("rh-settings-rail-account").click();
    await expect(page.getByTestId("rh-settings-sign-out")).toBeVisible();
  });

  test("locale picker changes UI to pt-BR", async ({ page }) => {
    await login(page);
    await page.goto("/en-US/settings");
    // The locale picker still lives in the General section row.
    await expect(page.getByTestId("rh-settings-modal")).toBeVisible();
    await page.selectOption("#set-locale", "pt-BR");
    await expect(page).toHaveURL(/\/pt-BR\/settings/);
    await expect(
      page.getByTestId("rh-settings-modal").locator("h1"),
    ).toContainText(/Configura/i);
  });

  test("deep link ?section=account opens the account section", async ({
    page,
  }) => {
    await login(page);
    await page.goto("/en-US/settings?section=account");
    await expect(page.getByTestId("rh-settings-modal")).toBeVisible();
    await expect(page.getByTestId("rh-settings-rail-account")).toHaveAttribute(
      "data-active",
      "true",
    );
  });

  test("Esc closes the modal and returns to the home page", async ({
    page,
  }) => {
    await login(page);
    await page.goto("/en-US/settings");
    await expect(page.getByTestId("rh-settings-modal")).toBeVisible();
    await page.keyboard.press("Escape");
    await page.waitForURL(/\/(en-US|pt-BR)$/);
  });

  test("logout clears tokens and redirects to login", async ({ page }) => {
    await login(page);
    await page.goto("/en-US/settings");
    await page.getByTestId("rh-settings-rail-account").click();
    await page.getByTestId("rh-settings-sign-out").click();
    await page.waitForURL(/\/(en-US|pt-BR)\/login/);
  });
});
