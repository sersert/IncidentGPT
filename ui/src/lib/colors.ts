/**
 * Recharts принимает цвета строкой в пропах fill/stroke, поэтому классы Tailwind
 * туда не годятся. Читаем те же токены из :root — один источник правды с index.css,
 * никаких хекс-литералов по компонентам.
 */
// Значения дублируют токены из index.css. Нужны как запасной вариант: если
// getComputedStyle вернёт пусто (стили ещё не применились), Recharts получит
// пустую заливку и нарисует график без столбцов и линий.
const fallback: Record<string, string> = {
  '--color-severity-critical': '#B92F35',
  '--color-severity-warning': '#8C5B13',
  '--color-severity-info': '#2C5DB2',
  '--color-severity-unknown': '#626D7E',
  '--color-accent': '#3456C5',
  '--color-status-resolved': '#24734F',
  '--color-border-soft': '#E9ECF1',
  '--color-text-muted': '#667388',
  '--color-surface-tertiary': '#EEF1F5',
  '--color-border-default': '#DDE2E9',
}

function token(name: string): string {
  if (typeof window !== 'undefined') {
    const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
    if (v) return v
  }
  return fallback[name] ?? '#3456C5'
}

export const chartColors = {
  get critical() { return token('--color-severity-critical') },
  get warning() { return token('--color-severity-warning') },
  get info() { return token('--color-severity-info') },
  get unknown() { return token('--color-severity-unknown') },
  get accent() { return token('--color-accent') },
  get resolved() { return token('--color-status-resolved') },
  get grid() { return token('--color-border-soft') },
  get axis() { return token('--color-text-muted') },
  get surface() { return token('--color-surface-tertiary') },
  get border() { return token('--color-border-default') },
}

/** Единый стиль тултипа Recharts во всех графиках. */
export const tooltipStyle = {
  background: 'var(--color-surface-tertiary)',
  border: '1px solid var(--color-border-default)',
  borderRadius: 8,
  fontSize: 12,
  padding: '8px 10px',
  boxShadow: 'var(--shadow-chart-tooltip)',
} as const

export const axisTick = { fill: 'var(--color-text-muted)', fontSize: 11 } as const
