// Ссылка на конкретный инстанс задаётся только при сборке документации.
export function resolveProductUrl(value: unknown): string | undefined {
  if (typeof value !== "string" || !value.trim()) return undefined;
  try {
    const url = new URL(value.trim());
    if (
      !["https:", "http:"].includes(url.protocol) ||
      url.username ||
      url.password
    )
      return undefined;
    return url.href;
  } catch {
    return undefined;
  }
}
export const productUrl = resolveProductUrl(import.meta.env.VITE_PRODUCT_URL);
