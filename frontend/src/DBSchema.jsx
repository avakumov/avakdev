import { useDBSchema } from "./api.js";
import MarkdownView from "./MarkdownView.jsx";
import DateDisplay from "@/components/DateDisplay.jsx";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardDescription,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Database, Loader2, AlertCircle, RefreshCw } from "lucide-react";

// Админ-раздел «База данных»: показывает схему (DDL), снятую на сервере
// через `pg_dump --schema-only`. Только просмотр — правок здесь нет.
function DBSchema() {
  const q = useDBSchema(true);
  const schema = q.data?.schema || "";

  return (
    <section>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="flex items-center gap-2 text-xl font-semibold">
          <Database className="size-5 text-muted-foreground" />
          База данных
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

      <Card className="my-3" size="sm">
        <CardHeader>
          <CardTitle className="text-sm">Схема</CardTitle>
          <CardDescription>
            DDL текущей базы (<code>pg_dump --schema-only</code>). Только
            просмотр.
            {q.data?.generated_at && (
              <>
                {" "}
                Снято <DateDisplay date={q.data.generated_at} withTime />.
              </>
            )}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {q.isLoading ? (
            <p className="flex items-center gap-1.5 text-sm text-muted-foreground">
              <Loader2 className="size-4 animate-spin" />
              Загружаю схему…
            </p>
          ) : q.isError ? (
            <p
              className="flex items-center gap-1.5 text-sm text-destructive"
              role="alert"
            >
              <AlertCircle className="size-4 shrink-0" />
              {q.error?.message || "Не удалось получить схему"}
            </p>
          ) : (
            // Схему показываем подсвеченным кодом: рендерим как блок ```sql
            // (у MarkdownView подключён rehype-highlight).
            <MarkdownView>{`~~~~sql\n${schema}\n~~~~`}</MarkdownView>
          )}
        </CardContent>
      </Card>
    </section>
  );
}

export default DBSchema;
