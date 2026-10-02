import * as React from "react"

import { cn } from "@/lib/utils"

// Textarea с авто-ростом по ФАКТИЧЕСКОМУ содержимому.
//
// Высота считается не по числу «логических» строк (переводов строк), а по
// реальной высоте текста: длинные строки браузер переносит сам, и такие
// переносы тоже учитываются. Снизу всегда остаётся одна пустая строка запаса.
//
// box-sizing: border-box (задан глобально в Tailwind), поэтому к scrollHeight
// добавляем толщину рамки и высоту одной строки.
const Textarea = React.forwardRef(function Textarea(
  { className, value, ...props },
  ref
) {
  const innerRef = React.useRef(null)

  // Внутренний ref нужен для измерений; внешний (если передан) пробрасываем.
  const setRef = React.useCallback(
    (node) => {
      innerRef.current = node
      if (typeof ref === "function") ref(node)
      else if (ref) ref.current = node
    },
    [ref]
  )

  const resize = React.useCallback(() => {
    const el = innerRef.current
    if (!el) return
    // Сбрасываем высоту, чтобы scrollHeight показал высоту самого контента
    // (с учётом переносов длинных строк).
    el.style.height = "auto"
    const cs = window.getComputedStyle(el)
    const parsed = parseFloat(cs.lineHeight)
    const line = Number.isFinite(parsed)
      ? parsed
      : parseFloat(cs.fontSize) * 1.625
    const border =
      (parseFloat(cs.borderTopWidth) || 0) +
      (parseFloat(cs.borderBottomWidth) || 0)
    // Контент + рамка + одна дополнительная строка.
    el.style.height = `${el.scrollHeight + border + line}px`
  }, [])

  // Пересчёт после монтирования и при каждом изменении значения.
  React.useLayoutEffect(() => {
    resize()
  }, [value, resize])

  // Переносы зависят от ширины — пересчитываем при её изменении.
  React.useEffect(() => {
    const el = innerRef.current
    if (!el || typeof ResizeObserver === "undefined") return
    let lastWidth = el.clientWidth
    const ro = new ResizeObserver(() => {
      // Реагируем только на изменение ширины, иначе будет цикл (меняем высоту).
      if (el.clientWidth === lastWidth) return
      lastWidth = el.clientWidth
      resize()
    })
    ro.observe(el)
    return () => ro.disconnect()
  }, [resize])

  return (
    <textarea
      ref={setRef}
      data-slot="textarea"
      value={value}
      className={cn(
        "w-full min-w-0 resize-none overflow-hidden rounded-lg border border-input bg-transparent px-3 py-2 text-sm leading-relaxed text-foreground transition-colors outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-input/30",
        className
      )}
      {...props}
      // rows=1 — базовый минимум до JS-замера; точную высоту задаёт resize().
      rows={1}
    />
  )
})

export { Textarea }
