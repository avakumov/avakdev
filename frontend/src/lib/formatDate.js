// Форматирование дат.

// "2026-08-23" или "2026-08-24T21:05:00Z" -> "23.08.2026"
// Пустая строка/мусор возвращаются как есть.
export function formatDateDmy(dateStr) {
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(dateStr || "");
  if (!m) return dateStr || "";
  return `${m[3]}.${m[2]}.${m[1]}`;
}

// Сегодняшняя дата в локальной таймзоне браузера: "2026-09-16".
// Даты дней в проекте считает клиент и присылает на сервер готовой строкой.
export function todayStr() {
  const d = new Date();
  const p = (n) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
}

// Секунды -> "чч:мм:сс" (счётчик времени чтения).
export function formatClock(totalSeconds) {
  const s = Math.max(0, Math.floor(totalSeconds || 0));
  const p = (n) => String(n).padStart(2, "0");
  return `${p(Math.floor(s / 3600))}:${p(Math.floor((s % 3600) / 60))}:${p(s % 60)}`;
}
