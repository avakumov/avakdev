import React from "react";
import ReactDOM from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import App from "./App.jsx";
import { TooltipProvider } from "@/components/ui/tooltip";
import { initTheme } from "./store.js";
import DevBanner from "./components/DevBanner.jsx";
import AppToaster from "./components/AppToaster.jsx";
import "./index.css";
import "./highlight.css";

// Применяем сохранённую тему до первого рендера (без «вспышки»).
initTheme();

// Клиент TanStack Query: кэширует и инвалидирует данные серверных запросов.
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      refetchOnWindowFocus: false,
    },
  },
});

ReactDOM.createRoot(document.getElementById("root")).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <DevBanner />
        <App />
        <AppToaster />
      </TooltipProvider>
    </QueryClientProvider>
  </React.StrictMode>,
);
