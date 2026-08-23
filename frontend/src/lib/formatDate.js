// Форматирование дат по-русски.

const MONTHS = [
  "января",
  "февраля",
  "марта",
  "апреля",
  "мая",
  "июня",
  "июля",
  "августа",
  "сентября",
  "октября",
  "ноября",
  "декабря",
];

// "2026-08-23" -> "23 августа 2026"
export function formatDateRu(dateStr) {
  const [y, m, d] = dateStr.split("-").map(Number);
  if (!y || !m || !d) return dateStr;
  return `${d} ${MONTHS[m - 1]} ${y}`;
}
