import { forwardRef, useEffect, useImperativeHandle, useRef } from "react";
import type { VM } from "../../lib/types";

export type ConsoleStatus = "connecting" | "connected" | "disconnected" | "error";

/**
 * noVNC canvas with the libvirt WebSocket proxy behind it. Keyboard events are
 * captured at the document level and forwarded to the canvas, because browser
 * focus can stay on the surrounding dialog even when the console is visible.
 */
export type VMConsoleHandle = {
  sendCtrlAltDel: () => void;
  focus: () => void;
};

export const VMConsolePanel = forwardRef<VMConsoleHandle, {
  vm: VM | null;
  reconnectToken: number;
  onStatus?: (status: ConsoleStatus, message: string) => void;
  className?: string;
}>(function VMConsolePanel({ vm, reconnectToken, onStatus, className }, ref) {
  const hostRef = useRef<HTMLDivElement>(null);
  const clientRef = useRef<{ disconnect: () => void; focus: (options?: FocusOptions) => void; sendCtrlAltDel: () => void } | null>(null);
  const statusRef = useRef(onStatus);
  statusRef.current = onStatus;

  useEffect(() => {
    const host = hostRef.current;
    if (!vm || !host) return;

    let cancelled = false;
    let client: { disconnect: () => void; focus: (options?: FocusOptions) => void } | null = null;
    let detach: (() => void) | null = null;
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
      if (element && (["INPUT", "TEXTAREA", "SELECT"].includes(element.tagName) || element.isContentEditable)) {
        return;
      }
      event.preventDefault();
      // Capture phase: stop the event before any other page handler sees it.
      event.stopPropagation();
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

    report("connecting");

    void (async () => {
      try {
        const { default: RFB } = await import("@novnc/novnc");
        if (cancelled) return;
        host.replaceChildren();
        const protocol = window.location.protocol === "https:" ? "wss" : "ws";
        const url = `${protocol}://${window.location.host}/api/v1/vms/${encodeURIComponent(vm.id)}/console`;
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
          report("disconnected", detail?.clean ? "Session closed." : "The console connection was lost.");
        });
        connection.addEventListener("securityfailure", () => {
          report("error", "The virtual machine rejected the console connection.");
        });
        client = connection;
        clientRef.current = connection;
        keyboard = (connection as unknown as {
          _keyboard?: { _handleKeyDown?: (event: KeyboardEvent) => void; _handleKeyUp?: (event: KeyboardEvent) => void };
        })._keyboard ?? null;

        document.addEventListener("keydown", forwardKey, true);
        document.addEventListener("keyup", forwardKey, true);
        host.addEventListener("pointerdown", focusCanvas);
        detach = () => {
          document.removeEventListener("keydown", forwardKey, true);
          document.removeEventListener("keyup", forwardKey, true);
          host.removeEventListener("pointerdown", focusCanvas);
        };

        window.setTimeout(() => {
          if (!cancelled) connection.focus({ preventScroll: true });
        }, 120);
      } catch (error) {
        report("error", (error as Error).message);
      }
    })();

    return () => {
      cancelled = true;
      detach?.();
      client?.disconnect();
      host.replaceChildren();
    };
  }, [vm, reconnectToken]);

  useImperativeHandle(ref, () => ({
    sendCtrlAltDel: () => clientRef.current?.sendCtrlAltDel(),
    focus: () => hostRef.current?.querySelector("canvas")?.focus({ preventScroll: true }),
  }));

  return (
    <div className={className}>
      <div
        ref={hostRef}
        className="grid size-full min-h-[320px] place-items-center overflow-hidden rounded-xl border border-[#1f2230] bg-[#08090d]"
      />
    </div>
  );
});
