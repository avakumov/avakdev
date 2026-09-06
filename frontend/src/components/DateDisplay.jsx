import { formatDateDmy } from "@/lib/formatDate.js";

const pad = (n) => String(n).padStart(2, "0");

// Единый компонент отображения даты в формате дд.мм.гггг.
// - "2026-08-23" (дата без времени) — показывается как есть;
// - "2026-08-24T21:05:00Z" (дата+время) — переводится в ЛОКАЛЬНЫЙ пояс
//   браузера, чтобы уведомления и «обновлено» показывали местное время,
//   а не серверное (UTC).
// Если даты нет — ничего не рендерит.
function DateDisplay({ date, withTime = false, className, ...props }) {
  if (!date) return null;

  const hasTime = typeof date === "string" && /T\d{2}:\d{2}/.test(date);
  const parsed = hasTime ? new Date(date) : null;
  const valid = parsed && !Number.isNaN(parsed.getTime());

  let text;
  if (valid) {
    text = `${pad(parsed.getDate())}.${pad(parsed.getMonth() + 1)}.${parsed.getFullYear()}`;
  } else {
    text = formatDateDmy(date);
  }

  let time = "";
  if (withTime) {
    if (valid) {
      time = `${pad(parsed.getHours())}:${pad(parsed.getMinutes())}`;
    } else if (hasTime) {
      const m = /T(\d{2}):(\d{2})/.exec(date);
      if (m) time = `${m[1]}:${m[2]}`;
    }
  }

  return (
    <span className={className} {...props}>
      {text}
      {time ? ` ${time}` : ""}
    </span>
  );
}

export default DateDisplay;
