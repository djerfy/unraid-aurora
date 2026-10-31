// Apply the stored theme before first paint to avoid a flash of the wrong theme.
(() => {
  try {
    const raw = localStorage.getItem("aurora.theme");
    const parsed = raw ? JSON.parse(raw) : null;
    const mode = parsed && parsed.theme ? parsed.theme : "dark";
    const systemLight = window.matchMedia("(prefers-color-scheme: light)").matches;
    const dark = mode === "dark" || (mode === "system" && !systemLight);
    document.documentElement.classList.toggle("dark", dark);
    document.documentElement.classList.toggle("light", !dark);
    if (parsed && parsed.accent) document.documentElement.dataset.accent = parsed.accent;
    if (parsed && parsed.density) document.documentElement.dataset.density = parsed.density;
  } catch {
    document.documentElement.classList.add("dark");
  }
})();
