import { useEffect, useState, type ReactNode } from "react";
import { initials } from "../../lib/format";
import { cn } from "./primitives";

/** Image with a coloured monogram/icon fallback, used for containers and VMs. */
export function EntityIcon({
  src,
  name,
  color,
  size = 40,
  className,
  fallback,
}: {
  src?: string;
  name: string;
  color: string;
  size?: number;
  className?: string;
  fallback?: ReactNode;
}) {
  const [failed, setFailed] = useState(false);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    setFailed(false);
    setLoaded(false);
  }, [src]);

  return (
    <span
      className={cn(
        "relative grid shrink-0 place-items-center overflow-hidden rounded-lg border",
        className,
      )}
      style={{
        width: size,
        height: size,
        color,
        backgroundColor: `color-mix(in srgb, ${color} 11%, var(--surface-2))`,
        borderColor: `color-mix(in srgb, ${color} 22%, transparent)`,
      }}
    >
      {!loaded || failed || !src ? (
        <span className="absolute inset-0 grid place-items-center" aria-hidden="true">
          {fallback ?? (
            <span style={{ fontSize: Math.max(9, size * 0.3), fontWeight: 700 }}>
              {initials(name)}
            </span>
          )}
        </span>
      ) : null}
      {src && !failed ? (
        <img
          src={src}
          alt=""
          loading="lazy"
          className={cn(
            "relative z-10 size-full object-contain p-[14%] transition-opacity duration-200",
            loaded ? "opacity-100" : "opacity-0",
          )}
          onLoad={() => setLoaded(true)}
          onError={() => setFailed(true)}
        />
      ) : null}
    </span>
  );
}

export function containerIconURL(id: string) {
  return `/api/v1/docker/containers/${encodeURIComponent(id)}/icon`;
}

export function vmIconURL(id: string) {
  return `/api/v1/vms/${encodeURIComponent(id)}/icon`;
}

export function userIconURL(name: string) {
  return `/api/v1/users/${encodeURIComponent(name)}/icon`;
}
