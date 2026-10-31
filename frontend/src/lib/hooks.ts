import { useEffect, useState } from "react";

export function useHotkeys(handlers: Record<string, (event: KeyboardEvent) => void>) {
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const key = [
        event.metaKey ? "mod" : "",
        event.ctrlKey && !event.metaKey ? "ctrl" : "",
        event.altKey ? "alt" : "",
        event.shiftKey ? "shift" : "",
        event.key.toLowerCase(),
      ]
        .filter(Boolean)
        .join("+");
      const handler = handlers[key] ?? handlers[event.key.toLowerCase()];
      if (handler) handler(event);
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [handlers]);
}

export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(() => window.matchMedia(query).matches);

  useEffect(() => {
    const media = window.matchMedia(query);
    const onChange = () => setMatches(media.matches);
    media.addEventListener("change", onChange);
    setMatches(media.matches);
    return () => media.removeEventListener("change", onChange);
  }, [query]);

  return matches;
}

export function usePageTitle(title: string) {
  useEffect(() => {
    document.title = `${title} · Aurora`;
    return () => {
      document.title = "Aurora · Unraid";
    };
  }, [title]);
}

export function useClickOutside<T extends HTMLElement>(
  ref: React.RefObject<T | null>,
  onOutside: () => void,
  active = true,
) {
  useEffect(() => {
    if (!active) return;
    const handler = (event: MouseEvent) => {
      if (ref.current && !ref.current.contains(event.target as Node)) onOutside();
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [ref, onOutside, active]);
}
