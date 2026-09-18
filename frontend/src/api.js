import { useQuery, useQueryClient } from "@tanstack/react-query";

// Базовый url — тот же хост, что отдал страницу (через Go-сервер,
// в dev — через vite-прокси на него).
const BASE = "";

// Глобальный обработчик «сессия истекла» (401). Регистрируется в App.jsx:
// при вызове сбрасывает кэш ["me"], и приложение показывает экран входа.
let onUnauthorized = null;
export function setOnUnauthorized(fn) {
  onUnauthorized = fn;
}

// Все API-запросы идут через глобальный fetch. Перехватываем 401 в одном
// месте, чтобы любое истечение сессии сразу возвращало пользователя на вход
// (кроме самого /api/login — там 401 значит «неверный пароль»).
const nativeFetch = window.fetch.bind(window);
window.fetch = async (...args) => {
  const res = await nativeFetch(...args);
  const url = typeof args[0] === "string" ? args[0] : (args[0]?.url || "");
  if (res.status === 401 && !url.includes("/api/login")) {
    onUnauthorized?.();
  }
  return res;
};

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

// Обновить данные текущего пользователя (PUT /api/me):
// необязательные телефон, Telegram и скорость чтения (0 = среднее).
export async function updateMe(
  { phone = "", telegram = "", reading_speed } = {}
) {
  const body = { phone, telegram };
  // reading_speed присылаем только явно — чтобы не сбрасывать значение
  // при сохранении контактов из других мест.
  if (reading_speed !== undefined) body.reading_speed = reading_speed;
  const res = await fetch(`${BASE}/api/me`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось сохранить профиль");
  return data;
}

// Обновить аватар текущего пользователя (PUT /api/me/avatar).
// payload: { preset? } или { photo_data?, photo_mime? }; пустое — сброс.
export async function updateAvatar(payload = {}) {
  const res = await fetch(`${BASE}/api/me/avatar`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось сохранить аватар");
  return data;
}

// Создать ссылку привязки Telegram (POST /api/me/telegram/link) — { url }.
export async function linkTelegram() {
  const res = await fetch(`${BASE}/api/me/telegram/link`, { method: "POST" });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось создать ссылку привязки");
  return data;
}

// Отвязать Telegram (POST /api/me/telegram/unlink).
export async function unlinkTelegram() {
  const res = await fetch(`${BASE}/api/me/telegram/unlink`, { method: "POST" });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось отключить Telegram");
  return data;
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

// Системные метрики сервера с /api/metrics.
// Опрашиваем только когда запрошено (enabled) — сейчас это раздел «Сервер»:
// на других страницах постоянный опрос не нужен.
export function useMetrics(refetchInterval = 5000, enabled = true) {
  return useQuery({
    queryKey: ["metrics"],
    queryFn: () => request("/api/metrics"),
    // Метрики обновляем периодически, чтобы показания были актуальными.
    staleTime: 2000,
    refetchInterval: enabled ? refetchInterval : false,
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

// Запросить откат задачи: dev-агент сделает git revert коммита задачи
// и передеплоит (PUT /api/app-tasks/:id с флагом revert_requested).
export async function requestTaskRollback(task) {
  const res = await fetch(`${BASE}/api/app-tasks/${task.id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      title: task.title,
      description: task.description,
      status: task.status,
      revert_requested: true,
    }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось запросить откат");
  return data;
}

// ==== Задачи (раздел «Задачи») ====

// Список задач и категорий с /api/tasks.
// Возвращает { tasks: [...], categories: [...] }.
export function useTasks(enabled = true) {
  return useQuery({
    queryKey: ["tasks"],
    queryFn: () => request("/api/tasks"),
    staleTime: 0,
    enabled,
    retry: 1,
  });
}

// Создать задачу (POST /api/tasks).
export async function createTask(payload) {
  const res = await fetch(`${BASE}/api/tasks`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось создать задачу");
  return data;
}

// Обновить задачу (PUT /api/tasks/:id).
export async function updateTask(id, payload) {
  const res = await fetch(`${BASE}/api/tasks/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось обновить задачу");
  return data;
}

// Удалить задачу (DELETE /api/tasks/:id).
export async function deleteTask(id) {
  const res = await fetch(`${BASE}/api/tasks/${id}`, {
    method: "DELETE",
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось удалить задачу");
  return data;
}

// ==== Цели ====

// Список целей с /api/goals.
export function useGoals(enabled = true) {
  return useQuery({
    queryKey: ["goals"],
    queryFn: () => request("/api/goals"),
    staleTime: 0,
    enabled,
    retry: 1,
  });
}

// Создать цель (POST /api/goals).
export async function createGoal(payload) {
  const res = await fetch(`${BASE}/api/goals`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось создать цель");
  return data;
}

// Обновить цель (PUT /api/goals/:id).
export async function updateGoal(id, payload) {
  const res = await fetch(`${BASE}/api/goals/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось обновить цель");
  return data;
}

// Удалить цель (DELETE /api/goals/:id).
// deleteTasks=true удаляет также привязанные к цели задачи.
export async function deleteGoal(id, deleteTasks = false) {
  const res = await fetch(
    `${BASE}/api/goals/${id}?delete_tasks=${deleteTasks ? 1 : 0}`,
    {
      method: "DELETE",
    },
  );
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось удалить цель");
  return data;
}

// Задать последовательность задач цели (PUT /api/goals/:id/tasks-order).
// taskIds — полный список id задач цели в нужном порядке.
export async function reorderGoalTasks(goalId, taskIds) {
  const res = await fetch(`${BASE}/api/goals/${goalId}/tasks-order`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ task_ids: taskIds }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok)
    throw new Error(data.error || "Не удалось изменить порядок задач");
  return data;
}

// Сгенерировать черновики задач для новой цели (POST /api/goals/generate-tasks).
// Ничего не сохраняет: возвращает список предлагаемых задач.
export async function generateGoalTasks(payload) {
  const res = await fetch(`${BASE}/api/goals/generate-tasks`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось сгенерировать задачи");
  return data;
}

// ==== Уведомления пользователя ====

// Список уведомлений с /api/notifications.
// Опрашивается каждые 30 секунд: сервер сдвигает «следующее время»
// периодических уведомлений при доставке.
export function useNotifications(enabled = true) {
  return useQuery({
    queryKey: ["notifications"],
    queryFn: () => request("/api/notifications"),
    staleTime: 0,
    enabled,
    retry: 1,
    refetchInterval: enabled ? 30_000 : false,
  });
}

// «Входящие»: наступившие по расписанию уведомления (колокольчик).
export function useNotificationInbox(enabled = true) {
  return useQuery({
    queryKey: ["notifications-inbox"],
    queryFn: () => request("/api/notifications/inbox"),
    staleTime: 0,
    enabled,
    retry: 1,
    refetchInterval: enabled ? 30_000 : false,
  });
}

// Закрыть «входящее» уведомление (DELETE /api/notifications/inbox/:id).
export async function dismissNotification(id) {
  const res = await fetch(`${BASE}/api/notifications/inbox/${id}`, {
    method: "DELETE",
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось закрыть уведомление");
  return data;
}

// Создать уведомление (POST /api/notifications).
export async function createNotification(payload) {
  const res = await fetch(`${BASE}/api/notifications`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось создать уведомление");
  return data;
}

// Удалить уведомление (DELETE /api/notifications/:id).
export async function deleteNotification(id) {
  const res = await fetch(`${BASE}/api/notifications/${id}`, {
    method: "DELETE",
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось удалить уведомление");
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

// ==== Раздел «Чтение» (книги fb2/epub → HTML) ====

// Список книг с /api/books.
export function useBooks(enabled = true) {
  return useQuery({
    queryKey: ["books"],
    queryFn: () => request("/api/books"),
    staleTime: 0,
    enabled,
    retry: 1,
  });
}

// Книга с текстом (GET /api/books/:id).
export async function fetchBook(id) {
  return request(`/api/books/${id}`);
}

// Загрузка книги (POST /api/books) — файл fb2/epub, конвертация на сервере.
export async function uploadBook(file) {
  const form = new FormData();
  form.append("file", file);
  const res = await fetch(`${BASE}/api/books`, {
    method: "POST",
    body: form,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось добавить книгу");
  return data;
}

// Удалить книгу (DELETE /api/books/:id).
export async function deleteBook(id) {
  const res = await fetch(`${BASE}/api/books/${id}`, { method: "DELETE" });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось удалить книгу");
  return data;
}

// Отметить книгу прочитанной или вернуть в чтение
// (PUT /api/books/:id/finished). Прочитанные книги уезжают в конец списка
// и не предлагаются для чтения в «Дне».
export async function setBookFinished(id, finished) {
  const res = await fetch(`${BASE}/api/books/${id}/finished`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ finished }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось отметить книгу");
  return data;
}

// Последняя закладка пользователя (GET /api/books/last-bookmark) — с неё
// продолжается чтение. Возвращает объект или null, если закладок нет.
export function useLastBookmark(enabled = true) {
  return useQuery({
    queryKey: ["last-bookmark"],
    queryFn: async () => {
      const res = await fetch(`${BASE}/api/books/last-bookmark`);
      if (!res.ok) {
        throw new Error(`Ошибка запроса /api/books/last-bookmark: ${res.status}`);
      }
      return res.json();
    },
    staleTime: 0,
    enabled,
    retry: 1,
  });
}

// Закладки книги (GET /api/books/:id/bookmarks).
export async function fetchBookmarks(id) {
  const res = await fetch(`${BASE}/api/books/${id}/bookmarks`);
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось загрузить закладки");
  return data;
}

// Добавить закладку (POST /api/books/:id/bookmarks).
// anchor — позиция выделения в символах от начала текста книги.
export async function addBookmark(id, anchor, excerpt) {
  const res = await fetch(`${BASE}/api/books/${id}/bookmarks`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ anchor, excerpt }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось сохранить закладку");
  return data;
}

// Удалить закладку (DELETE /api/books/:id/bookmarks/:bookmarkId).
export async function deleteBookmark(id, bookmarkId) {
  const res = await fetch(`${BASE}/api/books/${id}/bookmarks/${bookmarkId}`, {
    method: "DELETE",
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось удалить закладку");
  return data;
}

// Время чтения за день (GET /api/reading/time?date=ГГГГ-ММ-ДД).
// Ответ: { date, seconds, goal_seconds }.
export async function fetchReadingTime(date) {
  const res = await fetch(
    `${BASE}/api/reading/time?date=${encodeURIComponent(date)}`,
  );
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось загрузить время чтения");
  return data;
}

// Добавить секунды чтения к дню (POST /api/reading/time) — суммируется
// с уже сохранённым за этот день. keepalive позволяет отправить запрос
// в момент закрытия страницы.
export async function addReadingTime(date, seconds, keepalive = false) {
  const res = await fetch(`${BASE}/api/reading/time`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ date, seconds }),
    keepalive,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось сохранить время чтения");
  return data;
}

// Время чтения за день с сервера (react-query) — для карточки «Чтение» в «Дне».
// Ответ: { date, seconds, goal_seconds }.
export function useReadingTime(date, enabled = true) {
  return useQuery({
    queryKey: ["reading-time", date],
    queryFn: () => fetchReadingTime(date),
    staleTime: 0,
    enabled: enabled && Boolean(date),
    retry: 1,
  });
}

// Дни с чтением (GET /api/reading/history) — для отчётов: список всех дней,
// где накопились секунды, сразу.
export function useReadingHistory(enabled = true) {
  return useQuery({
    queryKey: ["reading-history"],
    queryFn: () => request("/api/reading/history"),
    staleTime: 0,
    enabled,
    retry: 1,
  });
}

// Цель чтения на день (PUT /api/reading/goal) — меняется для конкретной даты.
export async function setReadingGoal(date, goalSeconds) {
  const res = await fetch(`${BASE}/api/reading/goal`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ date, goal_seconds: goalSeconds }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось сохранить цель чтения");
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

// ==== Раздел «Заметки» (быстрые записи-черновики) ====

// Список заметок (GET /api/drafts): свежие сверху.
export function useDrafts(enabled = true) {
  return useQuery({
    queryKey: ["drafts"],
    queryFn: () => request("/api/drafts"),
    staleTime: 0,
    enabled,
    retry: 1,
  });
}

// Сохранить новую заметку (POST /api/drafts).
export async function createDraft(content) {
  const res = await fetch(`${BASE}/api/drafts`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ content }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось сохранить заметку");
  return data;
}

// Заменить текст заметки (PUT /api/drafts/:id).
export async function updateDraft(id, content) {
  const res = await fetch(`${BASE}/api/drafts/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ content }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось сохранить заметку");
  return data;
}

// Удалить заметку (DELETE /api/drafts/:id).
export async function deleteDraft(id) {
  const res = await fetch(`${BASE}/api/drafts/${id}`, { method: "DELETE" });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось удалить заметку");
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

// ==== День (ежедневный план) ====

// План на дату (GET /api/day?date=ГГГГ-ММ-ДД).
export async function fetchDayPlan(date) {
  return request(`/api/day?date=${date}`);
}

// История сформированных дней (GET /api/day/history).
export async function fetchDayHistory() {
  return request("/api/day/history");
}

// Предложения состава дня (POST /api/day/suggest).
export async function suggestDay(payload) {
  const res = await fetch(`${BASE}/api/day/suggest`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось сформировать день");
  return data;
}

// Сохранить план дня (PUT /api/day).
export async function saveDay(payload) {
  const res = await fetch(`${BASE}/api/day`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось сохранить день");
  return data;
}

// Отметить позицию плана дня выполненной/невыполненной (PUT /api/day/done).
// Вызов без сохранённого плана безопасен (ничего не обновит).
export async function setDayItemDone(date, kind, refId, done) {
  const res = await fetch(`${BASE}/api/day/done`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ date, kind, ref_id: refId, done }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Не удалось обновить позицию дня");
  return data;
}
