import { FlaskConical } from "lucide-react";
import { IS_LOCAL_DEV } from "@/lib/env";

// Узкая жёлтая полоса в самом верху экрана: сразу видно, что запущена
// локальная dev-версия приложения, а не production. На avakumov.ru не
// отображается. Высота фиксированная (h-7 = 28px) — её же учитывают отступы
// в App/Sidebar (см. IS_LOCAL_DEV там).
export default function DevBanner() {
  if (!IS_LOCAL_DEV) return null;
  return (
    <div className="fixed inset-x-0 top-0 z-[60] flex h-7 items-center justify-center gap-1.5 border-b border-amber-400 bg-amber-300 px-3 text-xs font-medium text-amber-950">
      <FlaskConical className="size-3.5 shrink-0" />
      <span className="truncate">Запущена локальная dev-версия приложения</span>
    </div>
  );
}
