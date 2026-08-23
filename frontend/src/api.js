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

// ==== «Важное» сообщение ====

// Текущее «важное» сообщение с /api/important.
// Возвращает content, а также enabled (показываем только на production)
// и seen_today (сообщение показывается раз в сутки).
export function useImportant(enabled = true) {
  return useQuery({
    queryKey: ["important"],
    queryFn: () => request("/api/important"),
    staleTime: 0,
    enabled,
    retry: 1,
  });
}

// Сохранить текст «важного» сообщения (PUT /api/important).
// Доступно только администраторам.
export async function saveImportantMessage(content) {
  const res = await fetch(`${BASE}/api/important`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ content }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось сохранить сообщение");
  return data;
}

// Отметить, что пользователь прочитал сообщение сегодня
// (POST /api/important/seen). До следующего дня сообщение не покажется.
export async function markImportantSeen() {
  const res = await fetch(`${BASE}/api/important/seen`, { method: "POST" });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось отметить прочитанным");
  return data;
}

// ==== Пользовательские метрики ====

// Определения метрик пользователя и их значения с /api/user-metrics.
// Возвращает { definitions: [{id, name, type, created}], values: [{metric_id, date, value}] }.
export function useUserMetrics(enabled = true) {
  return useQuery({
    queryKey: ["user-metrics"],
    queryFn: () => request("/api/user-metrics"),
    staleTime: 0,
    enabled,
    retry: 1,
  });
}

// Создать новую метрику (POST /api/user-metrics). Тип: "int" | "float" | "bool".
export async function createUserMetric(name, type, unit = "") {
  const res = await fetch(`${BASE}/api/user-metrics`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, type, unit }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось создать метрику");
  return data;
}

// Удалить метрику вместе со значениями (DELETE /api/user-metrics/:id).
export async function deleteUserMetric(id) {
  const res = await fetch(`${BASE}/api/user-metrics/${id}`, {
    method: "DELETE",
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось удалить метрику");
  return data;
}

// Переименовать метрику и поменять её единицу измерения (PUT /api/user-metrics/:id).
export async function updateUserMetric(id, name, unit) {
  const res = await fetch(`${BASE}/api/user-metrics/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, unit }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось изменить метрику");
  return data;
}

// Сохранить показатель метрики за день (PUT /api/user-metrics/:id/:date).
// За день фиксируется одно значение — повторное сохранение перезаписывает.
export async function setUserMetricValue(id, date, value) {
  const res = await fetch(`${BASE}/api/user-metrics/${id}/${date}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ value }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось сохранить показатель");
  return data;
}

// Удалить показатель метрики за конкретный день (DELETE /api/user-metrics/:id/:date).
export async function deleteUserMetricValue(id, date) {
  const res = await fetch(`${BASE}/api/user-metrics/${id}/${date}`, {
    method: "DELETE",
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось удалить показатель");
  return data;
}

// ==== Задачи по модификации приложения ====

// Список задач с /api/app-tasks. Возвращает { tasks: [{id, title, description, status, created, updated}] }.
export function useAppTasks(enabled = true) {
  return useQuery({
    queryKey: ["app-tasks"],
    queryFn: () => request("/api/app-tasks"),
    staleTime: 0,
    enabled,
    retry: 1,
  });
}

// Создать задачу (POST /api/app-tasks).
export async function createAppTask(title, description) {
  const res = await fetch(`${BASE}/api/app-tasks`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ title, description }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось создать задачу");
  return data;
}

// Обновить задачу: заголовок, описание, статус (PUT /api/app-tasks/:id).
export async function updateAppTask(id, { title, description, status }) {
  const res = await fetch(`${BASE}/api/app-tasks/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ title, description, status }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось обновить задачу");
  return data;
}

// Удалить задачу (DELETE /api/app-tasks/:id).
export async function deleteAppTask(id) {
  const res = await fetch(`${BASE}/api/app-tasks/${id}`, {
    method: "DELETE",
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось удалить задачу");
  return data;
}

// Запросить деплой задачи: dev-агент закоммитит изменения и запустит
// make deploy (PUT /api/app-tasks/:id с флагом deploy_requested).
export async function requestTaskDeploy(task) {
  const res = await fetch(`${BASE}/api/app-tasks/${task.id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      title: task.title,
      description: task.description,
      status: task.status,
      deploy_requested: true,
    }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось запросить деплой");
  return data;
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

// Сгенерировать аудио для конспекта через Yandex SpeechKit (POST /api/knowledge/:id/tts).
// Возвращает Blob аудио. Если аудио уже сгенерировано — сервер вернёт его без
// повторного обращения к SpeechKit (экономия токенов/квоты).
export async function synthesizeNoteAudio(id) {
  const res = await fetch(`${BASE}/api/knowledge/${id}/tts`, {
    method: "POST",
  });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data.error || "Не удалось сгенерировать аудио");
  }
  return res.blob();
}

// Проверить наличие аудио и получить его, если оно уже сгенерировано
// (GET /api/knowledge/:id/tts). Возвращает Blob или null, если аудио нет.
export async function getNoteAudio(id) {
  const res = await fetch(`${BASE}/api/knowledge/${id}/tts`);
  if (res.status === 404) return null;
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data.error || "Не удалось получить аудио");
  }
  return res.blob();
}
