import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { Clock } from "lucide-react";

// Поле ввода времени в 24-часовом формате ЧЧ:ММ.
// Наружу значение отдаётся строкой "HH:MM" (или ""), внутри — с маской:
// автоматически подставляется двоеточие, часы 00–23, минуты 00–59.
// Кнопка-часики открывает popup выбора времени (24 часа, без AM/PM),
// который рендерится в portal поверх любых модалок.
function TimeInput({ value, onChange, className, ...props }) {
  const rootRef = useRef(null);
  const [text, setText] = useState(value || "");
  const [focused, setFocused] = useState(false);
  const [open, setOpen] = useState(false);
  const [draftHour, setDraftHour] = useState(null); // час в открытом пикере
  const [pos, setPos] = useState({ top: 0, left: 0 }); // позиция popup

  // Внешнее изменение (сброс формы) — обновляем текст, но не перетираем
  // то, что пользователь сейчас набирает.
  useEffect(() => {
    if (!focused) setText(value || "");
  }, [value, focused]);

  const handleTextChange = (e) => {
    // Оставляем только цифры, максимум 4 (ЧЧММ).
    const digits = e.target.value.replace(/\D/g, "").slice(0, 4);
    const masked =
      digits.length <= 2 ? digits : `${digits.slice(0, 2)}:${digits.slice(2)}`;
    setText(masked);

    if (digits.length === 4) {
      const hh = Number(digits.slice(0, 2));
      const mm = Number(digits.slice(2));
      if (hh <= 23 && mm <= 59) {
        onChange(`${String(hh).padStart(2, "0")}:${String(mm).padStart(2, "0")}`);
        return;
      }
      // Некорректное время (например 25:xx) — значение не обновляем.
    }
    if (masked === "") onChange("");
  };

  const current = (value || "").match(/^(\d{2}):(\d{2})$/);

  // Открываем popup: позиция считается от поля ввода (в координатах вьюпорта).
  const openPicker = () => {
    const el = rootRef.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
    const W = 264; // ширина popup
    const left = Math.max(
      8,
      Math.min(r.right - W, window.innerWidth - W - 8),
    );
    setPos({ top: r.bottom + 6, left });
    setDraftHour(null);
    setOpen(true);
  };

  const applyMinute = (mm) => {
    const hh = draftHour ?? (current ? Number(current[1]) : 0);
    const time = `${String(hh).padStart(2, "0")}:${String(mm).padStart(2, "0")}`;
    onChange(time);
    setOpen(false);
  };

  const hours = Array.from({ length: 24 }, (_, i) => i);
  const minutesStep5 = Array.from({ length: 12 }, (_, i) => i * 5);

  const selHour = draftHour ?? (current ? Number(current[1]) : null);
  const selMinute = current ? Number(current[2]) : 0;

  const picker = (
    <div
      role="dialog"
      aria-label="Выбор времени"
      style={{ top: pos.top, left: pos.left }}
      className="fixed z-70 w-66 rounded-lg border border-border bg-popover p-3 text-popover-foreground shadow-lg"
    >
      <p className="mb-1.5 text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
        Часы (00–23)
      </p>
      <div className="grid grid-cols-6 gap-1">
        {hours.map((h) => (
          <button
            key={h}
            type="button"
            onClick={() => setDraftHour(h)}
            className={cn(
              "flex h-7 cursor-pointer items-center justify-center rounded-md text-sm tabular-nums transition-colors hover:bg-accent",
              selHour === h && "bg-accent font-semibold",
            )}
          >
            {String(h).padStart(2, "0")}
          </button>
        ))}
      </div>

      <p className="mt-2 mb-1.5 text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
        Минуты
      </p>
      <div className="grid grid-cols-6 gap-1">
        {minutesStep5.map((m) => (
          <button
            key={m}
            type="button"
            onClick={() => applyMinute(m)}
            className={cn(
              "flex h-7 cursor-pointer items-center justify-center rounded-md text-sm tabular-nums transition-colors hover:bg-accent",
              value && selMinute === m && "bg-accent font-semibold",
            )}
          >
            {String(m).padStart(2, "0")}
          </button>
        ))}
      </div>
      <p className="mt-2 text-[10px] text-muted-foreground">
        Точные минуты можно ввести вручную в поле
      </p>
    </div>
  );

  return (
    <div ref={rootRef} className={cn("relative", className)}>
      <Input
        value={text}
        onChange={handleTextChange}
        onFocus={() => setFocused(true)}
        onBlur={() => {
          setFocused(false);
          setText(value || "");
        }}
        placeholder="ЧЧ:ММ"
        inputMode="numeric"
        className="pr-9 tabular-nums"
        {...props}
      />
      <button
        type="button"
        onClick={openPicker}
        tabIndex={-1}
        aria-label="Выбрать время"
        className="absolute top-1/2 right-2 -translate-y-1/2 text-muted-foreground transition-colors hover:text-foreground"
      >
        <Clock className="size-4" />
      </button>

      {open && (
        <>
          {/* Клик мимо — закрыть */}
          <div
            className="fixed inset-0 z-60"
            onClick={() => setOpen(false)}
            aria-hidden="true"
          />
          {createPortal(picker, document.body)}
        </>
      )}
    </div>
  );
}

export default TimeInput;
