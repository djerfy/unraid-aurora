import { expect, test } from "@playwright/test";

test("login page renders without a session", async ({ page }) => {
  await page.route("**/api/v1/auth/session", (route) =>
    route.fulfill({ json: { authenticated: false, user: "" } }),
  );
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Welcome back" })).toBeVisible();
  await expect(page.getByLabel("Username")).toHaveValue("root");
  await expect(page.getByRole("button", { name: "Sign in" })).toBeVisible();
});
