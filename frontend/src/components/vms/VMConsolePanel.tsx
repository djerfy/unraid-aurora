import { forwardRef, useEffect, useImperativeHandle, useRef } from "react";
import { useI18n } from "../../i18n";
import type { VM } from "../../lib/types";

export type ConsoleStatus = "connecting" | "connected" | "disconnected" | "error";

/**
 * noVNC canvas with the libvirt WebSocket proxy behind it. Keyboard events are
 * captured at the document level and forwarded to the canvas, because browser
 * focus can stay on the surrounding dialog even when the console is visible.
 */
const SPECIAL_KEYS: Record<string, number> = {
  Backspace: 0xff08, Tab: 0xff09, Enter: 0xff0d, Escape: 0xff1b,
  Shift: 0xffe1, Control: 0xffe3, Alt: 0xffe9, Meta: 0xffeb, CapsLock: 0xffe5,
  PageUp: 0xff55, PageDown: 0xff56, End: 0xff57, Home: 0xff50,
  ArrowLeft: 0xff51, ArrowUp: 0xff52, ArrowRight: 0xff53, ArrowDown: 0xff54,
  Insert: 0xff63, Delete: 0xffff, ContextMenu: 0xff67, NumLock: 0xff7f,
  ScrollLock: 0xff14, Pause: 0xff13, PrintScreen: 0xff61,
  F1: 0xffbe, F2: 0xffbf, F3: 0xffc0, F4: 0xffc1, F5: 0xffc2, F6: 0xffc3,
  F7: 0xffc4, F8: 0xffc5, F9: 0xffc6, F10: 0xffc7, F11: 0xffc8, F12: 0xffc9,
};

/** Minimal DOM-key → X11 keysym mapping used when noVNC's mapper is unavailable. */
function keysymFor(event: KeyboardEvent): number | null {
  const code = event.code ?? "";
  if (code.startsWith("Numpad")) {
    if (/^Numpad[0-9]$/.test(code)) return 0xffb0 + Number(code.slice(-1));
    switch (code) {
      case "NumpadAdd": return 0xffab;
      case "NumpadSubtract": return 0xffad;
      case "NumpadMultiply": return 0xffaa;
      case "NumpadDivide": return 0xffaf;
      case "NumpadDecimal": return 0xffae;
      case "NumpadEnter": return 0xff8d;
    }
  }
  const special = SPECIAL_KEYS[event.key];
  if (special !== undefined) {
    if (event.code === "ShiftRight") return 0xffe2;
    if (code === "ControlRight") return 0xffe4;
    if (code === "AltRight") return 0xffea;
    if (code === "MetaRight") return 0xffec;
    return special;
  }
  if (event.key.length === 1) {
    const codePoint = event.key.codePointAt(0);
    if (codePoint === undefined) return null;
    if (codePoint >= 0x20 && codePoint <= 0xff) return codePoint;
    if (codePoint > 0xff) return 0x01000000 + codePoint;
  }
  return null;
}

type RfbInternals = {
  _sock?: { readyState?: string; flush?: () => void };
  constructor?: { messages?: { keyEvent?: (sock: unknown, keysym: number, down: number) => void } };
};

/**
 * Send a raw RFB KeyEvent, bypassing noVNC's state checks and QEMU extended
 * key events. Some QEMU builds accept the connection but silently ignore the
 * extended scancode form used by noVNC.
 */
function sendRawKey(connection: unknown, keysym: number, down: boolean): boolean {
  const rfb = connection as RfbInternals;
  const messages = rfb.constructor?.messages;
  const sock = rfb._sock;
  if (!messages?.keyEvent || !sock) return false;
  if (sock.readyState !== "open") return false;
  messages.keyEvent(sock, keysym, down ? 1 : 0);
  return true;
}

export type VMConsoleHandle = {
  sendCtrlAltDel: () => void;
  focus: () => void;
  clipboardPaste: (text: string) => boolean;
};

