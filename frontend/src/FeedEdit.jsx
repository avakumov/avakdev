import { useState } from "react";
import {
  useFeedItems,
  createFeedItem,
  updateFeedItem,
  deleteFeedItem,
  generateFeedItems,
  createFeedItemsBulk,
} from "./api.js";
import { useQueryClient } from "@tanstack/react-query";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardDescription,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Rss,
  Plus,
  Pencil,
  Loader2,
  AlertCircle,
  X,
  Save,
  Trash2,
  Eye,
  Sparkles,
  ThumbsUp,
  ThumbsDown,
} from "lucide-react";

// Типы контента ленты. Пока один — «вопрос-ответ» (размеры полей проверяет
// сервер).
const KINDS = [{ value: "qa", label: "Вопрос-ответ" }];

const kindLabel = (kind) => KINDS.find((x) => x.value === kind)?.label || kind;

// Строка-обёртка над обычным textarea (в стилистике shadcn/ui).
function Textarea({ className, ...props }) {
  return (
    <textarea
      data-slot="textarea"
      className={
        "w-full min-h-20 rounded-lg border border-input bg-transparent px-3 py-2 text-sm leading-relaxed text-foreground transition-colors outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-input/30 resize-y " +
        (className || "")
      }
      {...props}
    />
  );
}

