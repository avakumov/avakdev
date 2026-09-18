import { useState, useEffect } from "react";
import {
  useHealth,
  useMessage,
  useMetrics,
  useMe,
  useImportant,
  useNotificationInbox,
  markImportantSeen,
  logout,
  setOnUnauthorized,
} from "./api.js";
import { useAppStore } from "./store.js";
import { useQueryClient } from "@tanstack/react-query";
import Sidebar from "./Sidebar.jsx";
import Login from "./Login.jsx";
import UserBadge from "./UserBadge.jsx";
import Reports from "./Reports.jsx";
import Goals from "./Goals.jsx";
import Day from "./Day.jsx";
import Profile from "./Profile.jsx";
import Knowledge from "./Knowledge.jsx";
import Reading from "./Reading.jsx";
import Important from "./Important.jsx";
import Metrics from "./Metrics.jsx";
import Notes from "./Notes.jsx";
import NotesDock from "./NotesDock.jsx";
import Tasks from "./Tasks.jsx";
import AppTasks from "./AppTasks.jsx";
import User from "./User.jsx";
import { NotificationsModal } from "./Notifications.jsx";
import DateDisplay from "@/components/DateDisplay.jsx";
import MarkdownView from "./MarkdownView.jsx";
import Brand from "./Brand.jsx";

import {
  Card,
  CardHeader,
  CardTitle,
  CardAction,
  CardContent,
  CardFooter,
  CardDescription,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  RefreshCw,
  Activity,
  Cpu,
  MemoryStick,
  HardDrive,
  Wifi,
  WifiOff,
  Gauge,
  Clock,
  MessageSquare,
  Menu,
  AlertTriangle,
  Check,
  Loader2,
  AlertCircle,
  Bell,
} from "lucide-react";

// Пути в URL для разделов меню: рефреш страницы не сбрасывает раздел,
// работают кнопки назад/вперёд. Главная «/» — раздел «Цели».
const VIEW_PATHS = {
  day: "/",
  goals: "/goals",
  tasks: "/tasks",
  reports: "/reports",
  knowledge: "/knowledge",
  reading: "/reading",
  metrics: "/metrics",
  profile: "/profile",
  important: "/important",
  notes: "/notes",
  app: "/app",
  server: "/server",
  user: "/user",
};

const PATH_VIEWS = Object.fromEntries(
  Object.entries(VIEW_PATHS).map(([view, path]) => [path, view]),
);

// pathToView сопоставляет путь с разделом; неизвестные пути ведут в «День».
function pathToView(path) {
  const p = path.replace(/\/+$/, "") || "/";
  return PATH_VIEWS[p] || "day";
}

// Форматирование байтов в человекочитаемый вид (KB/MB/GB/TB).
function fmtBytes(b) {
  if (b >= 1e12) return (b / 1e12).toFixed(1) + " TB";
  if (b >= 1e9) return (b / 1e9).toFixed(1) + " GB";
  if (b >= 1e6) return (b / 1e6).toFixed(1) + " MB";
  if (b >= 1e3) return (b / 1e3).toFixed(1) + " KB";
  return b + " B";
}

// Форматирование аптайма (секунды -> "Xд Yч Zм").
function fmtUptime(s) {
  const d = Math.floor(s / 86400);
  const h = Math.floor((s % 86400) / 3600);
  const m = Math.floor((s % 3600) / 60);
  if (d > 0) return `${d}д ${h}ч ${m}м`;
  if (h > 0) return `${h}ч ${m}м`;
  return `${m}м`;
}

// Общая панель загрузки (скелетоны) для карточки.
function LoadingSkeleton() {
  return (
    <div className="space-y-3">
      <Skeleton className="h-4 w-3/4" />
      <Skeleton className="h-3 w-full" />
      <Skeleton className="h-3 w-5/6" />
    </div>
  );
}

// Полноэкранный индикатор загрузки (проверка сессии).
function LoadingScreen() {
  return (
    <main className="flex min-h-screen items-center justify-center">
      <div className="flex flex-col items-center gap-3 text-muted-foreground">
        <RefreshCw className="size-6 animate-spin" />
        <p className="text-sm">Проверка доступа…</p>
      </div>
    </main>
  );
}

