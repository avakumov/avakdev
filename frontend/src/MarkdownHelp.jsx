import MarkdownView from "./MarkdownView.jsx";
import { Card, CardHeader, CardTitle, CardContent, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { X, HelpCircle } from "lucide-react";

// Справка по Markdown: каждый пункт — заголовок, синтаксис (как писать)
// и наглядный пример (как будет выглядеть).
const HELP_ITEMS = [
  {
    title: "Заголовки",
    source: `# Заголовок 1
## Заголовок 2
### Заголовок 3`,
    example: `# Заголовок 1
## Заголовок 2
### Заголовок 3`,
  },
  {
    title: "Оформление текста",
    source: `**жирный текст**
*курсив*
~~зачёркнутый~~
\`код внутри строки\``,
    example: `**жирный текст**
*курсив*
~~зачёркнутый~~
\`код внутри строки\``,
  },
  {
    title: "Маркированный список",
    source: `- пункт 1
- пункт 2
  - вложенный пункт`,
    example: `- пункт 1
- пункт 2
  - вложенный пункт`,
  },
  {
    title: "Нумерованный список",
    source: `1. первый шаг
2. второй шаг
3. третий шаг`,
    example: `1. первый шаг
2. второй шаг
3. третий шаг`,
  },
  {
    title: "Чек-лист",
    source: `- [x] сделано
- [ ] в работе
- [ ] запланировано`,
    example: `- [x] сделано
- [ ] в работе
- [ ] запланировано`,
  },
  {
    title: "Ссылки",
    source: `[текст ссылки](https://example.com)`,
    example: `[текст ссылки](https://example.com)`,
  },
  {
    title: "Цитата",
    source: `> важная мысль или напоминание`,
    example: `> важная мысль или напоминание`,
  },
  {
    title: "Блок кода",
    source: `\`\`\`go
func main() {
    fmt.Println("Привет!")
}
\`\`\``,
    example: "```go\nfunc main() {\n    fmt.Println(\"Привет!\")\n}\n```",
  },
  {
    title: "Таблица",
    source: `| Пункт      | Статус  |
|------------|---------|
| Отчёт      | готово  |
| Конспект   | в работе |`,
    example: `| Пункт      | Статус  |
|------------|---------|
| Отчёт      | готово  |
| Конспект   | в работе |`,
  },
  {
    title: "Горизонтальная линия",
    source: `Текст до линии

---

Текст после линии`,
    example: `Текст до линии

---

Текст после линии`,
  },
];

// Модальное окно со справкой по основным возможностям Markdown.
function MarkdownHelp({ onClose }) {
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      onClick={onClose}
    >
      <Card
        className="w-full max-w-xl max-h-[85vh] flex flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <span className="flex size-8 items-center justify-center rounded-lg bg-primary text-primary-foreground">
              <HelpCircle className="size-4" />
            </span>
            Справка по Markdown
          </CardTitle>
          <CardDescription>
            Сообщение поддерживает Markdown-разметку: заголовки, списки, ссылки,
            таблицы и другое.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex-1 overflow-y-auto">
          <div className="space-y-4">
            {HELP_ITEMS.map((item) => (
              <div key={item.title} className="space-y-1.5">
                <p className="text-sm font-medium">{item.title}</p>
                <pre className="overflow-x-auto rounded-md bg-muted/60 p-3 text-xs leading-relaxed">
                  {item.source}
                </pre>
                <MarkdownView>{item.example}</MarkdownView>
              </div>
            ))}
          </div>
        </CardContent>
        <div className="flex justify-end border-t p-4">
          <Button onClick={onClose}>
            <X />
            Закрыть
          </Button>
        </div>
      </Card>
    </div>
  );
}

export default MarkdownHelp;
