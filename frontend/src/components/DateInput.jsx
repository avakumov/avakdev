import { useEffect, useRef, useState } from "react";
import { Input } from "@/components/ui/input";
import { formatDateDmy } from "@/lib/formatDate.js";
import { cn } from "@/lib/utils";
import { CalendarDays } from "lucide-react";

// Поле ввода даты в формате дд.мм.гггг.
// Наружу значение отдаётся в ISO (YYYY-MM-DD) через onChange, внутри
// отображается как дд.мм.гггг. Кнопка-календарь открывает нативный выбор
// даты браузера (поддерживается max — недоступные даты блокируются).
function DateInput({ value, max, onChange, className, ...props }) {
  const hiddenRef = useRef(null);
  const [text, setText] = useState(value ? formatDateDmy(value) : "");
  const [focused, setFocused] = useState(false);

  // Внешнее изменение (сброс формы, выбор через календарь) — обновляем текст,
  // но не перетираем то, что пользователь сейчас набирает.
  useEffect(() => {
    if (!focused) setText(value ? formatDateDmy(value) : "");
  }, [value, focused]);

  const handleTextChange = (e) => {
    const raw = e.target.value;
    setText(raw);
    const m = /^(\d{1,2})[./-](\d{1,2})[./-](\d{4})$/.exec(raw.trim());
    if (m) {
      onChange(`${m[3]}-${m[2].padStart(2, "0")}-${m[1].padStart(2, "0")}`);
    } else if (raw.trim() === "") {
      onChange("");
    }
  };

  const openPicker = () => {
    // showPicker доступен в современных браузерах; иначе — программный клик.
    if (hiddenRef.current?.showPicker) {
      try {
        hiddenRef.current.showPicker();
        return;
      } catch {
        /* не поддерживается — fallback ниже */
      }
    }
    hiddenRef.current?.click();
  };

  return (
    <div className={cn("relative", className)}>
      <Input
        value={text}
        onChange={handleTextChange}
        onFocus={() => setFocused(true)}
        onBlur={() => {
          setFocused(false);
          setText(value ? formatDateDmy(value) : "");
        }}
        placeholder="дд.мм.гггг"
        inputMode="numeric"
        className="pr-9"
        {...props}
      />
      <input
        ref={hiddenRef}
        type="date"
        value={value || ""}
        max={max}
        onChange={(e) => onChange(e.target.value)}
        tabIndex={-1}
        aria-hidden="true"
        className="sr-only"
      />
      <button
        type="button"
        onClick={openPicker}
        tabIndex={-1}
        aria-label="Выбрать дату"
        className="absolute top-1/2 right-2 -translate-y-1/2 text-muted-foreground transition-colors hover:text-foreground"
      >
        <CalendarDays className="size-4" />
      </button>
    </div>
  );
}

export default DateInput;
