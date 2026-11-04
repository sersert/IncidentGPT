// Синхронный скрипт задаёт тему до загрузки стилей и React, в том числе с script-src 'self'.
(() => {
  let saved
  try { saved = localStorage.getItem('incidentgpt-theme') } catch {}
  const theme = saved === 'light' || saved === 'dark'
    ? saved : matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  document.documentElement.dataset.theme = theme
  document.documentElement.style.colorScheme = theme
})()
