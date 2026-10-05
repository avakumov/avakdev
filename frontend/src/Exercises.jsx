import { useState } from "react";
import { useExercises } from "./api.js";
import DateDisplay from "@/components/DateDisplay.jsx";
import {
  Card,
  CardHeader,
  CardTitle,
  CardAction,
  CardContent,
  CardDescription,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import {
  GraduationCap,
  RefreshCw,
  Loader2,
  AlertCircle,
  CheckCircle2,
  Circle,
  ChevronDown,
  ChevronRight,
} from "lucide-react";

// Сколько символов вывода теста показываем, прежде чем обрезать.
const OUTPUT_PREVIEW = 1500;

// Зелёный бейдж «всё решено».
const doneBadge = "bg-emerald-600 text-white dark:bg-emerald-500";

// Карточка задания: что нужно сделать и прошло ли оно.
function TaskCard({ task }) {
  const output = (task.output || "").trim();
  const hasOutput = !task.passed && output;

  return (
    <Card
      size="sm"
      className={cn(task.passed && "ring-emerald-500/40 bg-emerald-500/5")}
    >
      <CardHeader>
        <CardTitle className="flex items-center gap-2 pr-2">
          {task.passed ? (
            <CheckCircle2 className="size-4 shrink-0 text-emerald-600 dark:text-emerald-400" />
          ) : (
            <Circle className="size-4 shrink-0 text-muted-foreground" />
          )}
          {task.title}
        </CardTitle>
        <CardAction>
          {task.passed ? (
            <Badge variant="secondary" className={doneBadge}>
              решено
            </Badge>
          ) : (
            <Badge variant="secondary">не решено</Badge>
          )}
        </CardAction>
        {task.description && <CardDescription>{task.description}</CardDescription>}
      </CardHeader>
      <CardContent className="space-y-2">
        {task.file && (
          <code className="text-xs text-muted-foreground">{task.file}</code>
        )}
        {task.error && (
          <pre className="overflow-x-auto rounded-md bg-destructive/10 p-2 text-xs whitespace-pre-wrap text-destructive">
            {task.error.slice(0, OUTPUT_PREVIEW)}
            {task.error.length > OUTPUT_PREVIEW && "\n… (вывод обрезан)"}
          </pre>
        )}
        {hasOutput && (
          <details className="text-sm">
            <summary className="cursor-pointer text-muted-foreground select-none">
              Вывод теста
            </summary>
            <pre className="mt-2 overflow-x-auto rounded-md bg-muted p-2 text-xs whitespace-pre-wrap">
              {output.slice(0, OUTPUT_PREVIEW)}
              {output.length > OUTPUT_PREVIEW && "\n… (вывод обрезан)"}
            </pre>
          </details>
        )}
      </CardContent>
    </Card>
  );
}

// Узел-тема: сворачиваемый заголовок + вложенные темы/задания.
// По умолчанию темы свёрнуты — дерево раскрывается по клику.
function TopicNode({ node }) {
  const [open, setOpen] = useState(false);
  const Chevron = open ? ChevronDown : ChevronRight;
  return (
    <div>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center gap-2 text-left"
        aria-expanded={open}
      >
        <Chevron className="size-4 shrink-0 text-muted-foreground" />
        <span className="text-base font-medium">{node.title}</span>
        {node.total_count > 0 && (
          <Badge variant="secondary" className={cn(node.passed && doneBadge)}>
            {node.passed_count} из {node.total_count}
          </Badge>
        )}
      </button>
      {open &&
        (node.children?.length ? (
          <div className="mt-2 space-y-3 border-l pl-4">
            {node.children.map((child) =>
              child.kind === "topic" ? (
                <TopicNode key={child.key} node={child} />
              ) : (
                <TaskCard key={child.key} task={child} />
              ),
            )}
          </div>
        ) : (
          <p className="mt-2 pl-6 text-sm text-muted-foreground">
            Пока нет заданий
          </p>
        ))}
    </div>
  );
}

// Раздел «Практика»: дерево тем и заданий. Код пишется локально, приложение
// лишь показывает результаты последнего прогона (`make exercises`).
function Exercises() {
  const q = useExercises(true);
  const nodes = q.data?.nodes || [];
  const available = q.data?.available !== false;

  return (
    <section>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="flex items-center gap-2 text-xl font-semibold">
          <GraduationCap className="size-5 text-muted-foreground" />
          Практика
        </h2>
        <Button
          variant="outline"
          onClick={() => q.refetch()}
          disabled={q.isFetching}
        >
          <RefreshCw className={q.isFetching ? "animate-spin" : ""} />
          {q.isFetching ? "Обновляю…" : "Обновить"}
        </Button>
      </div>

      <p className="mt-2 text-sm text-muted-foreground">
        Прогоняйте тесты как обычно — статус обновится сам:{" "}
        <code className="text-xs">go test ./...</code> в папке{" "}
        <code className="text-xs">exercises</code> или кнопка теста в
        редакторе.
        {q.data?.generated_at && (
          <>
            {" "}
            Последний прогон:{" "}
            <DateDisplay date={q.data.generated_at} withTime />.
          </>
        )}
      </p>

      {q.isLoading ? (
        <p className="mt-4 flex items-center gap-1.5 text-sm text-muted-foreground">
          <Loader2 className="size-4 animate-spin" />
          Загружаю задания…
        </p>
      ) : q.isError ? (
        <p
          className="mt-4 flex items-center gap-1.5 text-sm text-destructive"
          role="alert"
        >
          <AlertCircle className="size-4 shrink-0" />
          {q.error?.message || "Не удалось загрузить задания"}
        </p>
      ) : !available ? (
        <p className="mt-4 text-sm text-muted-foreground">
          Каталог заданий недоступен. Раздел работает в локальной версии, где
          рядом с сервером есть папка <code>exercises/</code>.
        </p>
      ) : nodes.length === 0 ? (
        <p className="mt-4 text-sm text-muted-foreground">
          Задания пока не добавлены.
        </p>
      ) : (
        <div className="mt-4 space-y-4">
          {nodes.map((node) =>
            node.kind === "topic" ? (
              <TopicNode key={node.key} node={node} />
            ) : (
              <TaskCard key={node.key} task={node} />
            ),
          )}
        </div>
      )}
    </section>
  );
}

export default Exercises;
