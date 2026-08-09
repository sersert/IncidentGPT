/**
 * Русское склонение по числу: 1 алерт, 2 алерта, 5 алертов.
 * Без него интерфейс пишет «3 алертов», и это сразу видно.
 */
export function plural(n: number, one: string, few: string, many: string): string {
  const mod10 = n % 10
  const mod100 = n % 100
  if (mod10 === 1 && mod100 !== 11) return one
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return few
  return many
}

/** Число вместе со склонённым словом: «3 алерта». */
export function count(n: number, one: string, few: string, many: string): string {
  return `${n} ${plural(n, one, few, many)}`
}

export const words = {
  alert: ['алерт', 'алерта', 'алертов'] as const,
  incident: ['инцидент', 'инцидента', 'инцидентов'] as const,
  resource: ['ресурс', 'ресурса', 'ресурсов'] as const,
  critical: ['критический', 'критических', 'критических'] as const,
  active: ['активный', 'активных', 'активных'] as const,
  analysis: ['разбор', 'разбора', 'разборов'] as const,
}

/** Удобная обёртка: countOf(3, words.alert) → «3 алерта». */
export function countOf(n: number, w: readonly [string, string, string]): string {
  return count(n, w[0], w[1], w[2])
}
