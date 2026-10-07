import { useEffect, useState } from "react";
import { Toaster } from "sonner";
import { useAppStore } from "../store.js";

// Всплывающие уведомления (Sonner). На десктопе — справа снизу, на мобильных —
// сверху (снизу мешают системная панель и жесты). Порог тот же, что у вёрстки
// (Tailwind lg = 1024px).
export default function AppToaster() {
  const theme = useAppStore((s) => s.theme);
  const [isMobile, setIsMobile] = useState(
    () => typeof window !== "undefined" && window.innerWidth < 1024,
  );

  useEffect(() => {
    const mq = window.matchMedia("(max-width: 1023px)");
    const onChange = () => setIsMobile(mq.matches);
    onChange();
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, []);

  return (
    <Toaster
      theme={theme}
      position={isMobile ? "top-center" : "bottom-right"}
      richColors
    />
  );
}
