import { useEffect, useMemo, useState } from "react";
import {
  fetchDayPlan,
  suggestDay,
  saveDay,
  useUserMetrics,
  setUserMetricValue,
  useTasks,
  useKnowledge,
  repeatKnowledge,
  setDayItemDone,
  useReports,
  updateReport,
  useLastBookmark,
  useReadingTime,
  useBooks,
  setReadingGoal,
} from "./api.js";
import { useQueryClient } from "@tanstack/react-query";
import { useAppStore } from "./store.js";
import { todayStr, formatClock } from "@/lib/formatDate.js";
import DateDisplay from "@/components/DateDisplay.jsx";
import MarkdownView from "./MarkdownView.jsx";
import { TaskFormModal } from "./Tasks.jsx";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardDescription,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import {
  CalendarDays,
  Clock,
  FileText,
  Loader2,
  AlertCircle,
  ListTodo,
  BookOpen,
  BarChart3,
  Check,
  X,
  XCircle,
  Sparkles,
  Save,
  CheckCircle2,
  Repeat,
  Pencil,
} from "lucide-react";
import { cn } from "@/lib/utils";

const fmtMin = (m) => {
  if (!m) return "0 мин";
  if (m < 60) return `${m} мин`;
  const h = Math.floor(m / 60);
  const r = m % 60;
  return r ? `${h} ч ${r} мин` : `${h} ч`;
};

// Минуты задачи из запланированных часов (как на сервере, минимум 1).
const taskMinutesFromHours = (hours) =>
  Math.max(1, Math.round((Number(hours) || 0) * 60));

// Подпись «категория · X ч» для строки задачи.
const taskMetaLabel = (category, hours) => {
  const h = Number(hours) || 0;
  return `${category || "Прочее"} · ${String(h)} ч`;
};

// Строка-кандидат: чекбокс «включить в день» + клик по строке открывает
// модалку (задача — редактирование, конспект — чтение). done — позиция
// выполнена (задача «Готова» / конспект «Повторено») — подсвечивается зелёным.
function CandidateRow({ c, onToggle, onOpen, done }) {
  const checked = c.selected;
  const doneLabel = c.kind === "task" ? "Готова" : "Повторено";
  return (
    <div
      className={cn(
        "flex w-full items-start gap-2.5 rounded-lg border px-3 py-2 transition-colors",
        done
          ? "border-emerald-500/50 bg-emerald-500/10"
          : "border-border/60 hover:bg-muted/40",
      )}
    >
      <button
        type="button"
        onClick={() => onToggle(c)}
        aria-label={checked ? "Убрать из дня" : "Добавить в день"}
        className={cn(
          "mt-0.5 flex size-4 shrink-0 cursor-pointer items-center justify-center rounded border transition-colors",
          checked
            ? "border-primary bg-primary text-primary-foreground"
            : "border-input bg-transparent",
        )}
      >
        {checked && <Check className="size-3" />}
      </button>
      <button
        type="button"
        onClick={() => onOpen(c)}
        className="min-w-0 flex-1 cursor-pointer text-left"
        title={done ? "Открыть" : "Открыть"}
      >
        <span className="wrap-break-word block text-sm font-medium text-foreground">
          {c.title}
        </span>
        {c.meta && (
          <span className="block text-xs text-muted-foreground">{c.meta}</span>
        )}
        {done && (
          <span className="mt-0.5 flex items-center gap-1 text-xs font-medium text-emerald-600 dark:text-emerald-400">
            <CheckCircle2 className="size-3.5" />
            {doneLabel}
          </span>
        )}
      </button>
      <span className="shrink-0 text-xs font-medium tabular-nums text-muted-foreground">
        {fmtMin(c.minutes)}
      </span>
    </div>
  );
}

