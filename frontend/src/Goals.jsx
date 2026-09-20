import { useState, useRef } from "react";
import {
  useGoals,
  createGoal,
  updateGoal,
  deleteGoal,
  useTasks,
  updateTask,
  generateGoalTasks,
  reorderGoalTasks,
} from "./api.js";
import { TaskFormModal, taskPayload, statusLabel, statusVariant, fmtHours } from "./Tasks.jsx";
import { useQueryClient } from "@tanstack/react-query";
import DateDisplay from "@/components/DateDisplay.jsx";
import DateInput from "@/components/DateInput.jsx";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Progress } from "@/components/ui/progress";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardDescription,
} from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Target,
  Plus,
  Save,
  Trash2,
  Loader2,
  AlertCircle,
  Edit,
  X,
  CalendarDays,
  Link2,
  Link2Off,
  ListTodo,
  Check,
  Sparkles,
  GripVertical,
  ChevronDown,
  ChevronRight,
} from "lucide-react";

// Статусы целей.
const STATUSES = [
  { value: "active", label: "В работе", variant: "default" },
  { value: "paused", label: "Приостановлена", variant: "outline" },
  { value: "achieved", label: "Достигнута", variant: "secondary" },
  { value: "cancelled", label: "Отменена", variant: "destructive" },
];

const statusMeta = (s) => STATUSES.find((x) => x.value === s) || STATUSES[0];

// Сортировка задач цели по последовательности выполнения (position, затем id).
const byGoalOrder = (a, b) =>
  (a.position ?? 0) - (b.position ?? 0) || a.id - b.id;

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