// Строка метрики с прогресс-баром (shadcn Progress) и абсолютным значением.
function MetricLine({ icon, label, value, suffix = "%", sub, tooltip }) {
  const bar = (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between gap-2">
        <span className="flex items-center gap-2 text-sm font-medium text-foreground">
          {icon}
          {label}
        </span>
        <span className="text-sm font-semibold tabular-nums">
          {value.toFixed(1)}
          {suffix}
        </span>
      </div>
      {sub && (
        <p className="text-xs text-muted-foreground tabular-nums">{sub}</p>
      )}
      <Progress value={value} className="h-2" />
    </div>
  );

  if (!tooltip) return bar;

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <div className="cursor-help">{bar}</div>
      </TooltipTrigger>
      <TooltipContent>{tooltip}</TooltipContent>
    </Tooltip>
  );
}

// Карточка доступности сети.
function NetworkLine({ up, rx, tx }) {
  const Icon = up ? Wifi : WifiOff;
  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between gap-2">
        <span className="flex items-center gap-2 text-sm font-medium text-foreground">
          <Icon className="size-4 text-muted-foreground" />
          Сеть
        </span>
        <Badge variant={up ? "default" : "destructive"}>
          {up ? "● В сети" : "● Нет сети"}
        </Badge>
      </div>
      <div className="flex justify-between text-xs text-muted-foreground tabular-nums">
        <span>⬇ {fmtBytes(rx)}</span>
        <span>⬆ {fmtBytes(tx)}</span>
      </div>
    </div>
  );
}

// Кнопка обновления одной карточки.
function RefreshButton({ onClick, refreshing }) {
  return (
    <Button variant="outline" size="sm" onClick={onClick} disabled={refreshing}>
      <RefreshCw className={refreshing ? "animate-spin" : ""} />
      {refreshing ? "Обновляю…" : "Обновить"}
    </Button>
  );
}

