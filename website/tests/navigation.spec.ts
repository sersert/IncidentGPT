import { test, expect } from '@playwright/test';

test('section URLs work after reload and in a new tab', async ({ page }) => {
  await page.goto('./#/ru/configuration?s=generator');
  await expect(page.getByRole('heading', { name: 'Генератор Helm values', exact: true })).toBeInViewport();
  // The sidebar TOC is present on desktop. Mobile direct links still retain their section.
  if (await page.getByRole('navigation', { name: 'На этой странице' }).isVisible()) {
    await page.getByRole('navigation', { name: 'На этой странице' }).getByRole('link', { name: 'Runbooks', exact: true }).click();
    await expect(page).toHaveURL(/#\/ru\/configuration\?s=runbooks$/);
  }
  const url = page.url();
  await page.reload();
  await expect(page).toHaveURL(url);
  const heading = page.getByRole('heading', { name: url.includes('runbooks') ? 'Runbooks' : 'Генератор Helm values', exact: true });
  await expect(heading).toBeInViewport();
  const tab = await page.context().newPage();
  await tab.goto(url);
  await expect(tab.getByRole('heading', { name: url.includes('runbooks') ? 'Runbooks' : 'Генератор Helm values', exact: true })).toBeInViewport();
  await tab.close();
});

test('mobile dialog restores trigger focus after Escape and navigation', async ({ page }, info) => {
  test.skip(info.project.name !== 'mobile');
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.goto('./#/ru');
  const trigger = page.getByRole('button', { name: 'Открыть меню', exact: true });
  await trigger.click();
  const dialog = page.getByRole('dialog', { name: 'Навигация', exact: true });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole('button', { name: 'Закрыть меню' })).toBeFocused();
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
  await expect(trigger).toBeFocused();
  await trigger.click();
  await dialog.getByRole('link', { name: 'Конфигурация', exact: true }).click();
  await expect(page).toHaveURL(/#\/ru\/configuration$/);
  await expect(trigger).toBeFocused();
  expect(errors).toEqual([]);
});
