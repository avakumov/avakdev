import { formatDateDmy, formatTime } from "@/lib/formatDate.js";

// Единый компонент отображения даты в формате дд.мм.гггг.
// Принимает строку даты ("2026-08-23") или дату-время
// ("2026-08-24T21:05:00Z"); с withTime дополнительно показывает время ЧЧ:ММ.
// Если даты нет — ничего не рендерит.
function DateDisplay({ date, withTime = false, className, ...props }) {
  if (!date) return null;
  const text = formatDateDmy(date);
  const time = withTime ? formatTime(date) : "";
  return (
    <span className={className} {...props}>
      {text}
      {time ? ` ${time}` : ""}
    </span>
  );
}

export default DateDisplay;