// Модалка создания/редактирования цели.
// При создании можно сгенерировать черновики задач (ИИ): они показываются
// в форме, редактируются списком и сохраняются вместе с целью по «Сохранить».
// Удаление цели (с опцией «удалить привязанные задачи») тоже живёт здесь.
function GoalFormModal({ initial, taskCount = 0, onClose, onSaved }) {
  const isCreate = !initial;
  const [title, setTitle] = useState(initial?.title || "");
  const [description, setDescription] = useState(initial?.description || "");
  const [targetDate, setTargetDate] = useState(initial?.target_date || "");
  const [status, setStatus] = useState(initial?.status || "active");
  const [drafts, setDrafts] = useState([]); // черновики задач (ещё не в БД)
  const [truncated, setTruncated] = useState(false); // ответ модели обрезан
  const [generating, setGenerating] = useState(false);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteTasks, setDeleteTasks] = useState(false); // удалить задачи вместе с целью
  const [error, setError] = useState("");

  const removeDraft = (index) =>
    setDrafts((prev) => prev.filter((_, i) => i !== index));

  const handleGenerate = async () => {
    if (!isCreate || !title.trim()) return;
    if (
      drafts.length > 0 &&
      !window.confirm("Заменить текущий список задач новым?")
    )
      return;
    setGenerating(true);
    setError("");
    setTruncated(false);
    try {
      const data = await generateGoalTasks({ title, description });
      setDrafts(
        (data.tasks || []).map((t) => ({
          title: t.title || "",
          description: t.description || "",
          category: t.category || "Прочее",
          planned_hours: Number(t.planned_hours) || 0,
        })),
      );
      // Сервер сообщает, если ответ модели обрезан по лимиту длины.
      setTruncated(Boolean(data.truncated));
    } catch (err) {
      setError(err.message || "Не удалось сгенерировать задачи");
    } finally {
      setGenerating(false);
    }
  };

  const handleSave = async () => {
    setSaving(true);
    setError("");
    const payload = {
      title,
      description,
      target_date: targetDate,
      status,
    };
    if (isCreate) {
      payload.tasks = drafts
        .map((d) => ({
          title: String(d.title || "").trim(),
          description: d.description || "",
          category: d.category || "Прочее",
          planned_hours: Number(d.planned_hours) || 0,
        }))
        .filter((d) => d.title);
    }
    try {
      if (initial) {
        await updateGoal(initial.id, payload);
      } else {
        await createGoal(payload);
      }
      onSaved();
      onClose();
    } catch (err) {
      setError(err.message || "Не удалось сохранить цель");
      setSaving(false);
    }
  };

  // Удаление доступно только в режиме редактирования.
  const handleDelete = async () => {
    if (!window.confirm(`Удалить цель «${initial.title}»?`)) return;
    setDeleting(true);
    setError("");
    try {
      await deleteGoal(initial.id, deleteTasks);
      onSaved();
      onClose();
    } catch (err) {
      setError(err.message || "Не удалось удалить цель");
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
            <Target className="size-4 text-muted-foreground" />
            {initial ? "Редактировать цель" : "Новая цель"}
          </CardTitle>
          <CardDescription>
            Дедлайн и статус. Прогресс считается по привязанным задачам.
          </CardDescription>
        </CardHeader>

        <CardContent className="flex-1 space-y-4 overflow-y-auto">
          <div className="space-y-1.5">
            <Label>Название</Label>
            <Input
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Например: пробежать марафон"
            />
          </div>

          <div className="space-y-1.5">
            <Label>Описание</Label>
            <Textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Критерии достижения (необязательно)…"
            />
          </div>

          {isCreate && (
            <div className="space-y-2 rounded-lg border border-border/60 p-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="min-w-0">
                  <p className="text-sm font-medium">Задачи цели</p>
                  <p className="text-xs text-muted-foreground">
                    Черновики сохранятся вместе с целью (если оставить поле
                    пустым — цель создастся без задач). ИИ делает 5–8 задач, а
                    число в описании цели («разбей на 28 задач») будет учтено.
                  </p>
                </div>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={handleGenerate}
                  disabled={generating || !title.trim()}
                >
                  {generating ? (
                    <Loader2 className="animate-spin" />
                  ) : (
                    <Sparkles />
                  )}
                  {generating ? "Генерирую…" : "Создать задачи с ИИ"}
                </Button>
              </div>

              {/* Предупреждение, если ответ модели обрезан по лимиту длины */}
              {truncated && (
                <p className="flex items-center gap-1.5 text-xs text-amber-600 dark:text-amber-400">
                  <AlertCircle className="size-3.5" />
                  Ответ модели обрезан — задач может быть меньше, чем нужно.
                  Нажмите «Создать задачи с ИИ» ещё раз.
                </p>
              )}

              {drafts.length === 0 ? (
                <p className="text-xs text-muted-foreground">
                  Задачи ещё не сгенерированы — нажмите кнопку выше.
                </p>
              ) : (
                <ul className="space-y-1.5">
                  {drafts.map((d, i) => (
                    <li
                      key={i}
                      className="flex items-start justify-between gap-2 rounded-md border border-border/60 px-2 py-1.5"
                    >
                      <div className="min-w-0">
                        <p className="wrap-break-word text-sm font-medium text-foreground">
                          {d.title}
                        </p>
                        {d.description && (
                          <p className="wrap-break-word text-xs text-muted-foreground">
                            {d.description}
                          </p>
                        )}
                        <div className="mt-0.5 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                          <Badge variant="outline">{d.category}</Badge>
                          {Number(d.planned_hours) > 0 && (
                            <span>≈ {d.planned_hours} ч</span>
                          )}
                        </div>
                      </div>
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon-sm"
                        className="-mt-1 -mr-1 shrink-0 text-muted-foreground hover:text-destructive"
                        onClick={() => removeDraft(i)}
                        aria-label="Удалить задачу"
                      >
                        <X className="size-4" />
                      </Button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )}

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label>Дедлайн</Label>
              <DateInput value={targetDate} onChange={setTargetDate} />
            </div>
            <div className="space-y-1.5">
              <Label>Статус</Label>
              <Select value={status} onValueChange={setStatus}>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {STATUSES.map((s) => (
                    <SelectItem key={s.value} value={s.value}>
                      {s.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          {!isCreate && (
            <div className="space-y-2 rounded-lg border border-destructive/40 p-3">
              <p className="flex items-center gap-1.5 text-sm font-medium text-destructive">
                <Trash2 className="size-3.5" />
                Удаление цели
              </p>
              {taskCount > 0 ? (
                <button
                  type="button"
                  onClick={() => setDeleteTasks((v) => !v)}
                  className="flex w-full items-start gap-2 rounded-lg border border-border/60 px-3 py-2 text-left transition-colors hover:bg-muted/40"
                >
                  <span
                    className={cn(
                      "mt-0.5 flex size-4 shrink-0 items-center justify-center rounded border transition-colors",
                      deleteTasks
                        ? "border-primary bg-primary text-primary-foreground"
                        : "border-input bg-transparent",
                    )}
                  >
                    {deleteTasks && <Check className="size-3" />}
                  </span>
                  <span className="min-w-0">
                    <span className="block text-sm font-medium text-foreground">
                      Удалить привязанные задачи
                    </span>
                    <span className="block text-xs text-muted-foreground">
                      Будет удалено {taskCount} задач
                    </span>
                  </span>
                </button>
              ) : (
                <p className="text-xs text-muted-foreground">
                  К цели не привязано задач.
                </p>
              )}
              <p className="text-xs text-muted-foreground">
                Цель удаляется кнопкой внизу — действие необратимо.
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
              {deleting ? "Удаляю…" : "Удалить цель"}
            </Button>
          )}
          <div className="flex gap-2">
            <Button variant="ghost" onClick={onClose} disabled={saving || deleting}>
              <X />
              Отмена
            </Button>
            <Button
              onClick={handleSave}
              disabled={saving || deleting || !title.trim()}
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

// Модалка «Привязать задачи»: выбор из задач без цели и уже привязанных
// к этой цели. Изменения применяются по кнопке «Сохранить».
function AttachTasksModal({ goal, tasks, onClose, onSaved }) {
  const candidates = tasks.filter(
    (t) => t.goal_id == null || t.goal_id === goal.id,
  );
  const [selected, setSelected] = useState(
    () =>
      new Set(
        candidates.filter((t) => t.goal_id === goal.id).map((t) => t.id),
      ),
  );
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const toggle = (id) =>
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });

  const handleSave = async () => {
    setSaving(true);
    setError("");
    const changed = candidates.filter(
      (t) => selected.has(t.id) !== (t.goal_id === goal.id),
    );
    try {
      for (const t of changed) {
        await updateTask(
          t.id,
          taskPayload(t, { goal_id: selected.has(t.id) ? goal.id : null }),
        );
      }
      onSaved();
      onClose();
    } catch (err) {
      setError(err.message || "Не удалось сохранить изменения");
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
            <Link2 className="size-4 text-muted-foreground" />
            Привязать задачи
          </CardTitle>
          <CardDescription>К цели «{goal.title}»</CardDescription>
        </CardHeader>

        <CardContent className="flex-1 space-y-2 overflow-y-auto">
          {candidates.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              Нет свободных задач без цели. Сначала создайте задачу.
            </p>
          ) : (
            candidates.map((t) => {
              const checked = selected.has(t.id);
              return (
                <button
                  key={t.id}
                  type="button"
                  onClick={() => toggle(t.id)}
                  className="flex w-full items-start gap-3 rounded-lg border border-border/60 px-3 py-2 text-left transition-colors hover:bg-muted/40"
                >
                  <span
                    className={cn(
                      "mt-0.5 flex size-4 shrink-0 items-center justify-center rounded border transition-colors",
                      checked
                        ? "border-primary bg-primary text-primary-foreground"
                        : "border-input bg-transparent",
                    )}
                  >
                    {checked && <Check className="size-3" />}
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="wrap-break-word block text-sm font-medium text-foreground">
                      {t.title}
                    </span>
                    <span className="mt-0.5 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                      <Badge variant={statusVariant(t.status)}>
                        {statusLabel(t.status)}
                      </Badge>
                      <span>{t.category || "Прочее"}</span>
                      {Number(t.planned_hours) > 0 && (
                        <span>План: {t.planned_hours} ч</span>
                      )}
                    </span>
                  </span>
                </button>
              );
            })
          )}

          {error && (
            <p
              className="flex items-center gap-1.5 pt-1 text-sm text-destructive"
              role="alert"
            >
              <AlertCircle className="size-4" />
              {error}
            </p>
          )}
        </CardContent>

        <div className="flex justify-end gap-2 border-t p-4">
          <Button variant="ghost" onClick={onClose} disabled={saving}>
            <X />
            Отмена
          </Button>
          <Button
            onClick={handleSave}
            disabled={saving || candidates.length === 0}
          >
            {saving ? <Loader2 className="animate-spin" /> : <Save />}
            {saving ? "Сохраняю…" : "Сохранить"}
          </Button>
        </div>
      </Card>
    </div>
  );
}

// Карточка цели.
function GoalCard({
  goal,
  goalTasks = [],
  onEdit,
  onStatusChange,
  onAddTask,
  onAttach,
  onUnlink,
  onOpenTask,
  onMoveTask,
}) {
  const meta = statusMeta(goal.status);
  // Сворачивание карточки: в шапке всегда видны название, прогресс и часы.
  const [open, setOpen] = useState(true);

  // Прогресс цели считается на сервере по задачам; здесь — подписи для бара.
  const orderedTasks = [...goalTasks].sort(byGoalOrder);
  const activeTasks = orderedTasks.filter((t) => t.status !== "cancelled");
  const doneTasks = activeTasks.filter((t) => t.status === "done");
  const sumHours = (list) =>
    list.reduce((s, t) => s + (Number(t.planned_hours) || 0), 0);
  const totalHours = sumHours(activeTasks);
  const doneHours = sumHours(doneTasks);
  // Выполненные задачи — в конце списка (как в разделе «Задачи»), остальные
  // (включая отменённые) сохраняют заданную последовательность выполнения.
  const displayTasks = [
    ...orderedTasks.filter((t) => t.status !== "done"),
    ...doneTasks,
  ];

  // Перетаскивание строк для смены последовательности выполнения.
  const listRef = useRef(null);
  const dragRef = useRef(null); // индекс перетаскиваемой задачи
  const overRef = useRef(null); // позиция вставки (перед строкой N)
  const pointerRef = useRef(null); // {pointerId, handle} для releasePointerCapture
  const [dragIndex, setDragIndex] = useState(null);
  const [overIndex, setOverIndex] = useState(null);

  const startDrag = (e, i) => {
    if (e.pointerType === "mouse" && e.button !== 0) return;
    e.preventDefault();
    pointerRef.current = { pointerId: e.pointerId, handle: e.currentTarget };
    try {
      e.currentTarget.setPointerCapture(e.pointerId);
    } catch {
      // игнорируем, если захват указателя недоступен
    }
    dragRef.current = i;
    overRef.current = i;
    setDragIndex(i);
    setOverIndex(i);
  };

  const moveDrag = (e) => {
    if (dragRef.current == null) return;
    const rows = listRef.current?.querySelectorAll("[data-goal-task]") || [];
    if (rows.length < 2) return;
    const y = e.clientY;
    let idx = rows.length - 1;
    for (let n = 0; n < rows.length; n++) {
      const r = rows[n].getBoundingClientRect();
      if (y < r.top + r.height / 2) {
        idx = n;
        break;
      }
    }
    overRef.current = idx;
    setOverIndex(idx);
  };

  const endDrag = () => {
    const from = dragRef.current;
    const over = overRef.current;
    dragRef.current = null;
    overRef.current = null;
    const meta = pointerRef.current;
    pointerRef.current = null;
    if (meta) {
      try {
        meta.handle.releasePointerCapture(meta.pointerId);
      } catch {
        // игнорируем
      }
    }
    setDragIndex(null);
    setOverIndex(null);
    if (from == null || over == null || over === from) return;
    // Новый порядок: вынимаем перетаскиваемую и вставляем на позицию over.
    // Считаем по отображаемому списку — он же уходит на сервер.
    const items = [...displayTasks];
    const [moved] = items.splice(from, 1);
    items.splice(over > from ? over - 1 : over, 0, moved);
    onMoveTask(goal, items.map((x) => x.id));
  };

  return (
    <Card className="my-3" size="sm">
      {/* Шапка цели: название, статус, прогресс и часы. Клик сворачивает/разворачивает. */}
      <CardHeader
        className="cursor-pointer select-none"
        onClick={() => setOpen((v) => !v)}
      >
        <div className="flex items-start justify-between gap-2">
          <div className="flex min-w-0 items-start gap-1.5">
            {open ? (
              <ChevronDown className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
            ) : (
              <ChevronRight className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
            )}
            <div className="min-w-0">
              <p className="wrap-break-word font-medium text-foreground">
                {goal.title}
              </p>
              <div className="mt-1 flex flex-wrap items-center gap-1.5">
                <Badge variant={meta.variant}>{meta.label}</Badge>
                {goal.target_date && (
                  <Badge variant="outline">
                    <CalendarDays className="mr-1 size-3" />
                    Дедлайн: <DateDisplay date={goal.target_date} />
                  </Badge>
                )}
              </div>
            </div>
          </div>
          <div
            className="flex shrink-0 gap-1"
            onClick={(e) => e.stopPropagation()}
          >
            <Button variant="outline" size="icon-sm" onClick={() => onEdit(goal)}>
              <Edit />
            </Button>
          </div>
        </div>

        <div className="space-y-1">
          <div className="flex items-center gap-3">
            <Progress value={goal.progress ?? 0} className="flex-1" />
            <span className="text-sm font-medium tabular-nums">
              {goal.progress ?? 0}%
            </span>
          </div>
          {activeTasks.length > 0 ? (
            <p className="text-xs text-muted-foreground">
              Часы: {fmtHours(doneHours)} / {fmtHours(totalHours)} · выполнено{" "}
              {doneTasks.length} из {activeTasks.length} задач
            </p>
          ) : (
            <p className="text-xs text-muted-foreground">
              Прогресс появится после добавления задач
            </p>
          )}
          {goal.status !== "achieved" &&
            activeTasks.length > 0 &&
            doneTasks.length === activeTasks.length && (
              <p className="text-xs text-muted-foreground">
                Все задачи выполнены — поставьте статус «Достигнута», чтобы
                получить 100%
              </p>
            )}
        </div>
      </CardHeader>

      {open && (
        <CardContent className="space-y-3">
          {goal.description && (
            <p className="whitespace-pre-wrap text-sm leading-relaxed text-muted-foreground">
              {goal.description}
            </p>
          )}

          <div className="border-t pt-3">
            <Select
              value={goal.status}
              onValueChange={(s) => onStatusChange(goal, s)}
            >
              <SelectTrigger className="h-8 w-44">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {STATUSES.map((s) => (
                  <SelectItem key={s.value} value={s.value}>
                    {s.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {/* Задачи цели: список привязанных, создание новой и привязка существующей */}
          <div className="border-t pt-3">
            <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
              <span className="flex items-center gap-1.5 text-xs font-semibold tracking-wide text-muted-foreground uppercase">
                <ListTodo className="size-3.5" />
                Задачи цели ({goalTasks.length})
              </span>
              <div className="flex flex-wrap gap-1.5">
                <Button variant="outline" size="sm" onClick={() => onAddTask(goal)}>
                  <Plus />
                  Задача
                </Button>
                <Button variant="outline" size="sm" onClick={() => onAttach(goal)}>
                  <Link2 />
                  Привязать
                </Button>
              </div>
            </div>

            {displayTasks.length > 1 && (
              <p className="mb-2 flex items-center gap-1 text-[11px] text-muted-foreground">
                <GripVertical className="size-3" />
                Перетащите задачу за ручку, чтобы изменить порядок выполнения
              </p>
            )}

            {displayTasks.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                Задач пока нет — добавьте новую или привяжите существующую.
              </p>
            ) : (
              <ul ref={listRef} className="space-y-1.5">
                {displayTasks.map((t, i) => {
                  const dragging = dragIndex === i;
                  const dropOver =
                    overIndex === i && dragIndex !== null && dragIndex !== i;
                  const done = t.status === "done";
                  return (
                    <li
                      key={t.id}
                      data-goal-task="true"
                      className={cn(
                        "flex items-start justify-between gap-1.5 rounded-lg border border-border/60 px-2 py-1.5 transition-colors",
                        dragging ? "opacity-50" : "hover:bg-muted/40",
                        // Выполненные задачи — серые, как неактивные.
                        done && "text-muted-foreground opacity-60",
                        dropOver && "border-primary/70 ring-1 ring-primary/40",
                      )}
                    >
                      {/* Номер по порядку выполнения */}
                      <span
                        className="w-5 shrink-0 pt-1.5 text-center text-xs font-medium tabular-nums text-muted-foreground"
                        aria-hidden="true"
                      >
                        {i + 1}
                      </span>
                      {/* Ручка перетаскивания (меняет последовательность) */}
                      <button
                        type="button"
                        onPointerDown={(e) => startDrag(e, i)}
                        onPointerMove={moveDrag}
                        onPointerUp={endDrag}
                        onPointerCancel={endDrag}
                        onLostPointerCapture={endDrag}
                        style={{ touchAction: "none" }}
                        title="Перетащите, чтобы изменить порядок выполнения"
                        aria-label={`Переместить задачу «${t.title}»`}
                        className="mt-0.5 shrink-0 cursor-grab touch-none rounded p-0.5 text-muted-foreground transition-colors select-none hover:bg-muted hover:text-foreground active:cursor-grabbing focus-visible:ring-2 focus-visible:ring-ring/50"
                      >
                        <GripVertical className="size-3.5" />
                      </button>
                      {/* Клик по задаче открывает её (редактирование) */}
                      <button
                        type="button"
                        onClick={() => onOpenTask(t)}
                        title="Открыть задачу"
                        className="min-w-0 flex-1 cursor-pointer rounded text-left outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
                      >
                        <span
                          className={cn(
                            "wrap-break-word block text-sm font-medium",
                            done ? "text-muted-foreground" : "text-foreground",
                          )}
                        >
                          {t.title}
                        </span>
                        <span className="mt-0.5 flex flex-wrap items-center gap-1.5">
                          <Badge variant={statusVariant(t.status)}>
                            {statusLabel(t.status)}
                          </Badge>
                          <span className="text-xs text-muted-foreground">
                            {t.category || "Прочее"}
                            {Number(t.planned_hours) > 0 &&
                              ` · План: ${t.planned_hours} ч`}
                          </span>
                        </span>
                      </button>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        className="-mt-1 -mr-1 shrink-0 text-muted-foreground hover:text-destructive"
                        onClick={() => onUnlink(t)}
                        aria-label="Отвязать задачу от цели"
                      >
                        <Link2Off className="size-4" />
                      </Button>
                    </li>
                  );
                })}
              </ul>
            )}
          </div>
        </CardContent>
      )}
    </Card>
  );
}

// Раздел «Цели»: список целей с прогрессом + добавление/редактирование.
// Для каждой цели можно создать задачу или привязать существующую.
function Goals() {
  const queryClient = useQueryClient();
  const goalsQuery = useGoals(true);
  const tasksQuery = useTasks(true);
  const [modal, setModal] = useState(null); // цель: создать/редактировать
  const [taskModalGoal, setTaskModalGoal] = useState(null); // новая задача для цели
  const [attachGoal, setAttachGoal] = useState(null); // привязка задач к цели
  const [editTask, setEditTask] = useState(null); // открытая задача цели (редактирование)

  const refreshGoals = () =>
    queryClient.invalidateQueries({ queryKey: ["goals"] });
  const refreshTasks = () =>
    queryClient.invalidateQueries({ queryKey: ["tasks"] });
  // Статус/состав задач влияет на прогресс цели (считается сервером из задач),
  // поэтому действия с задачами обновляют оба кэша.
  const refreshAll = () => {
    refreshGoals();
    refreshTasks();
  };

  const handleStatusChange = async (goal, status) => {
    try {
      await updateGoal(goal.id, {
        title: goal.title,
        description: goal.description,
        target_date: goal.target_date,
        status,
      });
      refreshGoals();
    } catch (err) {
      window.alert(err.message || "Не удалось изменить статус");
    }
  };

  const handleUnlink = async (task) => {
    if (!window.confirm(`Отвязать задачу «${task.title}» от цели?`)) return;
    try {
      await updateTask(task.id, taskPayload(task, { goal_id: null }));
      refreshAll(); // состав задач влияет на прогресс цели
    } catch (err) {
      window.alert(err.message || "Не удалось отвязать задачу");
    }
  };

  // Сохранение новой последовательности задач цели (после перетаскивания).
  const handleMoveTask = async (goal, orderedIds) => {
    try {
      await reorderGoalTasks(goal.id, orderedIds);
      refreshAll();
    } catch (err) {
      window.alert(err.message || "Не удалось изменить порядок задач");
    }
  };

  if (goalsQuery.isLoading || tasksQuery.isLoading) {
    return (
      <p className="text-sm text-muted-foreground">
        <Loader2 className="mr-1 inline size-4 animate-spin" />
        Загрузка целей…
      </p>
    );
  }
  if (goalsQuery.isError) {
    return (
      <p className="text-sm text-destructive">
        Ошибка: {goalsQuery.error?.message}
      </p>
    );
  }
  if (tasksQuery.isError) {
    return (
      <p className="text-sm text-destructive">
        Ошибка задач: {tasksQuery.error?.message}
      </p>
    );
  }

  const items = goalsQuery.data?.goals || [];
  const allTasks = tasksQuery.data?.tasks || [];
  const categories = tasksQuery.data?.categories || [];
  const goals = tasksQuery.data?.goals || [];

  return (
    <section>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="flex items-center gap-2 text-xl font-semibold">
          <Target className="size-5 text-muted-foreground" />
          Цели
        </h2>
        <Button onClick={() => setModal({ goal: null })}>
          <Plus />
          Новая цель
        </Button>
      </div>

      {items.length === 0 ? (
        <Card className="my-3" size="sm">
          <CardContent>
            <p className="flex items-center gap-2 text-sm text-muted-foreground">
              <Target className="size-4" />
              Пока нет ни одной цели. Добавьте первую.
            </p>
          </CardContent>
        </Card>
      ) : (
        items.map((g) => (
          <GoalCard
            key={g.id}
            goal={g}
            goalTasks={allTasks
              .filter((t) => t.goal_id === g.id)
              .sort(byGoalOrder)}
            onEdit={(goal) => setModal({ goal })}
            onStatusChange={handleStatusChange}
            onAddTask={(goal) => setTaskModalGoal(goal)}
            onAttach={(goal) => setAttachGoal(goal)}
            onUnlink={handleUnlink}
            onOpenTask={(task) => setEditTask(task)}
            onMoveTask={handleMoveTask}
          />
        ))
      )}

      {modal && (
        <GoalFormModal
          initial={modal.goal}
          // Сколько задач привязано — нужно для опции «удалить вместе с целью».
          taskCount={
            modal.goal
              ? allTasks.filter((t) => t.goal_id === modal.goal.id).length
              : 0
          }
          onClose={() => setModal(null)}
          // При создании цели с черновиками задач сохраняются и задачи —
          // обновляем оба кэша, иначе задачи не появятся до перезагрузки.
          onSaved={() => {
            refreshGoals();
            refreshTasks();
          }}
        />
      )}

      {taskModalGoal && (
        <TaskFormModal
          categories={categories}
          goals={goals}
          presetGoalId={taskModalGoal.id}
          onClose={() => setTaskModalGoal(null)}
          onSaved={refreshAll}
        />
      )}

      {attachGoal && (
        <AttachTasksModal
          goal={attachGoal}
          tasks={allTasks}
          onClose={() => setAttachGoal(null)}
          onSaved={refreshAll}
        />
      )}

      {editTask && (
        <TaskFormModal
          initial={editTask}
          categories={categories}
          goals={goals}
          onClose={() => setEditTask(null)}
          onSaved={refreshAll}
          onDeleted={refreshAll}
        />
      )}
    </section>
  );
}

export default Goals;