// Модалка создания/редактирования элемента ленты. Удаление — здесь же
// (отдельной кнопки в списке нет).
function FeedItemModal({ initial, onClose, onSaved }) {
  const isCreate = !initial;
  const [kind, setKind] = useState(initial?.kind || "qa");
  const [question, setQuestion] = useState(initial?.question || "");
  const [answer, setAnswer] = useState(initial?.answer || "");
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState("");

  const ready = question.trim() !== "" && answer.trim() !== "";

  const handleSave = async () => {
    setSaving(true);
    setError("");
    const payload = { kind, question, answer };
    try {
      if (initial) await updateFeedItem(initial.id, payload);
      else await createFeedItem(payload);
      onSaved();
      onClose();
    } catch (err) {
      setError(err.message || "Не удалось сохранить элемент ленты");
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (
      !window.confirm("Удалить элемент ленты? Счётчик показов тоже пропадёт.")
    ) {
      return;
    }
    setDeleting(true);
    setError("");
    try {
      await deleteFeedItem(initial.id);
      onSaved();
      onClose();
    } catch (err) {
      setError(err.message || "Не удалось удалить элемент ленты");
      setDeleting(false);
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      onClick={onClose}
    >
      <Card
        className="flex max-h-[85vh] w-full max-w-lg flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Rss className="size-4 text-muted-foreground" />
            {isCreate ? "Новый элемент ленты" : "Редактировать элемент ленты"}
          </CardTitle>
          <CardDescription>
            Показывается в ленте: сначала вопрос, ответ — после касания.
          </CardDescription>
        </CardHeader>

        <CardContent className="flex-1 space-y-4 overflow-y-auto">
          <div className="space-y-1.5">
            <Label>Тип контента</Label>
            <Select value={kind} onValueChange={setKind}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {KINDS.map((k) => (
                  <SelectItem key={k.value} value={k.value}>
                    {k.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-1.5">
            <Label>Вопрос</Label>
            <Textarea
              value={question}
              onChange={(e) => setQuestion(e.target.value)}
              placeholder="Например: что такое замыкание?"
            />
          </div>

          <div className="space-y-1.5">
            <Label>Ответ</Label>
            <Textarea
              value={answer}
              onChange={(e) => setAnswer(e.target.value)}
              placeholder="Ответ, который откроется после касания…"
            />
          </div>

          {!isCreate && (
            <div className="space-y-1 text-sm text-muted-foreground">
              <p className="flex items-center gap-1.5">
                <Eye className="size-4" />
                Показов:{" "}
                <span className="tabular-nums text-foreground">
                  {initial.views ?? 0}
                </span>
              </p>
              <p className="flex flex-wrap items-center gap-x-3 gap-y-1">
                <span className="flex items-center gap-1.5">
                  <ThumbsUp className="size-4" />
                  знаю:{" "}
                  <span className="tabular-nums text-foreground">
                    {initial.know_count ?? 0}
                  </span>
                </span>
                <span className="flex items-center gap-1.5">
                  <ThumbsDown className="size-4" />
                  не знаю:{" "}
                  <span className="tabular-nums text-foreground">
                    {initial.unknown_count ?? 0}
                  </span>
                </span>
              </p>
            </div>
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
        </CardContent>

        <div className="flex items-center justify-between gap-2 border-t p-4">
          {isCreate ? (
            <span />
          ) : (
            <Button
              variant="destructive"
              onClick={handleDelete}
              disabled={saving || deleting}
            >
              {deleting ? <Loader2 className="animate-spin" /> : <Trash2 />}
              {deleting ? "Удаляю…" : "Удалить"}
            </Button>
          )}
          <div className="flex gap-2">
            <Button variant="ghost" onClick={onClose} disabled={saving || deleting}>
              <X />
              Отмена
            </Button>
            <Button
              onClick={handleSave}
              disabled={saving || deleting || !ready}
            >
              {saving ? <Loader2 className="animate-spin" /> : <Save />}
              {saving ? "Сохраняю…" : "Сохранить"}
            </Button>
          </div>
        </div>
      </Card>
    </div>
  );
}

// Модалка ИИ-генерации элементов ленты: тема, описание и количество.
// Модель предлагает черновики (в БД они ещё не записаны), пользователь убирает
// лишние и сохраняет оставшиеся — как черновики задач цели.
function FeedGenerateModal({ onClose, onSaved }) {
  const [topic, setTopic] = useState("");
  const [description, setDescription] = useState("");
  const [count, setCount] = useState("5");
  const [drafts, setDrafts] = useState([]);
  const [truncated, setTruncated] = useState(false); // ответ модели обрезан
  const [generating, setGenerating] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const busy = generating || saving;

  const removeDraft = (index) =>
    setDrafts((prev) => prev.filter((_, i) => i !== index));

  const handleGenerate = async () => {
    if (!topic.trim()) return;
    if (
      drafts.length > 0 &&
      !window.confirm("Заменить текущий список элементов новым?")
    ) {
      return;
    }
    setGenerating(true);
    setError("");
    setTruncated(false);
    try {
      const data = await generateFeedItems({
        topic,
        description,
        count: Number(count) || 0,
      });
      setDrafts(
        (data.items || []).map((it) => ({
          question: it.question || "",
          answer: it.answer || "",
        })),
      );
      // Сервер сообщает, если ответ модели обрезан по лимиту длины.
      setTruncated(Boolean(data.truncated));
    } catch (err) {
      setError(err.message || "Не удалось сгенерировать элементы");
    } finally {
      setGenerating(false);
    }
  };

  const handleSave = async () => {
    setSaving(true);
    setError("");
    try {
      await createFeedItemsBulk(
        drafts
          .map((d) => ({
            question: String(d.question || "").trim(),
            answer: String(d.answer || "").trim(),
          }))
          .filter((d) => d.question && d.answer),
      );
      onSaved();
      onClose();
    } catch (err) {
      setError(err.message || "Не удалось сохранить элементы ленты");
      setSaving(false);
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      onClick={onClose}
    >
      <Card
        className="flex max-h-[85vh] w-full max-w-lg flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Sparkles className="size-4 text-muted-foreground" />
            Элементы ленты с ИИ
          </CardTitle>
          <CardDescription>
            Укажите тему, описание и количество — созданные элементы можно
            удалить перед сохранением.
          </CardDescription>
        </CardHeader>

        <CardContent className="flex-1 space-y-4 overflow-y-auto">
          <div className="space-y-1.5">
            <Label>Тема</Label>
            <Input
              value={topic}
              onChange={(e) => setTopic(e.target.value)}
              placeholder="Например: JavaScript для собеседований"
            />
          </div>

          <div className="space-y-1.5">
            <Label>Описание (необязательно)</Label>
            <Textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Что важно затронуть, уровень сложности, уклон…"
            />
          </div>

          <div className="flex flex-wrap items-end gap-3">
            <div className="space-y-1.5">
              <Label>Сколько элементов</Label>
              <Input
                type="number"
                min="1"
                max="50"
                value={count}
                onChange={(e) => setCount(e.target.value)}
                className="w-32"
              />
            </div>
            <Button
              variant="outline"
              onClick={handleGenerate}
              disabled={busy || !topic.trim()}
            >
              {generating ? <Loader2 className="animate-spin" /> : <Sparkles />}
              {generating ? "Генерирую…" : "Создать с ИИ"}
            </Button>
          </div>

          {truncated && (
            <p className="flex items-center gap-1.5 text-xs text-amber-600 dark:text-amber-400">
              <AlertCircle className="size-3.5" />
              Ответ модели обрезан — элементов может быть меньше, чем нужно.
              Нажмите «Создать с ИИ» ещё раз.
            </p>
          )}

          {drafts.length === 0 ? (
            <p className="text-xs text-muted-foreground">
              Элементы ещё не сгенерированы — заполните поля и нажмите кнопку
              выше.
            </p>
          ) : (
            <div className="space-y-2">
              <p className="text-xs text-muted-foreground">
                Получено {drafts.length}. Уберите лишние и сохраните — в базе их
                пока нет.
              </p>
              <ul className="space-y-1.5">
                {drafts.map((d, i) => (
                  <li
                    key={i}
                    className="flex items-start justify-between gap-2 border border-border/60 px-2 py-1.5"
                  >
                    <div className="min-w-0">
                      <p className="wrap-break-word text-sm font-medium text-foreground">
                        {d.question}
                      </p>
                      <p className="wrap-break-word text-xs text-muted-foreground">
                        {d.answer}
                      </p>
                    </div>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon-sm"
                      className="-mt-1 -mr-1 shrink-0 text-muted-foreground hover:text-destructive"
                      onClick={() => removeDraft(i)}
                      disabled={busy}
                      aria-label="Удалить элемент"
                    >
                      <X className="size-4" />
                    </Button>
                  </li>
                ))}
              </ul>
            </div>
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
        </CardContent>

        <div className="flex justify-end gap-2 border-t p-4">
          <Button variant="ghost" onClick={onClose} disabled={busy}>
            <X />
            Отмена
          </Button>
          <Button
            onClick={handleSave}
            disabled={busy || drafts.length === 0}
          >
            {saving ? <Loader2 className="animate-spin" /> : <Save />}
            {saving ? "Сохраняю…" : `Сохранить (${drafts.length})`}
          </Button>
        </div>
      </Card>
    </div>
  );
}

// Раздел «Лента» (пункт меню) — редактирование ленты. Это отдельный экран от
// ленты для просмотра, которая открывается свайпом влево на мобильных
// (см. Feed.jsx: он в меню не входит).
function FeedEdit() {
  const queryClient = useQueryClient();
  const feedQuery = useFeedItems(true);
  const [modal, setModal] = useState(null); // null | { item: null } | { item }
  const [generateOpen, setGenerateOpen] = useState(false);

  const items = feedQuery.data || [];

  const refresh = () => queryClient.invalidateQueries({ queryKey: ["feed"] });

  return (
    <section>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="flex items-center gap-2 text-xl font-semibold">
          <Rss className="size-5 text-muted-foreground" />
          Лента
        </h2>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" onClick={() => setGenerateOpen(true)}>
            <Sparkles />
            Создать с ИИ
          </Button>
          <Button onClick={() => setModal({ item: null })}>
            <Plus />
            Добавить элемент
          </Button>
        </div>
      </div>

      <p className="my-3 text-sm text-muted-foreground">
        Наполнение ленты, которая открывается свайпом влево на мобильных.
      </p>

      {feedQuery.isLoading ? (
        <p className="text-sm text-muted-foreground">
          <Loader2 className="mr-1 inline size-4 animate-spin" />
          Загрузка ленты…
        </p>
      ) : items.length === 0 ? (
        <Card className="my-3" size="sm">
          <CardContent>
            <p className="flex items-center gap-2 text-sm text-muted-foreground">
              <Rss className="size-4" />
              Пока в ленте пусто. Добавьте первый элемент.
            </p>
          </CardContent>
        </Card>
      ) : (
        items.map((item) => (
          <Card
            key={item.id}
            className="my-3 cursor-pointer"
            size="sm"
            onClick={() => setModal({ item })}
            title="Редактировать элемент"
          >
            <CardContent className="flex items-start gap-2">
              <div className="min-w-0 flex-1 space-y-1">
                <div className="flex flex-wrap items-center gap-1.5">
                  <Badge variant="secondary">{kindLabel(item.kind)}</Badge>
                  <span className="flex items-center gap-1 text-xs text-muted-foreground">
                    <Eye className="size-3" />
                    показов:{" "}
                    <span className="tabular-nums">{item.views ?? 0}</span>
                  </span>
                  <span className="flex items-center gap-1 text-xs text-muted-foreground">
                    <ThumbsUp className="size-3" />
                    знаю:{" "}
                    <span className="tabular-nums">{item.know_count ?? 0}</span>
                  </span>
                  <span className="flex items-center gap-1 text-xs text-muted-foreground">
                    <ThumbsDown className="size-3" />
                    не знаю:{" "}
                    <span className="tabular-nums">
                      {item.unknown_count ?? 0}
                    </span>
                  </span>
                </div>
                <p className="wrap-break-word text-sm font-medium text-foreground">
                  {item.question}
                </p>
                <p className="wrap-break-word text-sm whitespace-pre-wrap text-muted-foreground">
                  {item.answer}
                </p>
              </div>
              <Button
                variant="ghost"
                size="icon"
                className="shrink-0"
                onClick={(e) => {
                  e.stopPropagation();
                  setModal({ item });
                }}
                title="Редактировать элемент"
                aria-label="Редактировать элемент"
              >
                <Pencil />
              </Button>
            </CardContent>
          </Card>
        ))
      )}

      {modal && (
        <FeedItemModal
          initial={modal.item}
          onClose={() => setModal(null)}
          onSaved={refresh}
        />
      )}

      {generateOpen && (
        <FeedGenerateModal
          onClose={() => setGenerateOpen(false)}
          onSaved={refresh}
        />
      )}
    </section>
  );
}

export default FeedEdit;
