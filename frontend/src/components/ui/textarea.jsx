import * as React from "react"

import { cn } from "@/lib/utils"

// Число строк текста в значении (считаем по явным переводам строк).
// Пустое значение считаем одной строкой — тогда поле покажет две строки.
function lineCount(value) {
  if (value == null || value === "") return 1
  return String(value).split("\n").length
}

// Textarea с авто-ростом: отображаемое число строк всегда на 1 больше, чем
// строк текста в значении. Появление новой строки (Enter) увеличивает поле
// ровно на одну строку. Оформление — в стиле shadcn/ui (как Input).
//
// Значение контролируемое (value): строки считаются от него, поэтому в
// каждом месте использования нужно передавать value + onChange.
const Textarea = React.forwardRef(function Textarea(
  { className, value, ...props },
  ref
) {
  return (
    <textarea
      ref={ref}
      data-slot="textarea"
      value={value}
      className={cn(
        "w-full min-w-0 resize-none rounded-lg border border-input bg-transparent px-3 py-2 text-sm leading-relaxed text-foreground transition-colors outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-input/30",
        className
      )}
      {...props}
      // rows задаём после props, чтобы переданный снаружи rows не перебивал
      // авто-рост (строки текста + 1).
      rows={lineCount(value) + 1}
    />
  )
})

export { Textarea }