export const VMConsolePanel = forwardRef<VMConsoleHandle, {
  vm: VM | null;
  reconnectToken: number;
  onStatus?: (status: ConsoleStatus, message: string) => void;
  className?: string;
}>(function VMConsolePanel({ vm, reconnectToken, onStatus, className }, ref) {
  const { t } = useI18n();
  const hostRef = useRef<HTMLDivElement>(null);
  // Held in a ref so a locale change never re-runs the connection effect.
  const tRef = useRef(t);
  tRef.current = t;
  const clientRef = useRef<{
    disconnect: () => void;
    focus: (options?: FocusOptions) => void;
    sendCtrlAltDel: () => void;
    clipboardPasteFrom?: (text: string) => void;
    sendKey?: (keysym: number, code: string, down: boolean) => void;
  } | null>(null);
  const statusRef = useRef(onStatus);
  statusRef.current = onStatus;
  const vmId = vm?.id ?? "";

  useEffect(() => {
    const host = hostRef.current;
    if (!vmId || !host) return;

    let cancelled = false;
    let client: { disconnect: () => void; focus: (options?: FocusOptions) => void } | null = null;
    // noVNC keeps its layout-aware keyboard mapper on the RFB instance. Calling
    // it directly bypasses canvas focus and DOM event plumbing entirely.
    let keyboard: {
      _handleKeyDown?: (event: KeyboardEvent) => void;
      _handleKeyUp?: (event: KeyboardEvent) => void;
    } | null = null;

    const report = (status: ConsoleStatus, message = "") => {
      if (!cancelled) statusRef.current?.(status, message);
    };
    const canvas = () => host.querySelector("canvas");
    const focusCanvas = () => canvas()?.focus({ preventScroll: true });
    const forwardKey = (event: KeyboardEvent) => {
      const element = event.target as HTMLElement | null;
      // Only keep keys for real inputs that live inside the console UI itself.
      const typingInsideConsole =
        element && host.contains(element) &&
        (["INPUT", "TEXTAREA", "SELECT"].includes(element.tagName) || element.isContentEditable);
      if (typingInsideConsole) {
        return;
      }
      event.preventDefault();
      // Capture phase: stop the event before any other page handler sees it.
      event.stopPropagation();
      const keysym = keysymFor(event);
      if (keysym !== null) {
        const down = event.type === "keydown";
        if (!sendRawKey(clientRef.current, keysym, down)) {
          clientRef.current?.sendKey?.(keysym, event.code ?? "", down);
        }
        return;
      }
      const handler = event.type === "keydown" ? keyboard?._handleKeyDown : keyboard?._handleKeyUp;
      if (handler) {
        handler.call(keyboard, event);
        return;
      }
      // Fallback for noVNC versions that do not expose the mapper: re-dispatch
      // the event straight onto the canvas.
      const target = canvas();
      if (!target) return;
      target.focus({ preventScroll: true });
      target.dispatchEvent(
        new KeyboardEvent(event.type, {
          key: event.key,
          code: event.code,
          location: event.location,
          ctrlKey: event.ctrlKey,
          shiftKey: event.shiftKey,
          altKey: event.altKey,
          metaKey: event.metaKey,
          repeat: event.repeat,
          isComposing: event.isComposing,
          bubbles: true,
          cancelable: true,
        }),
      );
    };

    document.addEventListener("keydown", forwardKey, true);
    document.addEventListener("keyup", forwardKey, true);
    host.addEventListener("pointerdown", focusCanvas);
    const detach = () => {
      document.removeEventListener("keydown", forwardKey, true);
      document.removeEventListener("keyup", forwardKey, true);
      host.removeEventListener("pointerdown", focusCanvas);
    };

    report("connecting");

    void (async () => {
      try {
        const { default: RFB } = await import("@novnc/novnc");
        if (cancelled) return;
        host.replaceChildren();
        const protocol = window.location.protocol === "https:" ? "wss" : "ws";
        const url = `${protocol}://${window.location.host}/api/v1/vms/${encodeURIComponent(vmId)}/console`;
        const connection = new RFB(host, url, { shared: true });
        connection.scaleViewport = true;
        connection.resizeSession = false;
        connection.focusOnClick = true;
        connection.viewOnly = false;
        connection.addEventListener("connect", () => {
          if (cancelled) return;
          report("connected");
          window.setTimeout(() => {
            if (!cancelled) connection.focus({ preventScroll: true });
          }, 50);
        });
        connection.addEventListener("disconnect", (event: Event) => {
          if (cancelled) return;
          const detail = (event as CustomEvent<{ clean?: boolean }>).detail;
          report("disconnected", detail?.clean ? tRef.current("Session closed.") : tRef.current("The console connection was lost."));
        });
        connection.addEventListener("securityfailure", () => {
          report("error", tRef.current("The virtual machine rejected the console connection."));
        });
        client = connection;
        clientRef.current = connection;
        keyboard = (connection as unknown as {
          _keyboard?: { _handleKeyDown?: (event: KeyboardEvent) => void; _handleKeyUp?: (event: KeyboardEvent) => void };
        })._keyboard ?? null;

        window.setTimeout(() => {
          if (!cancelled) connection.focus({ preventScroll: true });
        }, 120);
      } catch (error) {
        report("error", (error as Error).message);
      }
    })();

    return () => {
      cancelled = true;
      detach();
      client?.disconnect();
      clientRef.current = null;
      host.replaceChildren();
    };
  }, [vmId, reconnectToken]);

  useImperativeHandle(ref, () => ({
    sendCtrlAltDel: () => clientRef.current?.sendCtrlAltDel(),
    focus: () => hostRef.current?.querySelector("canvas")?.focus({ preventScroll: true }),
    clipboardPaste: (text: string) => {
      if (!clientRef.current?.clipboardPasteFrom) return false;
      clientRef.current.clipboardPasteFrom(text);
      return true;
    },
  }));

  return (
    <div className={className}>
      <div ref={hostRef} className="grid size-full min-h-[320px] place-items-center overflow-hidden rounded-xl border border-[#1f2230] bg-[#08090d]" />
    </div>
  );
});
