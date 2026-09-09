import { useEffect, useRef, useState } from "react";
import { useImportant, saveImportantMessage } from "./api.js";
import { useQueryClient } from "@tanstack/react-query";
import DateDisplay from "@/components/DateDisplay.jsx";
import MarkdownView from "./MarkdownView.jsx";
import MarkdownHelp from "./MarkdownHelp.jsx";

import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardDescription,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Megaphone,
  Save,
  Edit,
  X,
  Loader2,
  AlertCircle,
  HelpCircle,
  Eye,
  PenLine,
} from "lucide-react";

// Строка-обёртка над обычным textarea (в стилистике shadcn/ui). Высота
// подстраивается под текст: чуть больше контента (запас ~одна строка).
function Textarea({ className, ...props }) {
  const ref = useRef(null);
  const value = props.value ?? "";

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.style.height = "auto";
    // Запас 20px — поле немного выше самого текста.
    el.style.height = el.scrollHeight + 20 + "px";
  }, [value]);

  return (
    <textarea
      ref={ref}
      data-slot="textarea"
      className={
        "w-full min-h-28 overflow-hidden rounded-lg border border-input bg-transparent px-3 py-2 text-sm leading-relaxed text-foreground transition-colors outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-input/30 " +
        (className || "")
      }
      {...props}
    />
  );
}

// Переключатель «Редактирование / Превью» + кнопка справки по Markdown.
function EditorToolbar({ tab, onTabChange, onHelp }) {
  const tabClass = (active) =>
    "rounded-md px-3 py-1.5 text-sm font-medium transition-colors " +
    (active
      ? "bg-background text-foreground shadow-sm"
      : "text-muted-foreground hover:text-foreground");

  return (
    <div className="flex flex-wrap items-center justify-between gap-2">
      <div className="flex items-center gap-1 rounded-lg border bg-muted/40 p-0.5">
        <button
          type="button"
          className={tabClass(tab === "edit")}
          onClick={() => onTabChange("edit")}
        >
          <span className="flex items-center gap-1.5">
            <PenLine className="size-3.5" />
            Редактирование
          </span>
        </button>
        <button
          type="button"
          className={tabClass(tab === "preview")}
          onClick={() => onTabChange("preview")}
        >
          <span className="flex items-center gap-1.5">
            <Eye className="size-3.5" />
            Превью
          </span>
        </button>
      </div>
      <Button variant="ghost" size="sm" onClick={onHelp}>
        <HelpCircle />
        Справка по Markdown
      </Button>
    </div>
  );
}

// Пункт меню «Важное»: личное сообщение пользователя. Показывается ему
// модальным окном раз в сутки перед входом в основной функционал —
// и только на production. Каждый пользователь управляет своим сообщением.
function Important() {
  const queryClient = useQueryClient();
  const importantQuery = useImportant(true);

  const [draft, setDraft] = useState("");
  const [tab, setTab] = useState("edit");
  const [helpOpen, setHelpOpen] = useState(false);
  const [editing, setEditing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: ["important"] });

  const data = importantQuery.data;
  const hasMessage = Boolean(data?.content && data.content.trim());

  const startEdit = () => {
    setDraft(data?.content || "");
    setTab("edit");
    setEditing(true);
    setError("");
  };

  const cancelEdit = () => {
    setEditing(false);
    setError("");
  };

  const handleSave = async () => {
    setSaving(true);
    setError("");
    try {
      await saveImportantMessage(draft);
      setEditing(false);
      refresh();
    } catch (err) {
      setError(err.message || "Не удалось сохранить сообщение");
    } finally {
      setSaving(false);
    }
  };

  return (
    <section>
      {importantQuery.isLoading ? (
        <p className="text-sm text-muted-foreground">
          <Loader2 className="mr-1 inline size-4 animate-spin" />
          Загрузка…
        </p>
      ) : importantQuery.isError ? (
        <p className="text-sm text-destructive">
          Ошибка: {importantQuery.error?.message}
        </p>
      ) : (
        <Card className="my-3" size="sm">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Megaphone className="size-4 text-muted-foreground" />
              Важное сообщение
            </CardTitle>
            <CardDescription>
              Это личное сообщение: показывается вам один раз в сутки перед
              входом в основной функционал — и только на production.
              Поддерживается разметка Markdown.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {editing ? (
              <>
                <EditorToolbar
                  tab={tab}
                  onTabChange={setTab}
                  onHelp={() => setHelpOpen(true)}
                />
                {tab === "edit" ? (
                  <Textarea
                    value={draft}
                    onChange={(e) => setDraft(e.target.value)}
                    placeholder="Текст важного сообщения… Можно использовать Markdown: **жирный**, списки, ссылки…"
                  />
                ) : draft.trim() ? (
                  <MarkdownView className="max-h-96 overflow-y-auto">
                    {draft}
                  </MarkdownView>
                ) : (
                  <p className="rounded-md border border-dashed p-4 text-center text-sm text-muted-foreground">
                    Превью пустое — напишите текст сообщения.
                  </p>
                )}
                {error && (
                  <p
                    className="flex items-center gap-1.5 text-sm text-destructive"
                    role="alert"
                  >
                    <AlertCircle className="size-4" />
                    {error}
                  </p>
                )}
                <div className="flex gap-2">
                  <Button onClick={handleSave} disabled={saving}>
                    {saving ? (
                      <Loader2 className="animate-spin" />
                    ) : (
                      <Save />
                    )}
                    {saving ? "Сохраняю…" : "Сохранить сообщение"}
                  </Button>
                  <Button variant="ghost" onClick={cancelEdit}>
                    <X />
                    Отмена
                  </Button>
                </div>
              </>
            ) : hasMessage ? (
              <>
                <MarkdownView>{data.content}</MarkdownView>
                <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                  <Badge variant="secondary">Только на production</Badge>
                  <Badge variant="secondary">Раз в сутки</Badge>
                  {data.updated_at && (
                    <span>
                      обновлено: <DateDisplay date={data.updated_at} withTime />
                      {data.updated_by ? ` · ${data.updated_by}` : ""}
                    </span>
                  )}
                </div>
                <Button variant="outline" size="sm" onClick={startEdit}>
                  <Edit />
                  Редактировать
                </Button>
              </>
            ) : (
              <>
                <p className="text-sm text-muted-foreground">
                  У вас ещё нет важного сообщения.
                </p>
                <Button variant="outline" onClick={startEdit}>
                  <Edit />
                  Создать сообщение
                </Button>
              </>
            )}
          </CardContent>
        </Card>
      )}

      {helpOpen && <MarkdownHelp onClose={() => setHelpOpen(false)} />}
    </section>
  );
}

export default Important;
