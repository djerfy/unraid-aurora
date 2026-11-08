import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { blurWallpaper } from "../lib/wallpaper";

export type ThemeMode = "dark" | "light" | "system";
export type Accent = "ember" | "violet" | "cyan" | "emerald" | "amber" | "blue";
export type Density = "comfortable" | "compact";

type Preferences = {
  theme: ThemeMode;
  accent: Accent;
  density: Density;
  glass: boolean;
  wallpaper: string;
  wallpaperBlur: string;
  performance: boolean;
};

type ThemeContextValue = Preferences & {
  resolvedTheme: "dark" | "light";
  setTheme: (theme: ThemeMode) => void;
  setAccent: (accent: Accent) => void;
  setDensity: (density: Density) => void;
  setGlass: (glass: boolean) => void;
  setWallpaper: (wallpaper: string, wallpaperBlur?: string) => void;
  setPerformance: (performance: boolean) => void;
  setPreferences: (prefs: Partial<Preferences>) => void;
};

const STORAGE_KEY = "aurora.theme";

const ThemeContext = createContext<ThemeContextValue | null>(null);

function readStored(): Preferences {
  const fallback: Preferences = { theme: "dark", accent: "ember", density: "comfortable", glass: false, wallpaper: "", wallpaperBlur: "", performance: false };
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return fallback;
    const parsed = JSON.parse(raw) as Partial<Preferences>;
    return {
      theme: parsed.theme ?? fallback.theme,
      accent: parsed.accent ?? fallback.accent,
      density: parsed.density ?? fallback.density,
      glass: parsed.glass === true,
      wallpaper: typeof parsed.wallpaper === "string" ? parsed.wallpaper : "",
      wallpaperBlur: typeof parsed.wallpaperBlur === "string" ? parsed.wallpaperBlur : "",
      performance: parsed.performance === true,
    };
  } catch {
    return fallback;
  }
}

function systemTheme(): "dark" | "light" {
  return window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [prefs, setPrefs] = useState<Preferences>(readStored);
  const [system, setSystem] = useState<"dark" | "light">(systemTheme);

  useEffect(() => {
    const media = window.matchMedia("(prefers-color-scheme: light)");
    const onChange = () => setSystem(media.matches ? "light" : "dark");
    media.addEventListener("change", onChange);
    return () => media.removeEventListener("change", onChange);
  }, []);

  const resolvedTheme = prefs.theme === "system" ? system : prefs.theme;

  useEffect(() => {
    const root = document.documentElement;
    root.classList.toggle("dark", resolvedTheme === "dark");
    root.classList.toggle("light", resolvedTheme === "light");
    root.dataset.accent = prefs.accent;
    root.dataset.density = prefs.density;
    root.dataset.glass = prefs.glass ? "on" : "off";
    root.dataset.wallpaper = prefs.wallpaper ? "on" : "off";
    root.dataset.performance = prefs.performance ? "on" : "off";
    root.style.colorScheme = resolvedTheme;
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(prefs));
    } catch {
      // Storage can be unavailable; the UI still works for this session.
    }
  }, [prefs, resolvedTheme]);

  const blurAttempt = useRef("");
  useEffect(() => {
    if (!prefs.glass || !prefs.wallpaper || prefs.wallpaperBlur) return;
    if (blurAttempt.current === prefs.wallpaper) return;
    blurAttempt.current = prefs.wallpaper;
    const source = prefs.wallpaper;
    void blurWallpaper(source)
      .then((blurred) => {
        if (!blurred || blurred === source) return;
        setPrefs((current) => (current.wallpaper === source ? { ...current, wallpaperBlur: blurred } : current));
      })
      .catch(() => {
        // Keep the original wallpaper; runtime glass blur still applies.
      });
  }, [prefs.glass, prefs.wallpaper, prefs.wallpaperBlur]);

  const setTheme = useCallback((theme: ThemeMode) => setPrefs((p) => ({ ...p, theme })), []);
  const setAccent = useCallback((accent: Accent) => setPrefs((p) => ({ ...p, accent })), []);
  const setDensity = useCallback((density: Density) => setPrefs((p) => ({ ...p, density })), []);
  const setGlass = useCallback((glass: boolean) => setPrefs((p) => ({ ...p, glass })), []);
  const setWallpaper = useCallback(
    (wallpaper: string, wallpaperBlur = "") => setPrefs((p) => ({ ...p, wallpaper, wallpaperBlur })),
    [],
  );
  const setPerformance = useCallback((performance: boolean) => setPrefs((p) => ({ ...p, performance })), []);
  const setPreferences = useCallback(
    (next: Partial<Preferences>) => setPrefs((p) => ({ ...p, ...next })),
    [],
  );

  const value = useMemo(
    () => ({ ...prefs, resolvedTheme, setTheme, setAccent, setDensity, setGlass, setWallpaper, setPerformance, setPreferences }),
    [prefs, resolvedTheme, setTheme, setAccent, setDensity, setGlass, setWallpaper, setPerformance, setPreferences],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme() {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error("useTheme must be used inside ThemeProvider");
  return ctx;
}
