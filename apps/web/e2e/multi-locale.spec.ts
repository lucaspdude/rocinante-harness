import { test, expect } from "@playwright/test";

test.describe("multi-locale", () => {
  test("root redirects to /en-US", async ({ page }) => {
    await page.goto("/");
    await expect(page).toHaveURL(/\/en-US/);
  });

  test("en-US login renders english strings", async ({ page }) => {
    await page.goto("/en-US/login");
    await expect(page.locator("h1")).toContainText(/Sign in/);
    await expect(page.locator('button[type="submit"]')).toContainText(/Sign in/);
  });

  test("pt-BR login renders portuguese strings", async ({ page }) => {
    await page.goto("/pt-BR/login");
    await expect(page.locator("h1")).toContainText(/Entrar/);
    await expect(page.locator('button[type="submit"]')).toContainText(/Entrar/);
  });

  test("Accept-Language negotiation picks pt-BR for pt-*", async ({
    page,
    context,
  }) => {
    await context.setExtraHTTPHeaders({ "Accept-Language": "pt-BR,pt;q=0.9" });
    await page.goto("/login");
    await expect(page).toHaveURL(/\/pt-BR\/login/);
  });

  test("Accept-Language negotiation picks en-US for en-*", async ({
    page,
    context,
  }) => {
    await context.setExtraHTTPHeaders({ "Accept-Language": "en-US,en;q=0.9" });
    await page.goto("/login");
    await expect(page).toHaveURL(/\/en-US\/login/);
  });
});
