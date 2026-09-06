// Форматирование дат.

// "2026-08-23" или "2026-08-24T21:05:00Z" -> "23.08.2026"
// Пустая строка/мусор возвращаются как есть.
export function formatDateDmy(dateStr) {
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(dateStr || "");
  if (!m) return dateStr || "";
  return `${m[3]}.${m[2]}.${m[1]}`;
}
