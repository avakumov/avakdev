import { useHealth, useMessage } from "./api.js";
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

function App() {
  // TanStack Query — данные с сервера
  const healthQuery = useHealth();
  const messageQuery = useMessage();

  // Zustand — глобальное UI-состояние
  const queryClient = useQueryClient();
  const lastUpdatedAt = useAppStore((s) => s.lastUpdatedAt);
  const setLastUpdatedAt = useAppStore((s) => s.setLastUpdatedAt);
  const incrementLoaded = useAppStore((s) => s.incrementLoaded);

  const refreshAll = async () => {
    await Promise.all([
      queryClient.refetchQueries(["health"]),
      queryClient.refetchQueries(["message"]),
    ]);
    setLastUpdatedAt(new Date().toLocaleTimeString());
    incrementLoaded();
  };

  const isRefreshing = healthQuery.isFetching || messageQuery.isFetching;

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
