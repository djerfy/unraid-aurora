// Redoc is reference-only: it renders the OpenAPI document but never executes
// requests. The spec is fetched same-origin so the session cookie is sent.
// Redoc 3 ships as an ES module, hence the dynamic import.
import { init } from "/redoc/redoc.standalone.js";

const target = document.getElementById("redoc");
if (target) {
  fetch("/api/openapi.json", {
    credentials: "same-origin",
    headers: { Accept: "application/json" },
  })
    .then((response) => {
      if (!response.ok) {
        throw new Error("HTTP " + response.status);
      }
      return response.json();
    })
    .then((spec) => {
      init(
        spec,
        {
          hideDownloadButtons: false,
          hideTryItPanel: true,
          nativeScrollbars: true,
          theme: {
            logo: {
              url: "/aurora-mark.svg",
              altText: "Aurora",
            },
            typography: {
              fontFamily:
                'Inter, ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif',
              fontSize: "14px",
            },
            colors: {
              primary: { main: "#ff6b4a" },
            },
          },
        },
        target,
      );
    })
    .catch((error) => {
      target.textContent =
        "Unable to load the API specification (" + error.message + "). Sign in and reload this page.";
    });
}