// Модальное окно «Важное сообщение»: блокирует основной функционал, пока
// пользователь не подтвердит прочтение. Показывается только на production
// и не чаще раза в сутки (проверка на сервере).
function ImportantGate({ content, onDone }) {
  const [marking, setMarking] = useState(false);
  const [error, setError] = useState("");

  const handleRead = async () => {
    setMarking(true);
    setError("");
    try {
      await markImportantSeen();
      onDone();
    } catch (err) {
      setError(err.message || "Не удалось подтвердить прочтение");
      setMarking(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto bg-black/60 p-4">
      <div className="flex min-h-full items-center justify-center">
        <Card className="w-full max-w-3xl">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <span className="flex size-9 items-center justify-center rounded-lg bg-primary text-primary-foreground">
              <AlertTriangle className="size-5" />
            </span>
            Важное сообщение
          </CardTitle>
          <CardDescription>
            Пожалуйста, прочитайте сообщение перед продолжением работы.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Сообщение показывается целиком, без внутреннего скролла */}
          <MarkdownView>{content}</MarkdownView>
          {error && (
            <p
              className="flex items-center gap-1.5 text-sm text-destructive"
              role="alert"
            >
              <AlertCircle className="size-4" />
              {error}
            </p>
          )}
          <Button onClick={handleRead} disabled={marking} className="w-full">
            {marking ? <Loader2 className="animate-spin" /> : <Check />}
            {marking ? "Подтверждаю…" : "Прочитал(а), продолжить"}
          </Button>
        </CardContent>
      </Card>
      </div>
    </div>
  );
}

function App() {
  // Авторизация: /api/me показывает, вошёл ли пользователь.
  const meQuery = useMe();
  const isAuthed = Boolean(meQuery.data);

  // TanStack Query — данные с сервера (запрашиваем только после входа).
  const importantQuery = useImportant(isAuthed);
  const notificationsQuery = useNotificationInbox(isAuthed);

  // Zustand — глобальное UI-состояние
  const queryClient = useQueryClient();

  // Любая 401 от API (кроме /api/login) = сессия истекла: помечаем
  // пользователя неавторизованным — App сам переключится на экран входа.
  useEffect(() => {
    setOnUnauthorized(() => {
      queryClient.setQueryData(["me"], null);
    });
  }, [queryClient]);
  const lastUpdatedAt = useAppStore((s) => s.lastUpdatedAt);
  const setLastUpdatedAt = useAppStore((s) => s.setLastUpdatedAt);

  // Текущий раздел меню, синхронизированный с URL: рефреш не сбрасывает,
  // кнопки назад/вперёд работают.
  const [viewState, setViewState] = useState(() =>
    pathToView(window.location.pathname),
  );

  // Разделы, доступные пользователю, приходят с сервера в /api/me (sections).
  const allowedSections = new Set(meQuery.data?.sections || []);
  // Недоступный раздел (например, «Сервер» для обычного пользователя) мягко
  // сводим к главной — сами данные всё равно защищены на сервере.
  const view = allowedSections.has(viewState) ? viewState : "day";

  // Данные серверной панели (health/message/metrics) запрашиваем только когда
  // открыт раздел «Сервер»: иначе /api/metrics опрашивался бы каждые 5 секунд
  // на любой странице (а у обычных пользователей это ещё и 403 — эти маршруты
  // только для админов).
  const serverOpen = isAuthed && view === "server";
  const healthQuery = useHealth(serverOpen);
  const messageQuery = useMessage(serverOpen);
  const metricsQuery = useMetrics(5000, serverOpen);

  // Если раздел в URL оказался недоступен — приводим URL обратно к главной.
  useEffect(() => {
    if (view !== viewState) {
      window.history.replaceState({}, "", VIEW_PATHS.day);
    }
  }, [view, viewState]);

  const setView = (v) => {
    setViewState(v);
    const path = VIEW_PATHS[v] || "/";
    if (window.location.pathname !== path) {
      window.history.pushState({}, "", path);
    }
  };

  // Реакция на кнопки назад/вперёд браузера.
  useEffect(() => {
    const onPop = () => setViewState(pathToView(window.location.pathname));
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);

  // Открыто ли боковое меню на мобильных (бургер).
  const [sidebarOpen, setSidebarOpen] = useState(false);
  // Модалка уведомлений (колокольчик).
  const [bellOpen, setBellOpen] = useState(false);

  // Пока проверяем сессию или «важное» сообщение — показываем спиннер.
  // (Сообщение решает, показывать ли модалку-гейт перед основным функционалом.)
  if (meQuery.isLoading || (isAuthed && importantQuery.isLoading)) {
    return <LoadingScreen />;
  }

  // Если не авторизован — только экран входа.
  if (!isAuthed) {
    return (
      <Login
        onSuccess={() => {
          // После логина сбрасываем закэшированные ошибки 401,
          // чтобы данные пере-запросились с валидной сессией.
          queryClient.removeQueries({ queryKey: ["health"] });
          queryClient.removeQueries({ queryKey: ["message"] });
          queryClient.removeQueries({ queryKey: ["metrics"] });
          queryClient.removeQueries({ queryKey: ["important"] });
          queryClient.removeQueries({ queryKey: ["user-metrics"] });
          queryClient.removeQueries({ queryKey: ["app-tasks"] });
          queryClient.removeQueries({ queryKey: ["tasks"] });
          queryClient.removeQueries({ queryKey: ["notifications"] });
          queryClient.removeQueries({ queryKey: ["notifications-inbox"] });
          queryClient.invalidateQueries({ queryKey: ["me"] });
        }}
      />
    );
  }

  // «Важное» сообщение: на production, если текст есть и сегодня ещё не
  // показывали — блокируем основной функционал до подтверждения прочтения.
  const important = importantQuery.data;
  const mustReadImportant = Boolean(
    important?.enabled &&
      important?.content &&
      important.content.trim() &&
      !important.seen_today,
  );

  if (mustReadImportant) {
    return (
      <ImportantGate
        content={important.content}
        onDone={() => queryClient.invalidateQueries({ queryKey: ["important"] })}
      />
    );
  }

  const handleLogout = async () => {
    await logout();
    // Сбрасываем данные после выхода.
    queryClient.removeQueries({ queryKey: ["health"] });
    queryClient.removeQueries({ queryKey: ["message"] });
    queryClient.removeQueries({ queryKey: ["metrics"] });
    queryClient.removeQueries({ queryKey: ["important"] });
    queryClient.removeQueries({ queryKey: ["user-metrics"] });
    queryClient.removeQueries({ queryKey: ["app-tasks"] });
    queryClient.removeQueries({ queryKey: ["tasks"] });
    queryClient.removeQueries({ queryKey: ["notifications"] });
    queryClient.removeQueries({ queryKey: ["notifications-inbox"] });
    await queryClient.invalidateQueries({ queryKey: ["me"] });
  };

  const refreshAll = async () => {
    await Promise.all([
      queryClient.refetchQueries(["health"]),
      queryClient.refetchQueries(["message"]),
      queryClient.refetchQueries(["metrics"]),
    ]);
    setLastUpdatedAt(new Date().toLocaleTimeString());
  };

  const isRefreshing =
    healthQuery.isFetching ||
    messageQuery.isFetching ||
    metricsQuery.isFetching;

  const m = metricsQuery.data;

  return (
    <div className="min-h-screen">
      {/* Мобильная шапка с бургером (на десктопе скрыта — меню в сайдбаре) */}
      <header className="sticky top-0 z-40 flex items-center gap-3 border-b bg-background/95 px-4 py-3 backdrop-blur lg:hidden">
        <Button
          variant="outline"
          size="icon"
          onClick={() => setSidebarOpen(true)}
          aria-label="Открыть меню"
          aria-expanded={sidebarOpen}
        >
          <Menu className="size-5" />
        </Button>
        <Brand onClick={() => setView("day")} />
        {meQuery.data && (
          <div className="ml-auto flex items-center gap-1">
            <Button
              variant="ghost"
              size="icon"
              onClick={() => setBellOpen(true)}
              aria-label="Уведомления"
              className="relative"
            >
              <Bell className="size-5" />
              {notificationsQuery.data?.inbox?.length > 0 && (
                <span className="absolute top-1 right-1 flex h-4 min-w-4 items-center justify-center rounded-full bg-primary px-1 text-[10px] font-semibold text-primary-foreground">
                  {notificationsQuery.data.inbox.length > 9
                    ? "9+"
                    : notificationsQuery.data.inbox.length}
                </span>
              )}
            </Button>
            <UserBadge user={meQuery.data} onClick={() => setView("user")} />
          </div>
        )}
      </header>

      {/* Боковое меню: закреплено слева на десктопе, выезжает из-за края
          на мобильных (бургер) */}
      <Sidebar
        view={view}
        onSelect={setView}
        open={sidebarOpen}
        onClose={() => setSidebarOpen(false)}
        user={meQuery.data}
        notifCount={notificationsQuery.data?.inbox?.length || 0}
        onOpenBell={() => setBellOpen(true)}
      />

      <main className="lg:pl-64">
        <div
          className={
            "mx-auto px-4 py-8 " +
            (view === "metrics"
              ? // Таблицу метрик не ограничиваем шириной текстовой колонки:
                // колонки дат остаются минимальными, а если не влезают —
                // появляется горизонтальная прокрутка.
                "max-w-none"
              : "max-w-3xl lg:max-w-5xl")
          }
        >
          {view === "day" && <Day onNavigate={setView} />}
          {view === "goals" && <Goals />}
          {view === "tasks" && <Tasks />}
          {view === "reports" && <Reports />}
          {view === "knowledge" && <Knowledge />}
          {view === "reading" && <Reading />}
          {view === "metrics" && <Metrics />}
          {view === "profile" && <Profile />}
          {view === "important" && <Important />}
          {view === "notes" && <Notes />}
          {view === "app" && <AppTasks />}
          {view === "user" && (
            <User user={meQuery.data} onLogout={handleLogout} />
          )}
          {view === "server" && (
            <>
              {/* Панель быстрых действий */}
              <div className="mb-6 flex flex-wrap items-center gap-3">
                <Button onClick={refreshAll} disabled={isRefreshing}>
                  <RefreshCw className={isRefreshing ? "animate-spin" : ""} />
                  {isRefreshing ? "Обновляю всё…" : "Обновить всё"}
                </Button>
                {lastUpdatedAt && (
                  <span className="flex items-center gap-1.5 text-sm text-muted-foreground">
                    <Clock className="size-4" />
                    Последнее обновление: {lastUpdatedAt}
                  </span>
                )}
              </div>

              {/* Блок системных метрик */}
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <Gauge className="size-4 text-muted-foreground" />
                    Метрики сервера
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  {metricsQuery.isLoading ? (
                    <LoadingSkeleton />
                  ) : metricsQuery.isError ? (
                    <p className="text-sm text-destructive">
                      Ошибка: {metricsQuery.error?.message}
                    </p>
                  ) : m ? (
                    <div className="grid gap-5 sm:grid-cols-2">
                      <MetricLine
                        icon={<Cpu className="size-4 text-muted-foreground" />}
                        label="CPU"
                        value={m.cpu}
                        sub={`${m.cpu_used_cores.toFixed(2)} / ${m.cpu_cores} ядер`}
                        tooltip="Загрузка процессора"
                      />
                      <MetricLine
                        icon={
                          <MemoryStick className="size-4 text-muted-foreground" />
                        }
                        label="Память"
                        value={m.memory}
                        sub={`${fmtBytes(m.memory_used_bytes)} / ${fmtBytes(m.memory_total_bytes)}`}
                        tooltip="Использование оперативной памяти"
                      />
                      <MetricLine
                        icon={
                          <HardDrive className="size-4 text-muted-foreground" />
                        }
                        label="Диск"
                        value={m.disk}
                        sub={`${fmtBytes(m.disk_used_bytes)} / ${fmtBytes(m.disk_total_bytes)}`}
                        tooltip="Занято места на корневом разделе"
                      />
                      <div className="flex flex-col justify-center space-y-3">
                        <NetworkLine
                          up={m.network_up}
                          rx={m.network.rx_bytes}
                          tx={m.network.tx_bytes}
                        />
                      </div>
                    </div>
                  ) : null}
                </CardContent>
                {m && (
                  <CardFooter className="justify-between">
                    <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
                      <Clock className="size-3.5" />
                      Аптайм: {fmtUptime(m.uptime_seconds)}
                    </span>
                    <span className="text-xs text-muted-foreground tabular-nums">
                      обновлено: <DateDisplay date={m.timestamp} withTime />
                    </span>
                  </CardFooter>
                )}
              </Card>

              {/* Карточка Health */}
              <Card className="mt-6">
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <Activity className="size-4 text-muted-foreground" />
                    Health
                  </CardTitle>
                  <CardAction>
                    <RefreshButton
                      onClick={() => queryClient.refetchQueries(["health"])}
                      refreshing={healthQuery.isFetching}
                    />
                  </CardAction>
                </CardHeader>
                <CardContent>
                  {healthQuery.isLoading ? (
                    <LoadingSkeleton />
                  ) : healthQuery.isError ? (
                    <p className="text-sm text-destructive">
                      Ошибка: {healthQuery.error?.message}
                    </p>
                  ) : (
                    <pre className="overflow-x-auto text-sm">
                      {JSON.stringify(healthQuery.data, null, 2)}
                    </pre>
                  )}
                </CardContent>
              </Card>

              {/* Карточка Message */}
              <Card className="mt-6">
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <MessageSquare className="size-4 text-muted-foreground" />
                    Message
                  </CardTitle>
                  <CardAction>
                    <RefreshButton
                      onClick={() => queryClient.refetchQueries(["message"])}
                      refreshing={messageQuery.isFetching}
                    />
                  </CardAction>
                </CardHeader>
                <CardContent>
                  {messageQuery.isLoading ? (
                    <LoadingSkeleton />
                  ) : messageQuery.isError ? (
                    <p className="text-sm text-destructive">
                      Ошибка: {messageQuery.error?.message}
                    </p>
                  ) : (
                    <pre className="overflow-x-auto text-sm">
                      {JSON.stringify(messageQuery.data, null, 2)}
                    </pre>
                  )}
                </CardContent>
              </Card>
            </>
          )}

          <footer className="mt-10 text-center text-xs text-muted-foreground">
            Собрано с shadcn/ui · Tailwind CSS v4 · Zustand · TanStack Query
          </footer>
        </div>
      </main>

      {isAuthed && bellOpen && (
        <NotificationsModal onClose={() => setBellOpen(false)} />
      )}

      {/* Быстрая заметка: кнопка «з» слева внизу — на всех страницах,
          в том числе поверх открытой книги. */}
      {isAuthed && <NotesDock />}
    </div>
  );
}

export default App;
