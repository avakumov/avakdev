import { useEffect } from "react";
import {
  X,
  CalendarDays,
  Target,
  ListTodo,
  FileText,
  BookOpen,
  BookOpenText,
  BarChart3,
  User,
  Megaphone,
  Wrench,
  Server,
  Bell,
  StickyNote,
  Rss,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import UserBadge from "./UserBadge.jsx";
import Brand from "./Brand.jsx";

// Пункты бокового меню. «Цели» — первым пунктом (главная страница).
// Доступные разделы приходят с сервера (/api/me → sections): список ниже
// фильтруется по ним, чтобы видимость меню не дублировалась на клиенте.
// feed-edit — редактирование ленты; сама лента для просмотра открывается
// свайпом и в меню не входит.
export const NAV_ITEMS = [
  { key: "day", label: "День", icon: CalendarDays },
  { key: "goals", label: "Цели", icon: Target },
  { key: "tasks", label: "Задачи", icon: ListTodo },
  { key: "reports", label: "Отчеты", icon: FileText },
  { key: "knowledge", label: "Знания", icon: BookOpen },
  { key: "reading", label: "Чтение", icon: BookOpenText },
  { key: "metrics", label: "Метрики", icon: BarChart3 },
  { key: "profile", label: "Резюме", icon: User },
  { key: "important", label: "Важное", icon: Megaphone },
  { key: "notes", label: "Заметки", icon: StickyNote },
  { key: "feed-edit", label: "Лента", icon: Rss },
  { key: "server", label: "Сервер", icon: Server },
  { key: "app", label: "Приложение", icon: Wrench },
];

// Боковое меню: на десктопе закреплено слева (lg+), на мобильных выезжает
// из-за края экрана (бургер). open/onClose управляют только мобильным режимом.
function Sidebar({ view, onSelect, open, onClose, user, notifCount = 0, onOpenBell }) {
  // Показываем только разделы, разрешённые сервером для этого пользователя.
  const allowedSections = new Set(user?.sections || []);
  const items = NAV_ITEMS.filter(({ key }) => allowedSections.has(key));
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
        {/* Бренд: клик ведёт на главную */}
        <div className="flex h-16 shrink-0 items-center gap-3 border-b border-sidebar-border px-4">
          <Brand
            onClick={() => {
              onSelect("day");
              onClose();
            }}
          />
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
          {items.map(({ key, label, icon: Icon }) => (
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

        {/* Низ: колокольчик уведомлений + пользователь */}
        <div className="hidden shrink-0 space-y-1 border-t border-sidebar-border p-2 lg:block">
          <Button
            variant="ghost"
            size="lg"
            className="w-full justify-start gap-3"
            onClick={onOpenBell}
          >
            <span className="relative">
              <Bell className="size-4" />
              {notifCount > 0 && (
                <span className="absolute -top-1.5 -right-1.5 flex h-3.5 min-w-3.5 items-center justify-center rounded-full bg-primary px-1 text-[9px] font-semibold text-primary-foreground">
                  {notifCount > 9 ? "9+" : notifCount}
                </span>
              )}
            </span>
            Уведомления
          </Button>
          <UserBadge
            user={user}
            onClick={() => {
              onSelect("user");
              onClose();
            }}
          />
        </div>
      </aside>
    </>
  );
}

export default Sidebar;
