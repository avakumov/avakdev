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

// ==== Дневные отчёты ====

// Список отчётов с /api/reports
export function useReports(enabled = true) {
  return useQuery({
    queryKey: ["reports"],
    queryFn: () => request("/api/reports"),
    staleTime: 0,
    enabled,
    retry: 1,
  });
}

// Создать или обновить отчёт за конкретный день (PUT /api/reports/:date)
export async function updateReport(date, content) {
  const res = await fetch(`${BASE}/api/reports/${date}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ content }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось сохранить отчёт");
  return data;
}

// ==== Профиль и резюме ====

// Текущий профиль (описание + сгенерированное резюме) с /api/profile
export function useProfile(enabled = true) {
  return useQuery({
    queryKey: ["profile"],
    queryFn: () => request("/api/profile"),
    staleTime: 0,
    enabled,
    retry: 1,
  });
}

// Сохранить описание профиля (PUT /api/profile)
export async function saveProfile(description) {
  const res = await fetch(`${BASE}/api/profile`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ description }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось сохранить профиль");
  return data;
}

// Сгенерировать резюме через DeepSeek (POST /api/profile/generate)
export async function generateResume() {
  const res = await fetch(`${BASE}/api/profile/generate`, {
    method: "POST",
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось сгенерировать резюме");
  return data;
}

// Сохранить отредактированный текст резюме (PUT /api/profile/resume)
export async function saveResumeText(resume) {
  const res = await fetch(`${BASE}/api/profile/resume`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ resume }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось сохранить резюме");
  return data;
}

// Прямая ссылка на отдельную HTML-страницу резюме
// (страница содержит только резюме в HTML+CSS).
export function profileResumeUrl() {
  return `${BASE}/api/profile/resume`;
}

// Загрузить фото для резюме (multipart, поле "photo").
// Возвращает обновлённый профиль вместе с photo_data/photo_mime.
export async function uploadProfilePhoto(file) {
  const form = new FormData();
  form.append("photo", file);
  const res = await fetch(`${BASE}/api/profile/photo`, {
    method: "POST",
    body: form,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось загрузить фото");
  return data;
}

// Удалить фото из резюме.
export async function deleteProfilePhoto() {
  const res = await fetch(`${BASE}/api/profile/photo`, {
    method: "DELETE",
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось удалить фото");
  return data;
}

// ==== Конспекты знаний ====

// Список конспектов с /api/knowledge
export function useKnowledge(enabled = true) {
  return useQuery({
    queryKey: ["knowledge"],
    queryFn: () => request("/api/knowledge"),
    staleTime: 0,
    enabled,
    retry: 1,
  });
}

// Отметить повторение конспекта (POST /api/knowledge/:id/repeat).
// Увеличивает счётчик повторений на 1.
export async function repeatKnowledge(id) {
  const res = await fetch(`${BASE}/api/knowledge/${id}/repeat`, {
    method: "POST",
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось отметить повторение");
  return data;
}

// Сгенерировать краткий конспект по теме через DeepSeek (POST /api/knowledge/generate)
export async function generateKnowledge(topic) {
  const res = await fetch(`${BASE}/api/knowledge/generate`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ topic }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok)
    throw new Error(data.error || "Не удалось сгенерировать конспект");
  return data;
}

// Сохранить новый конспект (POST /api/knowledge)
export async function createKnowledge(note) {
  const res = await fetch(`${BASE}/api/knowledge`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(note),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось сохранить конспект");
  return data;
}

// Обновить конспект (PUT /api/knowledge/:id)
export async function updateKnowledge(id, note) {
  const res = await fetch(`${BASE}/api/knowledge/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(note),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось обновить конспект");
  return data;
}

// Удалить конспект (DELETE /api/knowledge/:id)
export async function deleteKnowledge(id) {
  const res = await fetch(`${BASE}/api/knowledge/${id}`, {
    method: "DELETE",
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось удалить конспект");
  return data;
}
