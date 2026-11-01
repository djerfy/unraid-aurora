declare module "@novnc/novnc" {
  export type RFBOptions = {
    credentials?: { username?: string; password?: string; target?: string };
    shared?: boolean;
    wsProtocols?: string[];
  };

  export default class RFB extends EventTarget {
    constructor(target: HTMLElement, url: string, options?: RFBOptions);
    scaleViewport: boolean;
    clipViewport: boolean;
    resizeSession: boolean;
    viewOnly: boolean;
    focus(options?: FocusOptions): void;
    blur(): void;
    focusOnClick: boolean;
    disconnect(): void;
    sendCredentials(credentials: Record<string, string>): void;
    sendCtrlAltDel(): void;
  }
}
