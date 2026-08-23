import { useState } from "react";
import {
  useHealth,
  useMessage,
  useMetrics,
  useMe,
  useImportant,
  markImportantSeen,
  logout,
} from "./api.js";
import { useAppStore } from "./store.js";
import { useQueryClient } from "@tanstack/react-query";
import Login from "./Login.jsx";
import UserBadge from "./UserBadge.jsx";
import Reports from "./Reports.jsx";
import Profile from "./Profile.jsx";
import Knowledge from "./Knowledge.jsx";
import Important from "./Important.jsx";
import Metrics from "./Metrics.jsx";
import MarkdownView from "./MarkdownView.jsx";

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
import { Separator } from "@/components/ui/separator";
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
  FileText,
  Server,
  User,
  BookOpen,
  Megaphone,
  BarChart3,
  AlertTriangle,
  Check,
  Loader2,
  AlertCircle,
} from "lucide-react";

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
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
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
          <MarkdownView className="max-h-80 overflow-y-auto">
            {content}
          </MarkdownView>
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
  );
}

function App() {
  // Авторизация: /api/me показывает, вошёл ли пользователь.
  const meQuery = useMe();
  const isAuthed = Boolean(meQuery.data);

  // TanStack Query — данные с сервера (запрашиваем только после входа).
  const healthQuery = useHealth(isAuthed);
  const messageQuery = useMessage(isAuthed);
  const metricsQuery = useMetrics(5000, isAuthed);
  const importantQuery = useImportant(isAuthed);

  // Zustand — глобальное UI-состояние
  const queryClient = useQueryClient();
  const lastUpdatedAt = useAppStore((s) => s.lastUpdatedAt);
  const setLastUpdatedAt = useAppStore((s) => s.setLastUpdatedAt);

  // Текущий раздел меню: "reports" / "knowledge" / "metrics" / "profile" /
  // "important" / "server".
  const [view, setView] = useState("reports");

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
    <main className="mx-auto max-w-3xl px-6 py-12">
      {/* Шапка с пользователем */}
      <header className="mb-8 flex items-start justify-between gap-4">
        <div className="space-y-1">
          <h1 className="flex items-center gap-3 text-2xl font-heading font-semibold tracking-tight">
            <span className="flex size-9 items-center justify-center rounded-lg bg-primary text-primary-foreground">
              <Activity className="size-5" />
            </span>
            Go (Gin) + React + TanStack Query
          </h1>
        </div>
        {meQuery.data && (
          <UserBadge
            username={meQuery.data.username}
            isAdmin={meQuery.data.is_admin}
            onLogout={handleLogout}
          />
        )}
      </header>

      {/* Меню навигации: отчёты / знания / профиль / сервер */}
      <nav className="mb-6 flex flex-wrap items-center gap-2">
        <Button
          variant={view === "reports" ? "default" : "outline"}
          size="lg"
          onClick={() => setView("reports")}
        >
          <FileText className="size-4" />
          Отчеты
        </Button>
        <Button
          variant={view === "knowledge" ? "default" : "outline"}
          size="lg"
          onClick={() => setView("knowledge")}
        >
          <BookOpen className="size-4" />
          Знания
        </Button>
        <Button
          variant={view === "metrics" ? "default" : "outline"}
          size="lg"
          onClick={() => setView("metrics")}
        >
          <BarChart3 className="size-4" />
          Метрики
        </Button>
        <Button
          variant={view === "profile" ? "default" : "outline"}
          size="lg"
          onClick={() => setView("profile")}
        >
          <User className="size-4" />
          Профиль
        </Button>
        <Button
          variant={view === "important" ? "default" : "outline"}
          size="lg"
          onClick={() => setView("important")}
        >
          <Megaphone className="size-4" />
          Важное
        </Button>
        <Button
          variant={view === "server" ? "default" : "outline"}
          size="lg"
          onClick={() => setView("server")}
        >
          <Server className="size-4" />
          Сервер
        </Button>
      </nav>

      <Separator className="mb-8" />

      {view === "reports" && <Reports />}
      {view === "knowledge" && <Knowledge />}
      {view === "metrics" && <Metrics />}
      {view === "profile" && <Profile />}
      {view === "important" && <Important />}
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
                  обновлено: {m.timestamp}
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
    </main>
  );
}

export default App;
