import { useEffect, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { Terminal as XTerm } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { ExternalLink, RefreshCw, ShieldCheck, SquareTerminal } from "lucide-react";
import { Badge, Button, cn } from "../components/ui/primitives";
import { useApp } from "../app/AppDataProvider";
import { useToast } from "../app/ToastProvider";
import { useI18n } from "../i18n";

type Connection = "connecting" | "connected" | "closed" | "error";

export function TerminalPage() {
  const { snapshot, session } = useApp();
  const { toast } = useToast();
  const { t } = useI18n();
  const [params] = useSearchParams();
  const popout = params.get("popout") === "1";
  const hostRef = useRef<HTMLDivElement>(null);
  const [connection, setConnection] = useState<Connection>("connecting");
  const [generation, setGeneration] = useState(0);

  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;

    const term = new XTerm({
      cursorBlink: true,
      cursorStyle: "bar",
      fontSize: 13,
      fontFamily: '"JetBrains Mono Variable", "SFMono-Regular", monospace',
      lineHeight: 1.35,
      scrollback: 5000,
      allowTransparency: true,
      theme: {
        background: "#08090d",
        foreground: "#c7cbd6",
        cursor: "#ff6b4a",
        cursorAccent: "#08090d",
        selectionBackground: "rgba(255,107,74,0.28)",
        black: "#08090d",
        red: "#ff6b81",
        green: "#3ddc97",
        yellow: "#f5b44a",
        blue: "#5aa9ff",
        magenta: "#9b7bff",
        cyan: "#4de3c0",
        white: "#c7cbd6",
        brightBlack: "#5f6678",
        brightRed: "#ff8a9b",
        brightGreen: "#70e8b4",
        brightYellow: "#ffd27a",
        brightBlue: "#8dc3ff",
        brightMagenta: "#c0a8ff",
        brightCyan: "#8df0d8",
        brightWhite: "#f3f4f7",
      },
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(host);
    const resize = () => {
      try {
        fit.fit();
      } catch {
        // The terminal may be measured during the page transition.
      }
    };
    resize();

    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const socket = new WebSocket(`${protocol}//${window.location.host}/api/v1/terminal`);
    socket.binaryType = "arraybuffer";
    setConnection("connecting");

    socket.onopen = () => {
      setConnection("connected");
      term.reset();
      term.writeln("\x1b[38;5;209mAurora terminal\x1b[0m  \x1b[90mroot shell · encrypted transport\x1b[0m");
      term.writeln("");
      resize();
      term.focus();
    };
    socket.onmessage = (event) => {
      if (event.data instanceof ArrayBuffer) {
        term.write(new Uint8Array(event.data));
      } else if (event.data instanceof Blob) {
        void event.data.arrayBuffer().then((buffer) => term.write(new Uint8Array(buffer)));
      } else {
        term.write(String(event.data));
      }
    };
    socket.onerror = () => {
      setConnection("error");
    };
    socket.onclose = () => {
      setConnection("closed");
      term.writeln("");
      term.writeln("\x1b[33m[session closed]\x1b[0m");
    };

    const dataDisposable = term.onData((data) => {
      if (socket.readyState === WebSocket.OPEN) {
        socket.send(new TextEncoder().encode(data));
      }
    });
    const resizeDisposable = term.onResize(({ cols, rows }) => {
      if (socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({ type: "resize", cols, rows }));
      }
    });
    const observer = new ResizeObserver(() => resize());
    observer.observe(host);
    const focus = window.setTimeout(() => term.focus(), 120);

    return () => {
      window.clearTimeout(focus);
      observer.disconnect();
      dataDisposable.dispose();
      resizeDisposable.dispose();
      socket.close();
      term.dispose();
    };
  }, [generation]);

  const openWindow = () => {
    const popup = window.open(
      `${window.location.origin}/terminal?popout=1`,
      "aurora-terminal",
      "popup=yes,width=1200,height=800",
    );
    popup?.focus();
  };

  return (
    <div className={cn("flex min-h-0 w-full flex-col bg-[#08090d]", popout ? "h-dvh" : "flex-1")}>
      <div className="flex flex-wrap items-center gap-2 border-b border-[#1f2230] bg-[#08090d] px-4 py-2.5">
        <SquareTerminal className="size-4 text-[#ff8a6b]" />
        <span className="font-mono text-[11px] text-[#c7cbd6]">
          {session?.user ?? "root"}@{snapshot?.system.hostname ?? "server"}:~
        </span>
        <Badge
          tone={connection === "connected" ? "success" : connection === "connecting" ? "warning" : "danger"}
          dot
          pulse={connection === "connecting"}
        >
          {t(connection)}
        </Badge>
        <div className="ml-auto flex items-center gap-2">
          {!popout ? (
            <Button
              size="sm"
              variant="secondary"
              icon={<ExternalLink className="size-3" />}
              onClick={openWindow}
              className="border-[#2a2e3b] bg-[#11141c] text-[#c7cbd6] hover:border-[#3a4050] hover:bg-[#181c26]"
            >
              {t("New window")}
            </Button>
          ) : null}
          <Button
            size="sm"
            variant="secondary"
            icon={<RefreshCw className="size-3" />}
            onClick={() => {
              setGeneration((value) => value + 1);
              toast({ kind: "info", title: t("Reconnecting terminal") });
            }}
            className="border-[#2a2e3b] bg-[#11141c] text-[#c7cbd6] hover:border-[#3a4050] hover:bg-[#181c26]"
          >
            {t("Reconnect")}
          </Button>
        </div>
      </div>
      <div ref={hostRef} className="min-h-0 w-full flex-1 px-2 py-2" />
      <div className="flex items-center gap-2 border-t border-[#1f2230] bg-[#08090d] px-4 py-2 font-mono text-[9px] text-[#5f6678]">
        <ShieldCheck className="size-3 text-[#3ddc97]" />
        {t("Commands execute as root on the Unraid host. Keep this session private.")}
      </div>
    </div>
  );
}
