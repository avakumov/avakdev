// Признак локального (dev) запуска приложения: dev-сборка Vite или обращение
// по localhost/127.0.0.1 (в том числе к локальному Go-серверу на 127.0.0.1:8080).
// На production (avakumov.ru) остаётся false.
export const IS_LOCAL_DEV =
  import.meta.env.DEV ||
  (typeof window !== "undefined" &&
    ["localhost", "127.0.0.1", "::1"].includes(window.location.hostname));
