import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { cn } from "@/lib/utils";

// Отрисовка Markdown-контента в HTML. Используется вместо сырого <pre>,
// чтобы заголовки, списки, жирный текст и т.п. отображались корректно.
// bare — без собственной оболочки (подложка, отступы, text-sm): нужно там,
// где кегль и цвета задаёт родитель (например, карточка ленты с адаптивным
// кеглем). className всегда дополняет/переопределяет оформление.
function MarkdownView({ children, className, bare = false }) {
  // Стилизация каждого элемента Markdown отдельно (без tailwind-плагина typography).
  const components = {
    h1: (props) => <h1 className="text-base font-semibold" {...props} />,
    h2: (props) => <h2 className="text-base font-semibold" {...props} />,
    h3: (props) => <h3 className="text-sm font-semibold" {...props} />,
    h4: (props) => <h4 className="text-sm font-semibold" {...props} />,
    ul: (props) => <ul className="list-disc pl-5" {...props} />,
    ol: (props) => <ol className="list-decimal pl-5" {...props} />,
    li: (props) => <li className="my-0.5" {...props} />,
    p: (props) => <p className="my-2 first:mt-0 last:mb-0" {...props} />,
    a: (props) => (
      <a
        className="text-primary underline hover:text-primary/80"
        target="_blank"
        rel="noopener noreferrer"
        {...props}
      />
    ),
    code: (props) => (
      <code
        className="rounded bg-background px-1.5 py-0.5 text-[0.85em]"
        {...props}
      />
    ),
    pre: (props) => (
      <pre
        className="overflow-x-auto rounded-md bg-background p-3 my-2"
        {...props}
      />
    ),
    blockquote: (props) => (
      <blockquote
        className="my-2 border-l-2 border-border pl-3 text-muted-foreground"
        {...props}
      />
    ),
    strong: (props) => <strong className="font-semibold" {...props} />,
    table: (props) => (
      <table className="my-2 w-full border-collapse text-left" {...props} />
    ),
    th: (props) => (
      <th className="border border-border px-2 py-1 font-semibold" {...props} />
    ),
    td: (props) => <td className="border border-border px-2 py-1" {...props} />,
    input: (props) => <input disabled {...props} />,
    hr: (props) => <hr className="my-3 border-border" {...props} />,
  };

  return (
    <div
      className={cn(
        // bare — обёртку оформляет родитель (кегль и цвета задаёт карточка).
        !bare &&
          "space-y-0 rounded-md bg-muted/40 p-3 text-sm leading-relaxed text-foreground",
        className,
      )}
    >
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
        {children}
      </ReactMarkdown>
    </div>
  );
}

export default MarkdownView;
