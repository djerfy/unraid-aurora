import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import "./styles/globals.css";
import App from "./App";
import { AppDataProvider } from "./app/AppDataProvider";
import { ThemeProvider } from "./app/ThemeProvider";
import { ToastProvider } from "./app/ToastProvider";
import { I18nProvider } from "./i18n";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
      staleTime: 5_000,
    },
  },
});

const container = document.getElementById("root");
if (!container) throw new Error("Root element is missing");

createRoot(container).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <AppDataProvider>
          <I18nProvider>
            <ToastProvider>
              <BrowserRouter>
                <App />
              </BrowserRouter>
            </ToastProvider>
          </I18nProvider>
        </AppDataProvider>
      </ThemeProvider>
    </QueryClientProvider>
  </StrictMode>,
);
