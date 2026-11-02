import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { useApp } from "../app/AppDataProvider";
import { fr } from "./locales/fr";
import { setFormatLocale } from "../lib/format";

export type Locale = "en" | "fr";
export type TranslationParams = Record<string, string | number>;

type I18nValue = {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: string, params?: TranslationParams) => string;
};

const STORAGE_KEY = "aurora.locale";
const I18nContext = createContext<I18nValue | null>(null);

function normalizeLocale(value: unknown): Locale {
  return value === "fr" ? "fr" : "en";
}

function storedLocale(): Locale {
  try {
    return normalizeLocale(localStorage.getItem(STORAGE_KEY));
  } catch {
    return "en";
  }
}

function interpolate(text: string, params?: TranslationParams): string {
  if (!params) return text;
  let result = text;
  for (const [key, value] of Object.entries(params)) {
    result = result.replaceAll(`{${key}}`, String(value));
  }
  return result;
}

/**
 * English source strings are the translation keys, so an untranslated string
 * always falls back to readable English instead of a key such as "page.title".
 */
export function I18nProvider({ children }: { children: ReactNode }) {
  const { snapshot } = useApp();
  const [locale, setLocaleState] = useState<Locale>(storedLocale);

  useEffect(() => {
    if (!snapshot?.settings.language) return;
    const next = normalizeLocale(snapshot.settings.language);
    setLocaleState(next);
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch {
      // Private browsing or a blocked storage API: the in-memory locale still works.
    }
  }, [snapshot?.settings.language]);

  useEffect(() => {
    document.documentElement.lang = locale;
    setFormatLocale(locale);
  }, [locale]);

  const setLocale = useCallback((next: Locale) => {
    setLocaleState(next);
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch {
      // Nothing to persist; the current session keeps the requested locale.
    }
  }, []);

  const t = useCallback(
    (key: string, params?: TranslationParams) =>
      interpolate(locale === "fr" ? fr[key] ?? key : key, params),
    [locale],
  );

  // Keep non-React formatters in sync before children render.
  setFormatLocale(locale);

  const value = useMemo(() => ({ locale, setLocale, t }), [locale, setLocale, t]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nValue {
  const value = useContext(I18nContext);
  if (!value) throw new Error("useI18n must be used inside I18nProvider");
  return value;
}
