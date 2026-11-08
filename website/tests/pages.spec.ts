import { test, expect } from "@playwright/test";

for (const language of ["en", "ru"]) {
  test(`${language}: production routes and assets`, async ({
    page,
  }, testInfo) => {
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    page.on("response", (response) => {
      if (response.status() >= 400)
        errors.push(`${response.status()} ${response.url()}`);
    });
    for (const route of [
      "",
      "installation",
      "configuration",
      "web-ui",
      "logs",
      "examples",
      "troubleshooting",
      "limitations",
      "architecture",
    ]) {
      await page.goto(`./#/${language}/${route}`);
      await expect(page.locator("html")).toHaveAttribute("lang", language);
      await expect(page.locator("h1")).toBeVisible();
      await expect(page.locator("h1")).not.toBeEmpty();
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      ).toBe(true);
      if (route === "" || route === "web-ui")
        await page.screenshot({
          path: testInfo.outputPath(`${language}-${route || "home"}.png`),
          fullPage: true,
        });
      if (route === "architecture")
        await expect(page.locator(".diagram svg")).toHaveCount(2);
    }
    expect(errors).toEqual([]);
  });
}

test("example tabs work with keyboard and remain local", async ({ page }) => {
  await page.goto("./#/en");
  await page.getByRole("tab", { name: "Investigation", exact: true }).click();
  await expect(page.getByRole("tabpanel")).toContainText("Build the timeline");
  await page.keyboard.press("End");
  await expect(
    page.getByRole("tab", { name: "Handoff", exact: true }),
  ).toBeFocused();
  await expect(page.getByRole("tabpanel")).toContainText(
    "Prepare the next shift",
  );
  await page.keyboard.press("Home");
  await expect(
    page.getByRole("tab", { name: "Overview", exact: true }),
  ).toHaveAttribute("aria-selected", "true");
  await expect(page).toHaveURL(/#\/en$/);
});

test("theme persists and back-to-top preserves the document route", async ({
  page,
}) => {
  await page.goto("./#/en/installation");
  const theme = page.locator(".header-tools button[title]");
  await theme.click();
  const selected = await page.locator("html").getAttribute("data-theme");
  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("data-theme", selected!);
  await page.getByRole("button", { name: "Back to top", exact: true }).click();
  await expect(page).toHaveURL(/#\/en\/installation$/);
});

test("navigation, search and language switching", async ({
  page,
  isMobile,
}) => {
  await page.goto("./#/en/installation");
  if (isMobile) {
    const opener = page.getByRole("button", { name: "Open menu", exact: true });
    await opener.click();
    await expect(page.getByRole("dialog")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await expect(opener).toBeFocused();
    await opener.click();
  }
  const scope = isMobile
    ? page.getByRole("dialog")
    : page.locator(".header-tools");
  await scope.getByRole("button", { name: "RU", exact: true }).click();
  await expect(page).toHaveURL(/#\/ru\/installation$/);
  const search = scope.getByRole("searchbox");
  await search.fill("Redis");
  await scope.locator(".search-results a").first().click();
  if (isMobile) await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(page.locator("html")).toHaveAttribute("lang", "ru");
});

test("contents links scroll without changing hash route", async ({
  page,
  isMobile,
}) => {
  test.skip(isMobile, "Contents rail is desktop-only");
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("./#/en/installation");
  await page.locator(".toc a").last().click();
  await expect(page).toHaveURL(/#\/en\/installation\?s=verify$/);
  await expect(page.locator("#verify")).toBeInViewport();
});

test("themes remain usable when localStorage is disabled", async ({ page }) => {
  await page.addInitScript(() => {
    Object.defineProperty(window, "localStorage", {
      get() {
        throw new Error("Storage disabled");
      },
    });
  });
  await page.goto("./#/en");
  await expect(page.locator("h1")).toBeVisible();
  await page.locator(".header-tools button[title]").click();
  await expect(page.locator("html")).toHaveAttribute(
    "data-theme",
    /light|dark/,
  );
});
