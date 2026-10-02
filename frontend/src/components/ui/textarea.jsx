import * as React from "react"

import { cn } from "@/lib/utils"

const BASE =
  "w-full min-w-0 resize-none overflow-hidden rounded-lg border border-input bg-transparent px-3 py-2 text-sm leading-relaxed text-foreground transition-colors outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-input/30"

// Свойства, влияющие на перенос строк и высоту. Копируем их с реального поля
// на скрытое «зеркало», чтобы замер высоты совпадал один в один.
const MIRROR_PROPS = [
  "box-sizing",
  "width",
  "font-family",
  "font-size",
  "font-weight",
  "font-style",
  "letter-spacing",
  "line-height",
  "text-transform",
  "text-indent",
  "word-spacing",
  "tab-size",
  "padding-top",
  "padding-right",
  "padding-bottom",
  "padding-left",
  "border-top-width",
  "border-right-width",
  "border-bottom-width",
  "border-left-width",
  "overflow-wrap",
  "word-break",
  "white-space",
]

// Высота одной строки (px): line-height, с запасным вариантом, если он "normal".
function linePx(cs) {
  const v = parseFloat(cs.lineHeight)
  return Number.isFinite(v) ? v : parseFloat(cs.fontSize) * 1.625
}

function borderPx(cs) {
  return (parseFloat(cs.borderTopWidth) || 0) + (parseFloat(cs.borderBottomWidth) || 0)
}

// Скрытое «зеркало» для замера. Одно на всё приложение: замер синхронный,
// поэтому делить его между компонентами безопасно.
let mirrorEl = null
function getMirror() {
  if (typeof document === "undefined") return null
  if (mirrorEl && mirrorEl.isConnected) return mirrorEl
  const m = document.createElement("textarea")
  m.setAttribute("aria-hidden", "true")
  m.tabIndex = -1
  m.rows = 1
  m.wrap = "soft"
  Object.assign(m.style, {
    position: "absolute",
    top: "0",
    left: "-9999px",
    height: "auto",
    visibility: "hidden",
    overflow: "hidden",
    resize: "none",
    margin: "0",
    pointerEvents: "none",
    zIndex: "-1",
  })
  document.body.appendChild(m)
  mirrorEl = m
  return m
}

// Textarea с авто-ростом по ФАКТИЧЕСКОМУ содержимому.
//
// Высота = высота текста (с учётом переносов длинных строк, которые делает
// браузер) + рамка + одна дополнительная строка снизу.
//
// Замер идёт на скрытом offscreen-«зеркале», а не на самом поле: реальное поле
// НЕ схлопывается перед замером. Иначе при печати документ на миг становится
// короче, прокрученная страница «перескакивает» вверх и строка уезжает вниз
// экрана (заметно на длинных страницах).
const Textarea = React.forwardRef(function Textarea(
  { className, value, ...props },
  ref
) {
  const innerRef = React.useRef(null)

  // Внешний ref (если передан) + внутренний, нужный для замеров.
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
    const cs = window.getComputedStyle(el)
    const extra = borderPx(cs) + linePx(cs)
    const mirror = getMirror()
    if (mirror) {
      for (const p of MIRROR_PROPS) {
        mirror.style.setProperty(p, cs.getPropertyValue(p))
      }
      mirror.value = el.value
      el.style.height = `${mirror.scrollHeight + extra}px`
    } else {
      // Фолбэк без DOM-замера.
      el.style.height = "auto"
      el.style.height = `${el.scrollHeight + extra}px`
    }
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

  // Веб-шрифт может догрузиться после первого замера — пересчитываем высоту.
  React.useEffect(() => {
    if (typeof document === "undefined" || !document.fonts || !document.fonts.ready) {
      return
    }
    let alive = true
    document.fonts.ready.then(() => {
      if (alive) resize()
    })
    return () => {
      alive = false
    }
  }, [resize])

  return (
    <textarea
      ref={setRef}
      data-slot="textarea"
      value={value}
      className={cn(BASE, className)}
      {...props}
      // rows=1 — базовый минимум до JS-замера; точную высоту задаёт resize().
      rows={1}
    />
  )
})

export { Textarea }
