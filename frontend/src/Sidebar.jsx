import { useEffect } from "react";
import {
  Activity,
  X,
  FileText,
  BookOpen,
  BarChart3,
  User,
  Megaphone,
  Wrench,
  Server,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import UserBadge from "./UserBadge.jsx";

// Пункты бокового меню.
export const NAV_ITEMS = [
  { key: "reports", label: "Отчеты", icon: FileText },
  { key: "knowledge", label: "Знания", icon: BookOpen },
  { key: "metrics", label: "Метрики", icon: BarChart3 },
  { key: "profile", label: "Профиль", icon: User },
  { key: "important", label: "Важное", icon: Megaphone },
  { key: "server", label: "Сервер", icon: Server },
  { key: "app", label: "Приложение", icon: Wrench },
];

// Боковое меню: на десктопе закреплено слева (lg+), на мобильных выезжает
// из-за края экрана (бургер). open/onClose управляют только мобильным режимом.
function Sidebar({ view, onSelect, open, onClose, username, isAdmin, onLogout }) {
  // Пока сайдбар открыт на мобильных: Esc закрывает, прокрутка страницы
  // блокируется (чтобы фон не скроллился под затемнением).
  useEffect(() => {
    if (!open) return;
    const onKey = (e) => e.key === "Escape" && onClose();
    document.addEventListener("keydown", onKey);
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = "";
    };
  }, [open, onClose]);

  return (
    <>
      {/* Затемнение фона (только мобильные) */}
      {open && (
        <div
          className="fixed inset-0 z-40 bg-black/50 lg:hidden"
          onClick={onClose}
          aria-hidden="true"
        />
      )}

      <aside
        className={cn(
          "fixed inset-y-0 left-0 z-50 flex w-64 flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground",
          "transition-transform duration-200 ease-in-out lg:translate-x-0",
          open ? "translate-x-0" : "-translate-x-full",
        )}
      >
        {/* Бренд */}
        <div className="flex h-16 shrink-0 items-center gap-3 border-b border-sidebar-border px-4">
          <span className="flex size-9 items-center justify-center rounded-lg bg-primary text-primary-foreground">
            <Activity className="size-5" />
          </span>
          <span className="text-lg font-heading font-semibold tracking-tight">
            avakumov
          </span>
          <Button
            variant="ghost"
            size="icon"
            className="ml-auto lg:hidden"
            onClick={onClose}
            aria-label="Закрыть меню"
          >
            <X className="size-5" />
          </Button>
        </div>

        {/* Пункты меню */}
        <nav className="flex-1 space-y-1 overflow-y-auto p-3">
          {NAV_ITEMS.map(({ key, label, icon: Icon }) => (
            <Button
              key={key}
              variant={view === key ? "default" : "ghost"}
              size="lg"
              className="w-full justify-start gap-3"
              aria-current={view === key ? "page" : undefined}
              onClick={() => {
                onSelect(key);
                onClose();
              }}
            >
              <Icon className="size-4" />
              {label}
            </Button>
          ))}
        </nav>

        {/* Пользователь (на мобильных он в верхней шапке) */}
        <div className="hidden shrink-0 border-t border-sidebar-border p-3 lg:block">
          <UserBadge
            username={username}
            isAdmin={isAdmin}
            onLogout={onLogout}
          />
        </div>
      </aside>
    </>
  );
}

export default Sidebar;
