import { useQuery, useQueryClient } from "@tanstack/react-query";

// Базовый url — тот же хост, что отдал страницу (через Go-сервер,
// в dev — через vite-прокси на него).
const BASE = "";

async function request(path) {
  const res = await fetch(`${BASE}${path}`);
  if (!res.ok) {
    throw new Error(`Ошибка запроса ${path}: ${res.status}`);
  }
  return res.json();
}

// ==== Авторизация (TanStack Query) ====

// /api/me — текущий пользователь. Вызывается при загрузке страницы,
// чтобы понять, вошёл ли пользователь в систему.
// Возвращает объект пользователя, null (если не авторизован) или выбрасывает
// ошибку при сбое сервера.
export function useMe() {
  return useQuery({
    queryKey: ["me"],
    queryFn: async () => {
      const res = await fetch(`${BASE}/api/me`);
      if (res.status === 401 || res.status === 403) return null;
      if (!res.ok) {
        throw new Error(`Ошибка запроса /api/me: ${res.status}`);
      }
      return res.json();
    },
    staleTime: 0,
    retry: false,
  });
}

// /api/login — вход по логину/паролю.
export async function login(username, password) {
  const res = await fetch(`${BASE}/api/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось войти");
  return data;
}

// /api/logout — выход из системы.
export async function logout() {
  await fetch(`${BASE}/api/logout`, { method: "POST" });
}

// Использовать с useQueryClient().invalidateQueries(["me"]) после входа/выхода.
export function useInvalidateMe() {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: ["me"] });
}

// Данные endpoint'а /api/health
export function useHealth(enabled = true) {
  return useQuery({
    queryKey: ["health"],
    queryFn: () => request("/api/health"),
    // Серверные данные достаточно обновлять редко.
    staleTime: 30_000,
    enabled,
    retry: 1,
  });
}

// Приветственное сообщение с /api/message
export function useMessage(enabled = true) {
  return useQuery({
    queryKey: ["message"],
    queryFn: () => request("/api/message"),
    staleTime: 30_000,
    enabled,
    retry: 1,
  });
}

// Системные метрики сервера с /api/metrics
export function useMetrics(refetchInterval = 5000, enabled = true) {
  return useQuery({
    queryKey: ["metrics"],
    queryFn: () => request("/api/metrics"),
    // Метрики обновляем периодически, чтобы показания были актуальными.
    staleTime: 2000,
    refetchInterval,
    enabled,
    retry: 1,
  });
}
