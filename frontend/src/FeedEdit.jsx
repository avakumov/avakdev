import { useEffect, useState } from "react";
import {
  useFeedItems,
  createFeedItem,
  updateFeedItem,
  deleteFeedItem,
  generateFeedItems,
  createFeedItemsBulk,
} from "./api.js";
import { useQueryClient } from "@tanstack/react-query";
import { cn } from "@/lib/utils";
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
  Check,
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
  const [topic, setTopic] = useState(initial?.topic || "");
  const [question, setQuestion] = useState(initial?.question || "");
  const [answer, setAnswer] = useState(initial?.answer || "");
  const [details, setDetails] = useState(initial?.details || "");
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState("");

  // Раздел, вопрос и ответ обязательны; объяснение и примеры — нет.
  const ready =
    topic.trim() !== "" && question.trim() !== "" && answer.trim() !== "";

  const handleSave = async () => {
    setSaving(true);
    setError("");
    const payload = { kind, topic, question, answer, details };
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
            <Label>Раздел</Label>
            <Input
              value={topic}
              onChange={(e) => setTopic(e.target.value)}
              placeholder="golang, linux, ооп…"
            />
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

          <div className="space-y-1.5">
            <Label>Объяснение и примеры (необязательно)</Label>
            <Textarea
              value={details}
              onChange={(e) => setDetails(e.target.value)}
              placeholder={"Пояснение и примеры кода.\nВопрос, ответ и это поле — Markdown, например:\n```go\nfunc main() {}\n```"}
            />
            <p className="text-xs text-muted-foreground">
              Показывается в ленте по кнопке «подробнее…» под ответом, мелким
              шрифтом.
            </p>
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
                  <Check className="size-4" />
                  знаю:{" "}
                  <span className="tabular-nums text-foreground">
                    {initial.know_count ?? 0}
                  </span>
                </span>
                <span className="flex items-center gap-1.5">
                  <X className="size-4" />
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
          topic: it.topic || "",
          question: it.question || "",
          answer: it.answer || "",
          details: it.details || "",
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
            topic: String(d.topic || "").trim(),
            question: String(d.question || "").trim(),
            answer: String(d.answer || "").trim(),
            details: String(d.details || "").trim(),
          }))
          .filter((d) => d.topic && d.question && d.answer),
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
            удалить перед сохранением. Раздел каждого элемента ИИ подберёт сам.
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
                      {d.topic && (
                        <Badge variant="secondary" className="mb-1">
                          {d.topic}
                        </Badge>
                      )}
                      <p className="wrap-break-word text-sm font-medium text-foreground">
                        {d.question}
                      </p>
                      <p className="wrap-break-word text-xs text-muted-foreground">
                        {d.answer}
                      </p>
                      {d.details?.trim() && (
                        <p className="mt-0.5 text-xs text-muted-foreground/80">
                          + объяснение и примеры
                        </p>
                      )}
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
  // Выделение строк в таблице (desktop) для операций над пачкой.
  const [selected, setSelected] = useState(() => new Set());
  const [deleting, setDeleting] = useState(false);
  const [listError, setListError] = useState("");

  const items = feedQuery.data || [];

  const refresh = () => queryClient.invalidateQueries({ queryKey: ["feed"] });

  // После обновления списка (удаление, генерация) в выделении не должно
  // оставаться id, которых уже нет.
  useEffect(() => {
    setSelected((prev) => {
      if (prev.size === 0) return prev;
      const ids = new Set((feedQuery.data || []).map((i) => i.id));
      const next = new Set([...prev].filter((id) => ids.has(id)));
      return next.size === prev.size ? prev : next;
    });
  }, [feedQuery.data]);

  const toggleSelect = (id) =>
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });

  const clearSelection = () => setSelected(new Set());

  // Операции с выделенными. Пока одна — удалить. Массового удаления на
  // сервере нет, поэтому удаляем по одному; про неудачные сообщаем.
  const handleDeleteSelected = async () => {
    const ids = [...selected];
    if (ids.length === 0) return;
    if (!window.confirm(`Удалить выбранные элементы (${ids.length})?`)) return;
    setDeleting(true);
    setListError("");
    const failed = [];
    for (const id of ids) {
      try {
        await deleteFeedItem(id);
      } catch (err) {
        failed.push(err.message);
      }
    }
    clearSelection();
    refresh();
    setDeleting(false);
    if (failed.length > 0) {
      setListError(
        `Не удалось удалить ${failed.length} из ${ids.length}: ${failed[0]}`,
      );
    }
  };

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
        <>
          {/* Операции с выделенными строками (desktop). */}
          {selected.size > 0 && (
            <div className="my-3 flex flex-wrap items-center gap-3 border border-border/60 px-3 py-2">
              <span className="text-sm">Выбрано: {selected.size}</span>
              <Button
                variant="destructive"
                size="sm"
                onClick={handleDeleteSelected}
                disabled={deleting}
              >
                {deleting ? <Loader2 className="animate-spin" /> : <Trash2 />}
                {deleting ? "Удаляю…" : "Удалить выбранные"}
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={clearSelection}
                disabled={deleting}
              >
                <X />
                Снять выделение
              </Button>
              {listError && (
                <span className="flex items-center gap-1 text-sm text-destructive">
                  <AlertCircle className="size-3.5 shrink-0" />
                  {listError}
                </span>
              )}
            </div>
          )}

          {/* Таблица — desktop (md и шире). Клик по строке выделяет её. */}
          <Card className="my-3 hidden overflow-hidden md:block" size="sm">
            <table className="w-full table-fixed text-sm">
              <colgroup>
                <col className="w-[6%]" />
                <col className="w-[30%]" />
                <col className="w-[30%]" />
                <col className="w-[8%]" />
                <col className="w-[8%]" />
                <col className="w-[8%]" />
                <col className="w-[10%]" />
              </colgroup>
              <thead>
                <tr className="border-b bg-muted/40 text-left text-[11px] tracking-wide text-muted-foreground uppercase">
                  <th className="px-3 py-2 align-middle font-bold"> </th>
                  <th className="px-1.5 py-2 align-middle font-bold">Вопрос</th>
                  <th className="px-1.5 py-2 align-middle font-bold">Ответ</th>
                  <th className="px-1.5 py-2 align-middle font-bold">Показы</th>
                  <th className="px-1.5 py-2 align-middle font-bold">Знаю</th>
                  <th className="px-1.5 py-2 align-middle font-bold">Не знаю</th>
                  <th className="px-2 py-2 text-right align-middle font-bold">
                    
                  </th>
                </tr>
              </thead>
              <tbody>
                {items.map((item) => {
                  const isSel = selected.has(item.id);
                  return (
                    <tr
                      key={item.id}
                      onClick={() => toggleSelect(item.id)}
                      aria-selected={isSel}
                      title={isSel ? "Снять выделение" : "Выделить"}
                      className={cn(
                        "cursor-pointer border-b border-border/60 last:border-0 hover:bg-muted/30",
                        isSel && "bg-muted/60",
                      )}
                    >
                      <td className="px-3 py-2 align-top">
                        <span
                          className={cn(
                            "mt-0.5 flex size-4 items-center justify-center border transition-colors",
                            isSel
                              ? "border-primary bg-primary text-primary-foreground"
                              : "border-input bg-transparent",
                          )}
                        >
                          {isSel && <Check className="size-3" />}
                        </span>
                      </td>
                      <td className="px-1.5 py-2 align-top">
                        <p className="line-clamp-2 font-medium wrap-break-word text-foreground">
                          {item.question}
                        </p>
                        {item.topic && (
                          <Badge variant="secondary" className="mt-1">
                            {item.topic}
                          </Badge>
                        )}
                      </td>
                      <td className="px-1.5 py-2 align-top">
                        <p
                          className="line-clamp-2 wrap-break-word text-muted-foreground"
                          title={item.answer}
                        >
                          {item.answer}
                        </p>
                        {item.details?.trim() && (
                          <span className="text-xs text-muted-foreground">
                            · есть объяснение
                          </span>
                        )}
                      </td>
                      <td className="px-1.5 py-2 align-top tabular-nums">
                        {item.views ?? 0}
                      </td>
                      <td className="px-1.5 py-2 align-top tabular-nums text-emerald-600 dark:text-emerald-400">
                        {item.know_count ?? 0}
                      </td>
                      <td className="px-1.5 py-2 align-top tabular-nums text-destructive">
                        {item.unknown_count ?? 0}
                      </td>
                      <td className="px-2 py-2 text-right align-top">
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          onClick={(e) => {
                            e.stopPropagation();
                            setModal({ item });
                          }}
                          title="Редактировать элемент"
                          aria-label="Редактировать элемент"
                        >
                          <Pencil />
                        </Button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </Card>

          {/* Карточки — mobile (< md) */}
          <div className="md:hidden">
            {items.map((item) => (
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
                      {item.topic && (
                        <Badge variant="secondary">{item.topic}</Badge>
                      )}
                      <Badge variant="outline">{kindLabel(item.kind)}</Badge>
                      <span className="flex items-center gap-1 text-xs text-muted-foreground">
                        <Eye className="size-3" />
                        показов:{" "}
                        <span className="tabular-nums">{item.views ?? 0}</span>
                      </span>
                      <span className="flex items-center gap-1 text-xs text-muted-foreground">
                        <Check className="size-3" />
                        знаю:{" "}
                        <span className="tabular-nums">{item.know_count ?? 0}</span>
                      </span>
                      <span className="flex items-center gap-1 text-xs text-muted-foreground">
                        <X className="size-3" />
                        не знаю:{" "}
                        <span className="tabular-nums">
                          {item.unknown_count ?? 0}
                        </span>
                      </span>
                      {item.details?.trim() && (
                        <span className="text-xs text-muted-foreground">
                          · есть объяснение
                        </span>
                      )}
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
            ))}
          </div>
        </>
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
