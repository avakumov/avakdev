import { useHealth, useMessage, useMetrics } from "./api.js";
import { useAppStore } from "./store.js";
import { useQueryClient } from "@tanstack/react-query";

function Card({ title, children, onRefresh, refreshing }) {
  return (
    <section className="card">
      <div className="card__header">
        <h2>{title}</h2>
        {onRefresh && (
          <button
            className="card__refresh"
            onClick={onRefresh}
            disabled={refreshing}
          >
            {refreshing ? "Обновляю…" : "Обновить"}
          </button>
        )}
      </div>
      {children}
    </section>
  );
}

// Форматирование байтов в человекочитаемый вид (KB/MB/GB/TB).
function fmtBytes(b) {
  if (b >= 1e12) return (b / 1e12).toFixed(1) + " TB";
  if (b >= 1e9) return (b / 1e9).toFixed(1) + " GB";
  if (b >= 1e6) return (b / 1e6).toFixed(1) + " MB";
  if (b >= 1e3) return (b / 1e3).toFixed(1) + " KB";
  return b + " B";
}

// Карточка метрики с цветным прогресс-баром.
// Помимо процентов показывает абсолютное значение (например, "3.2 / 8.0 GB").
function MetricCard({ label, value, suffix = "%", sub }) {
  // Цвет в зависимости от нагрузки: зелёный, жёлтый, красный.
  let color = "#22c55e";
  if (value >= 85) color = "#ef4444";
  else if (value >= 60) color = "#f59e0b";

  return (
    <div className="metric">
      <div className="metric__top">
        <span className="metric__label">{label}</span>
        <span className="metric__value">
          {value.toFixed(1)}
          {suffix}
        </span>
      </div>
      {sub && <div className="metric__abs">{sub}</div>}
      <div className="metric__bar">
        <div
          className="metric__fill"
          style={{
            width: `${Math.min(100, Math.max(0, value))}%`,
            background: color,
          }}
        />
      </div>
    </div>
  );
}

// Карточка доступности сети.
function NetworkCard({ up, rx, tx }) {
  const fmtUptime = (s) => {
    const d = Math.floor(s / 86400);
    const h = Math.floor((s % 86400) / 3600);
    const m = Math.floor((s % 3600) / 60);
    if (d > 0) return `${d}д ${h}ч ${m}м`;
    if (h > 0) return `${h}ч ${m}м`;
    return `${m}м`;
  };

  return (
    <div className="metric metric--network">
      <div className="metric__top">
        <span className="metric__label">Сеть</span>
        <span className={`metric__status ${up ? "is-up" : "is-down"}`}>
          {up ? "● В сети" : "● Нет сети"}
        </span>
      </div>
      <div className="metric__net">
        <span>⬇ {fmtBytes(rx)}</span>
        <span>⬆ {fmtBytes(tx)}</span>
      </div>
    </div>
  );
}

function App() {
  // TanStack Query — данные с сервера
  const healthQuery = useHealth();
  const messageQuery = useMessage();
  const metricsQuery = useMetrics();

  // Zustand — глобальное UI-состояние
  const queryClient = useQueryClient();
  const lastUpdatedAt = useAppStore((s) => s.lastUpdatedAt);
  const setLastUpdatedAt = useAppStore((s) => s.setLastUpdatedAt);
  const incrementLoaded = useAppStore((s) => s.incrementLoaded);

  const refreshAll = async () => {
    await Promise.all([
      queryClient.refetchQueries(["health"]),
      queryClient.refetchQueries(["message"]),
      queryClient.refetchQueries(["metrics"]),
    ]);
    setLastUpdatedAt(new Date().toLocaleTimeString());
    incrementLoaded();
  };

  const isRefreshing =
    healthQuery.isFetching ||
    messageQuery.isFetching ||
    metricsQuery.isFetching;

  const m = metricsQuery.data;

  return (
    <main className="container">
      <h1>🚀 Go (Gin) + React + TanStack Query</h1>
      <p className="subtitle">
        Фронтенд отдаётся Go-сервером, данные приходят с <code>/api</code>
      </p>

      <div className="toolbar">
        <button onClick={refreshAll} disabled={isRefreshing}>
          {isRefreshing ? "Обновляю…" : "Обновить всё"}
        </button>
        {lastUpdatedAt && (
          <span className="toolbar__info">
            Последнее обновление: {lastUpdatedAt}
          </span>
        )}
      </div>

      {/* Блок системных метрик */}
      <Card title="Метрики сервера">
        {metricsQuery.isLoading ? (
          <p className="muted">Загрузка метрик…</p>
        ) : metricsQuery.isError ? (
          <pre className="error">Ошибка: {metricsQuery.error?.message}</pre>
        ) : (
          m && (
            <>
              <div className="metrics-grid">
                <MetricCard
                  label="CPU"
                  value={m.cpu}
                  sub={`${m.cpu_used_cores.toFixed(2)} / ${m.cpu_cores} ядер`}
                />
                <MetricCard
                  label="Память"
                  value={m.memory}
                  sub={`${fmtBytes(m.memory_used_bytes)} / ${fmtBytes(m.memory_total_bytes)}`}
                />
                <MetricCard
                  label="Диск"
                  value={m.disk}
                  sub={`${fmtBytes(m.disk_used_bytes)} / ${fmtBytes(m.disk_total_bytes)}`}
                />
                <NetworkCard
                  up={m.network_up}
                  rx={m.network.rx_bytes}
                  tx={m.network.tx_bytes}
                />
              </div>
              <p className="metric__meta">
                Аптайм: {m.uptime_seconds} сек · время: {m.timestamp}
              </p>
            </>
          )
        )}
      </Card>

      <Card
        title="Health"
        onRefresh={() => queryClient.refetchQueries(["health"])}
        refreshing={healthQuery.isFetching}
      >
        {healthQuery.isLoading ? (
          <p className="muted">Загрузка…</p>
        ) : healthQuery.isError ? (
          <pre className="error">Ошибка: {healthQuery.error?.message}</pre>
        ) : (
          <pre>{JSON.stringify(healthQuery.data, null, 2)}</pre>
        )}
      </Card>

      <Card
        title="Message"
        onRefresh={() => queryClient.refetchQueries(["message"])}
        refreshing={messageQuery.isFetching}
      >
        {messageQuery.isLoading ? (
          <p className="muted">Загрузка…</p>
        ) : messageQuery.isError ? (
          <pre className="error">Ошибка: {messageQuery.error?.message}</pre>
        ) : (
          <pre>{JSON.stringify(messageQuery.data, null, 2)}</pre>
        )}
      </Card>
    </main>
  );
}

export default App;
