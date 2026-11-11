(() => {
  let theme = "system";
  try { theme = localStorage.getItem("incidentgpt-theme") || "system"; } catch { /* Storage may be disabled. */ }
  document.documentElement.dataset.theme = theme === "light" || theme === "dark" ? theme : matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
})();
