import { useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { cn } from "@/lib/utils";

// OverlayInput — ввод для узкого места (например, ячейки таблицы), где обычного
// поля не видно при наборе.
//
// В ячейке остаётся только невидимая «подложка» нужного размера, поэтому вёрстка
// не меняется ни при вводе, ни при фокусе. Само поле ввода рисуется ПОВЕРХ через
// портал (position: fixed), поэтому его не обрезают контейнеры со скроллом
// (таблица метрик прокручивается по горизонтали) и оно расширяется по длине
// введённого текста. Как только фокус уходит — поле исчезает.
//
// Пропсы совпадают с обычным <input/>; wrapperClassName задаёт размер подложки
// (высоту). Если передан непустой error — поле краснеет, а ПОД ним показывается
// сообщение. Сообщение вынесено из потока (absolute) и не меняет размер поля,
// поэтому инпут не смещается при его появлении.
function OverlayInput({
  value,
  onChange,
  onKeyDown,
  onBlur,
  autoFocus,
  type = "text",
  className,
  wrapperClassName,
  error,
  ...props
}) {
  const anchorRef = useRef(null);
  const inputRef = useRef(null);
  const didFocus = useRef(false);
  const [rect, setRect] = useState(null);

  // Позиция поля: берём границы подложки. Пересчитываем при изменении размера
  // окна и при прокрутке (в том числе внутри контейнеров таблицы).
  useLayoutEffect(() => {
    const el = anchorRef.current;
    if (!el) return;
    const update = () => setRect(el.getBoundingClientRect());
    update();
    window.addEventListener("resize", update);
    window.addEventListener("scroll", update, true);
    return () => {
      window.removeEventListener("resize", update);
      window.removeEventListener("scroll", update, true);
    };
  }, []);

  // Автофокус — один раз, после того как поле отрисовано.
  useLayoutEffect(() => {
    if (autoFocus && rect && !didFocus.current && inputRef.current) {
      inputRef.current.focus();
      didFocus.current = true;
    }
  }, [autoFocus, rect]);

  const text = value == null ? "" : String(value);
  // Ширина по числу символов (ch) + запас под padding, границы и курсор.
  const widthCh = Math.max([...text].length, 3) + 2;

  const vw = typeof window === "undefined" ? 0 : window.innerWidth;
  // У правого края окна разворачиваем поле влево, чтобы оно не ушло за экран.
  const alignRight = rect ? rect.left > vw / 2 : false;
  const pos = rect
    ? alignRight
      ? { right: Math.max(0, vw - rect.right), top: rect.top }
      : { left: rect.left, top: rect.top }
    : null;

  return (
    <>
      <span
        ref={anchorRef}
        aria-hidden="true"
        className={cn("block h-8 w-full", wrapperClassName)}
      />
      {pos &&
        createPortal(
          <div
            className="fixed z-50"
            style={{ ...pos, width: `${widthCh}ch`, height: rect.height }}
          >
            <input
              ref={inputRef}
              {...props}
              type={type}
              value={value}
              onChange={onChange}
              onKeyDown={onKeyDown}
              onBlur={onBlur}
              aria-invalid={error ? true : undefined}
              className={cn(
                "h-full w-full min-w-0 rounded-md border bg-background px-1 text-center text-sm tabular-nums shadow-lg outline-none focus-visible:ring-2",
                error
                  ? "border-destructive focus-visible:ring-destructive/30"
                  : "border-ring focus-visible:ring-ring/40",
                className,
              )}
            />
            {error && (
              <span
                className={cn(
                  "absolute top-full mt-1 rounded bg-destructive px-1.5 py-0.5 text-xs whitespace-nowrap text-white shadow",
                  alignRight ? "right-0" : "left-0",
                )}
              >
                {error}
              </span>
            )}
          </div>,
          document.body,
        )}
    </>
  );
}

export default OverlayInput;
