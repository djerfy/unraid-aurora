import { useState, type FormEvent } from "react";
import { Navigate, useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import {
  ArrowRight,
  Eye,
  EyeOff,
  LockKeyhole,
  Moon,
  ShieldCheck,
  Sun,
  User,
} from "lucide-react";
import { AuroraMark, AuroraWordmark } from "../components/layout/Brand";
import { Button, cn } from "../components/ui/primitives";
import { useApp } from "../app/AppDataProvider";
import { useTheme } from "../app/ThemeProvider";
import { useToast } from "../app/ToastProvider";
import { api, ApiError } from "../lib/api";

export function LoginPage() {
  const { session } = useApp();
  const { resolvedTheme, setTheme } = useTheme();
  const { toast } = useToast();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const [username, setUsername] = useState("root");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  if (session?.authenticated) {
    return <Navigate to="/" replace />;
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (loading) return;
    setError("");
    setLoading(true);
    try {
      await api.login(username.trim(), password);
      await queryClient.invalidateQueries({ queryKey: ["session"] });
      await queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
      toast({ kind: "success", title: "Welcome back", description: "Your session is now active." });
      navigate("/", { replace: true });
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.status === 429 ? "Too many attempts. Try again in 15 minutes." : err.message);
      } else {
        setError("Unable to reach the server. Check your connection and try again.");
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="relative grid min-h-dvh grid-cols-1 bg-[var(--bg)] lg:grid-cols-[1.05fr_1fr]">
      {/* Visual panel --------------------------------------------------- */}
      <section className="relative hidden overflow-hidden bg-[#06070b] lg:flex lg:flex-col lg:justify-between lg:p-12">
        <div className="pointer-events-none absolute inset-0">
          <div className="absolute -left-40 -top-40 size-[640px] rounded-full bg-[radial-gradient(circle,rgba(255,107,74,0.22),transparent_65%)] blur-2xl" />
          <div className="absolute -bottom-52 right-[-10%] size-[620px] rounded-full bg-[radial-gradient(circle,rgba(155,123,255,0.18),transparent_65%)] blur-2xl" />
          <div className="absolute inset-0 bg-[linear-gradient(rgba(255,255,255,0.035)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.035)_1px,transparent_1px)] bg-[size:64px_64px] [mask-image:radial-gradient(ellipse_70%_70%_at_40%_40%,black,transparent)]" />
        </div>

        <div className="relative flex items-center justify-between">
          <div className="[&_p]:!text-white [&_p:last-child]:!text-white/40">
            <AuroraWordmark />
          </div>
          <span className="flex items-center gap-1.5 rounded-full border border-white/10 bg-white/[0.04] px-2.5 py-1 text-[10px] font-medium text-white/60 backdrop-blur">
            <span className="size-1.5 rounded-full bg-[#3ddc97]" /> Control plane online
          </span>
        </div>

        <div className="relative mx-auto flex w-full max-w-[520px] flex-col items-center">
          <div className="relative grid size-[280px] place-items-center">
            <div className="absolute inset-0 rounded-full border border-white/[0.06]" />
            <div className="absolute inset-8 rounded-full border border-white/[0.08]" />
            <div className="absolute inset-16 rounded-full border border-white/[0.1]" />
            <div className="absolute inset-0 animate-spin-slow rounded-full">
              <span className="absolute left-1/2 top-0 size-2 -translate-x-1/2 -translate-y-1/2 rounded-full bg-[#ff6b4a] shadow-[0_0_18px_4px_rgba(255,107,74,0.5)]" />
            </div>
            <div className="absolute inset-8 animate-[spin-slow_18s_linear_infinite_reverse] rounded-full">
              <span className="absolute bottom-0 left-1/2 size-1.5 -translate-x-1/2 translate-y-1/2 rounded-full bg-[#9b7bff] shadow-[0_0_14px_3px_rgba(155,123,255,0.45)]" />
            </div>
            <div className="relative grid size-32 place-items-center rounded-3xl border border-white/10 bg-white/[0.04] backdrop-blur-xl">
              <AuroraMark size={56} />
            </div>
            <span className="absolute -left-6 top-16 rounded-lg border border-white/10 bg-white/[0.05] px-2.5 py-1.5 font-mono text-[9px] text-white/60 backdrop-blur">
              NATIVE · SHADOW
            </span>
            <span className="absolute -right-2 bottom-20 rounded-lg border border-white/10 bg-white/[0.05] px-2.5 py-1.5 font-mono text-[9px] text-white/60 backdrop-blur">
              LIVE · 2s
            </span>
          </div>

          <div className="mt-12 text-center">
            <p className="text-[10px] font-semibold uppercase tracking-[0.3em] text-[#ff8a6b]">Unraid, reimagined</p>
            <h1 className="mt-3 text-[34px] font-semibold leading-[1.1] tracking-[-0.04em] text-white">
              Your data.
              <br />
              <span className="bg-gradient-to-r from-white via-white/80 to-white/40 bg-clip-text text-transparent">
                In perfect focus.
              </span>
            </h1>
            <p className="mx-auto mt-4 max-w-sm text-[12.5px] leading-relaxed text-white/45">
              One calm surface for your array, containers, virtual machines and network — with live telemetry
              and deliberate controls.
            </p>
          </div>
        </div>

        <div className="relative flex items-center justify-between border-t border-white/[0.07] pt-5 text-[10px] text-white/35">
          <span>Independent interface for Unraid</span>
          <span className="flex items-center gap-2">
            <span className="font-mono">Go · React</span>
            <span className="size-1 rounded-full bg-white/20" />
            <span className="font-mono">v0.1.0</span>
          </span>
        </div>
      </section>

      {/* Form panel ----------------------------------------------------- */}
      <section className="relative flex min-h-dvh flex-col px-5 py-6 sm:px-10">
        <div className="pointer-events-none absolute inset-0 overflow-hidden lg:hidden">
          <div className="app-atmosphere" />
        </div>
        <div className="relative flex items-center justify-between">
          <div className="lg:hidden">
            <AuroraWordmark />
          </div>
          <div className="ml-auto flex items-center gap-2">
            <button
              type="button"
              onClick={() => setTheme(resolvedTheme === "dark" ? "light" : "dark")}
              className="grid size-9 place-items-center rounded-lg border border-line bg-[var(--surface-2)] text-muted transition hover:text-ink"
              aria-label="Toggle theme"
            >
              {resolvedTheme === "dark" ? <Sun className="size-4" /> : <Moon className="size-4" />}
            </button>
          </div>
        </div>

        <div className="relative mx-auto flex w-full max-w-[368px] flex-1 flex-col justify-center py-10">
          <div className="mb-8">
            <span className="inline-flex items-center gap-1.5 rounded-full border border-line bg-[var(--surface-2)] px-2.5 py-1 text-[10px] font-medium text-muted">
              <ShieldCheck className="size-3 text-[var(--accent)]" /> Secure session
            </span>
            <h2 className="mt-5 text-[27px] font-semibold tracking-[-0.03em] text-ink">Welcome back</h2>
            <p className="mt-1.5 text-[12.5px] text-muted">Sign in with the root account of this server.</p>
          </div>

          <form onSubmit={(event) => void submit(event)} className="space-y-4">
            <div>
              <label htmlFor="username" className="mb-1.5 block text-[10.5px] font-medium uppercase tracking-[0.1em] text-subtle">
                Username
              </label>
              <div className="flex h-11 items-center gap-2.5 rounded-xl border border-line bg-[var(--surface-2)] px-3 transition focus-within:border-[var(--accent-line)] focus-within:bg-[var(--surface)]">
                <User className="size-4 text-subtle" />
                <input
                  id="username"
                  name="username"
                  autoComplete="username"
                  autoFocus
                  value={username}
                  onChange={(event) => setUsername(event.target.value)}
                  placeholder="root"
                  className="h-full w-full bg-transparent text-[13px] text-ink outline-none placeholder:text-subtle"
                />
              </div>
            </div>

            <div>
              <label htmlFor="password" className="mb-1.5 block text-[10.5px] font-medium uppercase tracking-[0.1em] text-subtle">
                Root password
              </label>
              <div className="flex h-11 items-center gap-2.5 rounded-xl border border-line bg-[var(--surface-2)] px-3 transition focus-within:border-[var(--accent-line)] focus-within:bg-[var(--surface)]">
                <LockKeyhole className="size-4 text-subtle" />
                <input
                  id="password"
                  name="password"
                  type={showPassword ? "text" : "password"}
                  autoComplete="current-password"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  placeholder="••••••••"
                  className="h-full w-full bg-transparent text-[13px] text-ink outline-none placeholder:text-subtle"
                />
                <button
                  type="button"
                  onClick={() => setShowPassword((value) => !value)}
                  aria-label={showPassword ? "Hide password" : "Show password"}
                  className="text-subtle transition hover:text-ink"
                >
                  {showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
                </button>
              </div>
            </div>

            {error ? (
              <div className="animate-fade-up rounded-xl border border-[color-mix(in_srgb,var(--danger)_24%,transparent)] bg-[var(--danger-soft)] px-3 py-2.5 text-[11px] text-[var(--danger)]">
                {error}
              </div>
            ) : null}

            <Button
              type="submit"
              variant="primary"
              size="lg"
              loading={loading}
              className="w-full"
              icon={!loading ? <ArrowRight className="size-4" /> : undefined}
            >
              {loading ? "Signing in…" : "Sign in"}
            </Button>
          </form>

          <div className={cn("mt-8 flex items-center justify-between border-t border-line pt-4 text-[10px] text-subtle")}>
            <button type="button" onClick={() => setTheme(resolvedTheme)} className="hover:text-ink">
              Aurora control plane
            </button>
            <a href="https://unraid.net" target="_blank" rel="noreferrer" className="hover:text-ink">
              unraid.net ↗
            </a>
          </div>
        </div>
      </section>
    </div>
  );
}