// Модалка чтения конспекта из дня: контент + кнопка «Я повторил».
function NoteReadModal({ note, done, onClose, onRepeat }) {
  const [busy, setBusy] = useState(false);

  const handleRepeat = async () => {
    if (done || busy) return;
    setBusy(true);
    try {
      await onRepeat();
    } finally {
      setBusy(false);
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      onClick={onClose}
    >
      <Card
        className="flex max-h-[85vh] w-full max-w-2xl flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        <CardHeader>
          <CardTitle className="flex flex-wrap items-center gap-2">
            <BookOpen className="size-4 shrink-0 text-muted-foreground" />
            <span className="min-w-0 flex-1">{note.title}</span>
            {done && (
              <Badge
                variant="secondary"
                className="gap-1 text-emerald-600 dark:text-emerald-400"
              >
                <CheckCircle2 className="size-3.5" />
                Повторено
              </Badge>
            )}
          </CardTitle>
          <CardDescription>
            {note.topic}
            {note.reading_minutes
              ? ` · чтение ${fmtMin(note.reading_minutes)}`
              : ""}
          </CardDescription>
        </CardHeader>
        <CardContent className="flex-1 overflow-y-auto">
          <MarkdownView>{note.content}</MarkdownView>
        </CardContent>
        <div className="flex justify-end gap-2 border-t p-4">
          <Button
            variant="outline"
            onClick={handleRepeat}
            disabled={busy || done}
          >
            {busy ? (
              <Loader2 className="animate-spin" />
            ) : done ? (
              <CheckCircle2 />
            ) : (
              <Repeat />
            )}
            {busy ? "Отмечаю…" : done ? "Повторено сегодня" : "Я повторил"}
          </Button>
          <Button variant="ghost" onClick={onClose} disabled={busy}>
            <X />
            Закрыть
          </Button>
        </div>
      </Card>
    </div>
  );
}

// Отчёт за текущий день (как в разделе «Отчёты», но без выбора даты):
// показывается в конце дня; если отчёт на дату уже есть — его можно
// перезаписать.
function DayReportCard({ date }) {
  const queryClient = useQueryClient();
  const reportsQuery = useReports(true);
  const [draft, setDraft] = useState("");
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState("");

  const todayReport = reportsQuery.data?.find((r) => r.date === date);

  // Подставляем уже сохранённый отчёт за эту дату, если он есть.
  useEffect(() => {
    setDraft(todayReport?.content ?? "");
  }, [todayReport?.content]);

  const handleSave = async () => {
    setSaving(true);
    setError("");
    setSaved(false);
    try {
      await updateReport(date, draft);
      setSaved(true);
      queryClient.invalidateQueries({ queryKey: ["reports"] });
    } catch (err) {
      setError(err.message || "Не удалось сохранить отчёт");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card className="my-3" size="sm">
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-sm">
          <FileText className="size-4 text-muted-foreground" />
          Отчёт за сегодня · <DateDisplay date={date} />
        </CardTitle>
        <CardDescription>
          Что произошло за этот день — сохранится в разделе «Отчёты».
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-2">
        <textarea
          data-slot="textarea"
          value={draft}
          onChange={(e) => {
            setDraft(e.target.value);
            setSaved(false);
          }}
          placeholder="Задачи выполнены, метрики зафиксированы, итоги дня…"
          className="w-full min-h-24 rounded-lg border border-input bg-transparent px-3 py-2 text-sm leading-relaxed text-foreground transition-colors outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-input/30 resize-y"
        />
        {todayReport && todayReport.content !== "" && (
          <p className="flex items-center gap-1.5 text-sm text-amber-600">
            <AlertCircle className="size-3.5" />
            На эту дату уже есть отчёт — сохранение перезапишет его.
          </p>
        )}
        {error && (
          <p
            className="flex items-center gap-1.5 text-sm text-destructive"
            role="alert"
          >
            <AlertCircle className="size-3.5" />
            {error}
          </p>
        )}
        {saved && (
          <p className="flex items-center gap-1.5 text-sm text-emerald-600 dark:text-emerald-400">
            <Check className="size-3.5" />
            Отчёт сохранён
          </p>
        )}
        <div className="flex flex-wrap items-center gap-2">
          <Button
            onClick={handleSave}
            disabled={saving || draft.trim() === ""}
          >
            {saving ? <Loader2 className="animate-spin" /> : <Save />}
            {saving
              ? "Сохраняю…"
              : todayReport && todayReport.content !== ""
                ? "Заменить отчёт"
                : "Сохранить отчёт"}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

// Быстрый ввод метрик на день (время не считается).
function MetricsBlock({ date }) {
  const queryClient = useQueryClient();
  const metricsQuery = useUserMetrics();
  const [busyId, setBusyId] = useState(null);

  const defs = metricsQuery.data?.definitions || [];
  const values = metricsQuery.data?.values || [];
  const valueFor = (id) =>
    values.find((v) => v.metric_id === id && v.date === date)?.value;

  const setValue = async (id, value) => {
    setBusyId(id);
    try {
      await setUserMetricValue(id, date, value);
      queryClient.invalidateQueries({ queryKey: ["user-metrics"] });
    } catch (err) {
      window.alert(err.message || "Не удалось сохранить метрику");
    } finally {
      setBusyId(null);
    }
  };

  if (metricsQuery.isLoading) {
    return (
      <p className="text-sm text-muted-foreground">
        <Loader2 className="mr-1 inline size-4 animate-spin" />
        Метрики…
      </p>
    );
  }

  return (
    <div className="space-y-1.5">
      {defs.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          Пока нет ни одной метрики — добавьте их в разделе «Метрики».
        </p>
      ) : (
        defs.map((d) => {
          const val = valueFor(d.id);
          return (
            <MetricRow
              key={d.id}
              def={d}
              value={val}
              hasValue={val != null && val !== ""}
              busy={busyId === d.id}
              onSet={(v) => setValue(d.id, v)}
            />
          );
        })
      )}
    </div>
  );
}

function MetricRow({ def, value, busy, onSet, hasValue }) {
  const unit = def.unit ? ` ${def.unit}` : "";
  // Метрика, зафиксированная за этот день, подсвечивается зелёным.
  const rowCls = cn(
    "flex items-center justify-between gap-2 rounded-lg border px-3 py-2 transition-colors",
    hasValue
      ? "border-emerald-500/50 bg-emerald-500/10"
      : "border-border/60",
  );

  // Черновик числового значения: сохраняется автоматически (см. commit).
  const [draft, setDraft] = useState(value ?? "");
  useEffect(() => {
    setDraft(value ?? "");
  }, [value]);

  if (def.type === "bool") {
    const v = value === "true" ? true : value === "false" ? false : null;
    const yesActive = v === true;
    const noActive = v === false;
    return (
      <div className={rowCls}>
        <span className="min-w-0 text-sm font-medium">{def.name}</span>
        <div className="flex shrink-0 items-center gap-1">
          <Button
            size="sm"
            variant="outline"
            disabled={busy}
            onClick={() => onSet(yesActive ? "false" : "true")}
            className={
              yesActive
                ? "border-emerald-600 bg-emerald-600 text-white hover:bg-emerald-700"
                : "text-muted-foreground hover:text-foreground"
            }
          >
            {yesActive && <CheckCircle2 />}
            Да
          </Button>
          <Button
            size="sm"
            variant="outline"
            disabled={busy}
            onClick={() => onSet(noActive ? "true" : "false")}
            className={
              noActive
                ? "border-destructive bg-destructive text-white hover:bg-destructive/90"
                : "text-muted-foreground hover:text-foreground"
            }
          >
            {noActive && <XCircle />}
            Нет
          </Button>
        </div>
      </div>
    );
  }

  // Числовая метрика: кнопки сохранения нет — значение сохраняется по Enter
  // или при уходе с поля (если введено число).
  const commit = () => {
    if (draft.trim() !== "") {
      onSet(draft.trim());
    } else {
      setDraft(value ?? ""); // очистили — возвращаем сохранённое значение
    }
  };

  return (
    <div className={rowCls}>
      <span className="min-w-0 text-sm font-medium">
        {def.name}
        {unit && <span className="text-muted-foreground">{unit}</span>}
      </span>
      <Input
        type="number"
        step={def.type === "float" ? "0.01" : "1"}
        value={draft}
        disabled={busy}
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") e.currentTarget.blur();
        }}
        onBlur={commit}
        className="h-8 w-28"
        placeholder="—"
      />
    </div>
  );
}

// Подпись фрагмента закладки для карточки «Чтение».
const bookmarkLabel = (s, max = 80) => {
  const t = String(s || "")
    .replace(/\s+/g, " ")
    .trim();
  return t.length > max ? `${t.slice(0, max)}…` : t;
};

// Раздел «День»: формирование плана на сегодня (задачи + повторение знаний),
// метрики без времени; внизу — прошедшие дни.
function Day({ onNavigate }) {
  const queryClient = useQueryClient();
  const date = todayStr();
  const [hours, setHours] = useState("2");
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [suggest, setSuggest] = useState(null); // результат «сформировать»
  const [savedPlan, setSavedPlan] = useState(null);
  const [error, setError] = useState("");
  // Модалки из списка кандидатов: редактирование задачи / чтение конспекта.
  const [openTask, setOpenTask] = useState(null);
  const [openNote, setOpenNote] = useState(null);
  // Выполненные позиции (задача «Готова», конспект «Повторено») — зелёные.
  const [doneKeys, setDoneKeys] = useState({});

  // Полные данные задач (для модалки) и конспектов (для чтения).
  const tasksQuery = useTasks(true);
  const knowledgeQuery = useKnowledge(true);

  // Чтение: рекомендация на день, время за сегодня и последняя закладка
  // для продолжения. Цель дня своя у каждой даты (по умолчанию 1 час).
  const openReading = useAppStore((s) => s.openReading);
  const lastBookmarkQuery = useLastBookmark(true);
  const lastBookmark = lastBookmarkQuery.data || null;
  // Прочитанные книги для чтения в «Дне» не используем: сервер отдаёт их
  // в конце списка, поэтому первая непрочитанная — первая в списке.
  const booksQuery = useBooks(true);
  const nextBook = (booksQuery.data || []).find((b) => !b.finished_at) || null;
  const readingTarget = lastBookmark ? null : nextBook;
  const readingTimeQuery = useReadingTime(date);
  const todayReadingSeconds = readingTimeQuery.data?.seconds || 0;
  const readingGoalSeconds = readingTimeQuery.data?.goal_seconds || 3600;
  const readingGoalReached = todayReadingSeconds >= readingGoalSeconds;
  // Правка цели на сегодня: черновик в минутах (null — редактор закрыт).
  const [goalDraft, setGoalDraft] = useState(null);
  const [savingGoal, setSavingGoal] = useState(false);
  const [goalError, setGoalError] = useState("");

  const openGoalEditor = () => {
    setGoalDraft(String(Math.round(readingGoalSeconds / 60)));
    setGoalError("");
  };

  // Сохраняем цель только на сегодняшний день (в минутах).
  const handleSaveGoal = async () => {
    const minutes = Math.round(Number(goalDraft));
    if (!Number.isFinite(minutes) || minutes < 1 || minutes > 1440) {
      setGoalError("От 1 до 1440 мин");
      return;
    }
    setSavingGoal(true);
    setGoalError("");
    try {
      await setReadingGoal(date, minutes * 60);
      await queryClient.invalidateQueries({ queryKey: ["reading-time", date] });
      setGoalDraft(null);
    } catch (err) {
      setGoalError(err.message || "Не удалось сохранить цель");
    } finally {
      setSavingGoal(false);
    }
  };

  // «Читать»: открываем «Чтение» на последней закладке, а если её нет —
  // на первой непрочитанной книге. Прочитанные книги не предлагаем.
  const handleRead = () => {
    if (lastBookmark) {
      openReading({
        bookId: lastBookmark.book_id,
        anchor: lastBookmark.anchor,
        excerpt: lastBookmark.excerpt,
      });
    } else if (nextBook) {
      openReading({ bookId: nextBook.id });
    } else {
      openReading({});
    }
    onNavigate?.("reading");
  };

  // Открыть модалку по kind+id: полный объект берём из соответствующих
  // запросов (в списках кандидата/плана только краткая строка).
  const openByKind = (kind, id) => {
    if (kind === "task") {
      const t = tasksQuery.data?.tasks?.find((x) => x.id === id);
      if (t) setOpenTask(t);
    } else {
      const n = knowledgeQuery.data?.find((x) => x.id === id);
      if (n) setOpenNote(n);
    }
  };
  const openCandidate = (c) => openByKind(c.kind, c.id);

  // Перечитать план дня с сервера (заголовки/статусы могли поменяться).
  const reloadPlan = () =>
    fetchDayPlan(date)
      .then(setSavedPlan)
      .catch(() => {});

  // Отметить позицию выполненной: обновляем локально (сразу) и сохраняем
  // в БД (переживает перезагрузку страницы).
  const markPlanItem = async (kind, id, done) => {
    setSavedPlan((p) =>
      p
        ? {
            ...p,
            items: p.items.map((it) =>
              it.kind === kind && it.ref_id === id ? { ...it, done } : it,
            ),
          }
        : p,
    );
    try {
      await setDayItemDone(date, kind, id, done);
    } catch {
      /* план мог быть ещё не сохранён — отметка останется локальной */
    }
  };

  // После сохранения задачи из модалки: обновляем строку кандидата,
  // а «Готовую» задачу помечаем выполненной и убираем из плана.
  const handleTaskSaved = async (updated) => {
    setOpenTask(null);
    queryClient.invalidateQueries({ queryKey: ["tasks"] });
    if (!updated || updated.id == null) return;
    const done = updated.status === "done";
    setDoneKeys((prev) => {
      const next = { ...prev };
      const k = `task:${updated.id}`;
      if (done) next[k] = true;
      else delete next[k];
      return next;
    });
    setSuggest((prev) => {
      if (!prev) return prev;
      return {
        ...prev,
        tasks: prev.tasks.map((c) =>
          c.id === updated.id
            ? {
                ...c,
                selected: done ? false : c.selected,
                title: updated.title || c.title,
                meta: taskMetaLabel(updated.category, updated.planned_hours),
                minutes: taskMinutesFromHours(updated.planned_hours),
              }
            : c,
        ),
      };
    });
    await markPlanItem("task", updated.id, done);
    await reloadPlan();
  };

  // После «Я повторил» в модалке конспекта: помечаем выполненной и
  // убираем из плана (повторение уже состоялось).
  const handleNoteRepeated = async (note) => {
    try {
      await repeatKnowledge(note.id);
      queryClient.invalidateQueries({ queryKey: ["knowledge"] });
      setDoneKeys((prev) => ({ ...prev, [`note:${note.id}`]: true }));
      markPlanItem("note", note.id, true);
      setSuggest((prev) =>
        prev
          ? {
              ...prev,
              notes: prev.notes.map((c) =>
                c.id === note.id ? { ...c, selected: false } : c,
              ),
            }
          : prev,
      );
      setOpenNote(null);
      await markPlanItem("note", note.id, true);
      await reloadPlan();
    } catch (err) {
      window.alert(err.message || "Не удалось отметить повторение");
    }
  };

  const load = async () => {
    try {
      const plan = await fetchDayPlan(date);
      setSavedPlan(plan);
    } catch {
      /* раздел не критичен */
    }
  };

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [date]);

  const handleSuggest = async () => {
    const minutes = Math.round(Number(hours || 0) * 60);
    if (minutes < 1) {
      window.alert("Укажите доступное время (в часах)");
      return;
    }
    setLoading(true);
    setError("");
    try {
      const data = await suggestDay({ date, minutes });
      setSuggest(data);
    } catch (err) {
      setError(err.message || "Не удалось сформировать день");
    } finally {
      setLoading(false);
    }
  };

  const toggle = (kind, cand) => {
    if (!suggest) return;
    setSuggest({
      ...suggest,
      [kind === "task" ? "tasks" : "notes"]: suggest[kind === "task" ? "tasks" : "notes"].map((c) =>
        c.id === cand.id && c.kind === cand.kind ? { ...c, selected: !c.selected } : c,
      ),
    });
  };

  const handleSave = async () => {
    if (!suggest) return;
    const items = [...suggest.tasks, ...suggest.notes]
      .filter((c) => c.selected)
      .map((c) => ({ kind: c.kind, ref_id: c.id }));
    setSaving(true);
    setError("");
    try {
      const plan = await saveDay({
        date,
        budget_minutes: Math.round(Number(hours || 0) * 60),
        items,
      });
      setSavedPlan(plan);
      setSuggest(null);
    } catch (err) {
      setError(err.message || "Не удалось сохранить день");
    } finally {
      setSaving(false);
    }
  };
  const selectedMinutes = useMemo(() => {
    if (!suggest) return 0;
    return [...suggest.tasks, ...suggest.notes]
      .filter((c) => c.selected)
      .reduce((s, c) => s + c.minutes, 0);
  }, [suggest]);

  return (
    <section>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="flex items-center gap-2 text-xl font-semibold">
          <CalendarDays className="size-5 text-muted-foreground" />
          День · <DateDisplay date={date} />
        </h2>
      </div>

      {/* Лимит времени + «Сформировать» */}
      <Card className="my-3" size="sm">
        <CardContent className="flex flex-wrap items-end gap-3 pt-4">
          <div className="space-y-1.5">
            <label className="flex items-center gap-1.5 text-sm font-medium">
              <Clock className="size-3.5" />
              Сколько времени есть сегодня (в часах)
            </label>
            <Input
              type="number"
              min="0.25"
              step="0.25"
              value={hours}
              onChange={(e) => setHours(e.target.value)}
              className="w-32"
            />
          </div>
          <Button onClick={handleSuggest} disabled={loading}>
            {loading ? <Loader2 className="animate-spin" /> : <Sparkles />}
            {loading ? "Формирую…" : "Сформировать"}
          </Button>
          <span className="text-xs text-muted-foreground">
            Метрики в лимит не входят и всегда показываются ниже.
          </span>
        </CardContent>
      </Card>

      {error && (
        <p className="my-2 flex items-center gap-1.5 text-sm text-destructive" role="alert">
          <AlertCircle className="size-4" />
          {error}
        </p>
      )}

      {suggest ? (
        <>
          <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
            <span>Использовано:</span>
            <span className="font-medium text-foreground">
              {fmtMin(selectedMinutes)} из {fmtMin(suggest.minutes)}
            </span>
            <span className="text-xs">(скорость чтения {suggest.reading_speed} симв/мин)</span>
          </div>

          {/* Задачи */}
          <Card className="my-3" size="sm">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-sm">
                <ListTodo className="size-4 text-muted-foreground" />
                Задачи на день
                <Badge variant="secondary">{suggest.tasks.length}</Badge>
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-1.5">
              {suggest.tasks.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  Нет активных задач. Добавьте задачи в разделе «Задачи».
                </p>
              ) : (
                suggest.tasks.map((c) => (
                  <CandidateRow
                    key={`t${c.id}`}
                    c={c}
                    onToggle={(x) => toggle("task", x)}
                    onOpen={openCandidate}
                    done={Boolean(doneKeys[`task:${c.id}`])}
                  />
                ))
              )}
            </CardContent>
          </Card>

          {/* Повторение знаний */}
          <Card className="my-3" size="sm">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-sm">
                <BookOpen className="size-4 text-muted-foreground" />
                Повторить из знаний
                <Badge variant="secondary">{suggest.notes.length}</Badge>
              </CardTitle>
              <CardDescription>
                Время считается по символам конспекта.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-1.5">
              {suggest.notes.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  Сейчас нет заметок к повторению.
                </p>
              ) : (
                suggest.notes.map((c) => (
                  <CandidateRow
                    key={`n${c.id}`}
                    c={c}
                    onToggle={(x) => toggle("note", x)}
                    onOpen={openCandidate}
                    done={Boolean(doneKeys[`note:${c.id}`])}
                  />
                ))
              )}
            </CardContent>
          </Card>

          <div className="my-3 flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setSuggest(null)}>
              <X />
              Отмена
            </Button>
            <Button onClick={handleSave} disabled={saving || selectedMinutes === 0}>
              {saving ? <Loader2 className="animate-spin" /> : <Save />}
              {saving ? "Сохраняю…" : "Сохранить день"}
            </Button>
          </div>
        </>
      ) : (
        /* Уже сформированный день (если есть) */
        savedPlan?.items?.length > 0 && (
          <Card className="my-3" size="sm">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-sm">
                <CheckCircle2 className="size-4 text-muted-foreground" />
                План дня
                <span className="ml-auto text-xs font-normal text-muted-foreground">
                  {fmtMin(savedPlan.total_minutes)}
                </span>
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-1.5">
              {savedPlan.items.map((it, i) => {
                const done = Boolean(
                  it.done || doneKeys[`${it.kind}:${it.ref_id}`],
                );
                return (
                  <button
                    key={`${it.kind}-${it.ref_id}-${i}`}
                    type="button"
                    onClick={() => openByKind(it.kind, it.ref_id)}
                    className={cn(
                      "flex w-full cursor-pointer items-center justify-between gap-2 rounded-md border px-3 py-1.5 text-left transition-colors hover:bg-muted/40",
                      done
                        ? "border-emerald-500/50 bg-emerald-500/10"
                        : "border-border/60",
                    )}
                    title="Открыть"
                  >
                    <span className="flex min-w-0 items-center gap-1.5 text-sm">
                      <span className="shrink-0">
                        {it.kind === "task" ? "☑" : "▤"}
                      </span>
                      <span className="min-w-0 truncate">{it.title}</span>
                      {done && (
                        <CheckCircle2 className="size-3.5 shrink-0 text-emerald-600 dark:text-emerald-400" />
                      )}
                    </span>
                    <span className="shrink-0 text-xs text-muted-foreground">
                      {fmtMin(it.minutes)}
                    </span>
                  </button>
                );
              })}
              <p className="pt-1 text-xs text-muted-foreground">
                Чтобы изменить план — задайте время выше и нажмите «Сформировать».
              </p>
            </CardContent>
          </Card>
        )
      )}

      {/* Чтение: цель на день (по умолчанию 1 ч, правится на эту дату),
          прочитанное за сегодня и переход к последней закладке. При достижении
          цели карточка зеленеет. */}
      <Card
        className={cn(
          "my-3",
          readingGoalReached && "border-emerald-500/50 bg-emerald-500/10",
        )}
        size="sm"
      >
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm">
            <BookOpen className="size-4 text-muted-foreground" />
            Чтение
          </CardTitle>
          <CardDescription>
            Рекомендация на день — {fmtMin(readingGoalSeconds / 60)}.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {goalDraft === null ? (
            <p className="flex flex-wrap items-center gap-x-1.5 text-sm text-muted-foreground">
              {readingTimeQuery.isLoading ? (
                "Смотрю время чтения…"
              ) : (
                <>
                  Сегодня:{" "}
                  <span
                    className={cn(
                      "tabular-nums",
                      readingGoalReached
                        ? "text-emerald-600 dark:text-emerald-400"
                        : "text-foreground",
                    )}
                  >
                    {formatClock(todayReadingSeconds)}
                  </span>
                  из{" "}
                  <span className="tabular-nums text-foreground">
                    {formatClock(readingGoalSeconds)}
                  </span>
                  {readingGoalReached && (
                    <span className="flex items-center gap-1 text-emerald-600 dark:text-emerald-400">
                      <CheckCircle2 className="size-3.5" />
                      цель дня выполнена
                    </span>
                  )}
                  <Button
                    variant="ghost"
                    size="icon"
                    className="size-6"
                    onClick={openGoalEditor}
                    title="Изменить цель чтения на сегодня"
                    aria-label="Изменить цель чтения на сегодня"
                  >
                    <Pencil className="size-3.5" />
                  </Button>
                </>
              )}
            </p>
          ) : (
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-sm text-muted-foreground">
                Цель чтения на сегодня:
              </span>
              <Input
                type="number"
                min="1"
                max="1440"
                step="5"
                value={goalDraft}
                onChange={(e) => setGoalDraft(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") handleSaveGoal();
                  if (e.key === "Escape") setGoalDraft(null);
                }}
                className="w-20"
                aria-label="Цель чтения в минутах"
                autoFocus
              />
              <span className="text-sm text-muted-foreground">мин</span>
              <Button size="sm" onClick={handleSaveGoal} disabled={savingGoal}>
                {savingGoal ? <Loader2 className="animate-spin" /> : <Save />}
                Сохранить
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setGoalDraft(null)}
                disabled={savingGoal}
              >
                <X />
                Отмена
              </Button>
              {goalError && (
                <span className="flex items-center gap-1 text-xs text-destructive">
                  <AlertCircle className="size-3.5" />
                  {goalError}
                </span>
              )}
            </div>
          )}
          <p className="text-sm text-muted-foreground">
            {lastBookmarkQuery.isLoading || booksQuery.isLoading ? (
              "Смотрю книги…"
            ) : lastBookmark ? (
              <>
                Продолжить:{" "}
                <span className="text-foreground">
                  «{bookmarkLabel(lastBookmark.excerpt)}»
                </span>
                {lastBookmark.book_title && ` — ${lastBookmark.book_title}`}
              </>
            ) : nextBook ? (
              <>
                Начну книгу:{" "}
                <span className="text-foreground">{nextBook.title}</span>
              </>
            ) : (booksQuery.data || []).length > 0 ? (
              "Все книги прочитаны."
            ) : (
              "Книг пока нет — добавьте в разделе «Чтение»."
            )}
          </p>
          <Button
            size="sm"
            onClick={handleRead}
            disabled={lastBookmarkQuery.isLoading || booksQuery.isLoading}
          >
            <BookOpen />
            Читать
          </Button>
        </CardContent>
      </Card>

      {/* Метрики на день */}
      <Card className="my-3" size="sm">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm">
            <BarChart3 className="size-4 text-muted-foreground" />
            Метрики за сегодня
          </CardTitle>
          <CardDescription>Время не рассчитывается.</CardDescription>
        </CardHeader>
        <CardContent>
          <MetricsBlock date={date} />
        </CardContent>
      </Card>

      {/* Отчёт за сегодня — в конце дня */}
      <DayReportCard date={date} />

      {/* Модалки из списка кандидатов */}
      {openTask && (
        <TaskFormModal
          initial={openTask}
          categories={tasksQuery.data?.categories || []}
          goals={tasksQuery.data?.goals || []}
          onClose={() => setOpenTask(null)}
          onSaved={handleTaskSaved}
        />
      )}
      {openNote && (
        <NoteReadModal
          note={openNote}
          done={Boolean(doneKeys[`note:${openNote.id}`])}
          onClose={() => setOpenNote(null)}
          onRepeat={() => handleNoteRepeated(openNote)}
        />
      )}
    </section>
  );
}

export default Day;
