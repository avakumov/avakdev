import { useMemo, useState } from "react";
import { useTasks, createTask, updateTask, deleteTask } from "./api.js";
import { useQueryClient } from "@tanstack/react-query";
import DateDisplay from "@/components/DateDisplay.jsx";
import DateInput from "@/components/DateInput.jsx";

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
  ListTodo,
  Plus,
  Save,
  Trash2,
  Loader2,
  AlertCircle,
  X,
  Clock,
  CalendarDays,
  Hourglass,
  Filter,
  Target,
} from "lucide-react";

// Статусы задач: метка + вариант бейджа.
const STATUSES = [
  { value: "todo", label: "Новая" },
  { value: "in_progress", label: "В работе" },
  { value: "done", label: "Готова" },
  { value: "cancelled", label: "Отменена" },
];

const STATUS_META = {
  todo: { label: "Новая", variant: "outline" },
  in_progress: { label: "В работе", variant: "default" },
  done: { label: "Готова", variant: "secondary" },
  cancelled: { label: "Отменена", variant: "outline" },
};

export const statusLabel = (s) => STATUS_META[s]?.label || s;
export const statusVariant = (s) => STATUS_META[s]?.variant || "outline";

// Базовый payload задачи для PUT/POST /api/tasks.
// extra позволяет переопределить отдельные поля (например, статус или goal_id).
export function taskPayload(t, extra = {}) {
  return {
    category: t.category || "Прочее",
    title: t.title,
    description: t.description || "",
    planned_hours: Number(t.planned_hours) || 0,
    actual_hours: Number(t.actual_hours) || 0,
    deadline: t.deadline || "",
    status: t.status || "todo",
    goal_id: t.goal_id != null ? t.goal_id : null,
    ...extra,
  };
}

// Часы: 2 -> "2", 2.5 -> "2.5", пусто/0 -> "0".
const fmtHours = (h) => {
  const n = Number(h || 0);
  return Number.isInteger(n) ? String(n) : String(Math.round(n * 100) / 100);
};

// Перенос по словам (стандартное поведение) — см. ячейку названия в TaskRow.

// Строка-обёртка над обычным textarea (в стилистике shadcn/ui).
function Textarea({ className, ...props }) {
  return (
    <textarea
      data-slot="textarea"
      className={
        "w-full min-h-24 rounded-lg border border-input bg-transparent px-3 py-2 text-sm leading-relaxed text-foreground transition-colors outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-input/30 resize-y " +
        (className || "")
      }
      {...props}
    />
  );
}

// Модальное окно с формой задачи (создание или редактирование).
// goals — цели пользователя (из /api/tasks); presetGoalId — цель,
// с которой создаётся задача сразу (кнопка «Задача» в карточке цели).
export function TaskFormModal({
  initial,
  categories,
  goals = [],
  presetGoalId,
  onClose,
  onSaved,
}) {
  const [category, setCategory] = useState(initial?.category || "");
  const [title, setTitle] = useState(initial?.title || "");
  const [description, setDescription] = useState(initial?.description || "");
  const [plannedHours, setPlannedHours] = useState(
    initial ? fmtHours(initial.planned_hours) : "",
  );
  const [actualHours, setActualHours] = useState(
    initial ? fmtHours(initial.actual_hours) : "",
  );
  const [deadline, setDeadline] = useState(initial?.deadline || "");
  const [status, setStatus] = useState(initial?.status || "todo");
  // Ключ цели в Select: "none" — без цели, иначе строковый id.
  // Если указанная цель пропала (например, удалена), сбрасываем на «Без цели».
  const [goalKey, setGoalKey] = useState(() => {
    const exists = (id) => goals.some((g) => g.id === id);
    if (initial?.goal_id != null && exists(initial.goal_id)) {
      return String(initial.goal_id);
    }
    if (presetGoalId != null && exists(presetGoalId)) {
      return String(presetGoalId);
    }
    return "none";
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const num = (v) => {
    const n = Number(v);
    return Number.isFinite(n) && n >= 0 ? n : 0;
  };

  const handleSave = async () => {
    setSaving(true);
    setError("");
    const payload = {
      category: category || "Прочее",
      title,
      description,
      planned_hours: num(plannedHours),
      actual_hours: num(actualHours),
      deadline,
      status,
      goal_id: goalKey === "none" ? null : Number(goalKey),
    };
    try {
      const saved = initial
        ? await updateTask(initial.id, payload)
        : await createTask(payload);
      onSaved(saved);
      onClose();
    } catch (err) {
      setError(err.message || "Не удалось сохранить задачу");
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
            <ListTodo className="size-4 text-muted-foreground" />
            {initial ? "Редактировать задачу" : "Новая задача"}
          </CardTitle>
          <CardDescription>
            Категория, планируемое и фактическое время в часах, дедлайн и статус.
          </CardDescription>
        </CardHeader>

        <CardContent className="flex-1 space-y-4 overflow-y-auto">
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label>Категория</Label>
              <Select value={category} onValueChange={setCategory}>
                <SelectTrigger className="w-full">
                  <SelectValue placeholder="Выберите категорию" />
                </SelectTrigger>
                <SelectContent>
                  {categories.map((c) => (
                    <SelectItem key={c} value={c}>
                      {c}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
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

          <div className="space-y-1.5">
            <Label>Название</Label>
            <Input
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Например: сверстать страницу контактов"
            />
          </div>

          <div className="space-y-1.5">
            <Label>Описание</Label>
            <Textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Подробности (необязательно)…"
            />
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label>Планируемое время, ч</Label>
              <Input
                type="number"
                min="0"
                step="0.5"
                value={plannedHours}
                onChange={(e) => setPlannedHours(e.target.value)}
                placeholder="0"
              />
            </div>
            <div className="space-y-1.5">
              <Label>Фактическое время, ч</Label>
              <Input
                type="number"
                min="0"
                step="0.5"
                value={actualHours}
                onChange={(e) => setActualHours(e.target.value)}
                placeholder="0"
              />
            </div>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label>Цель (необязательно)</Label>
              <Select value={goalKey} onValueChange={setGoalKey}>
                <SelectTrigger className="w-full">
                  <SelectValue placeholder="Без цели" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">Без цели</SelectItem>
                  {goals.map((g) => (
                    <SelectItem key={g.id} value={String(g.id)}>
                      {g.title}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label>Дедлайн</Label>
              <DateInput value={deadline} onChange={setDeadline} />
            </div>
          </div>

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
          <Button variant="ghost" onClick={onClose} disabled={saving}>
            <X />
            Отмена
          </Button>
          <Button
            onClick={handleSave}
            disabled={saving || !title.trim()}
          >
            {saving ? <Loader2 className="animate-spin" /> : <Save />}
            {saving ? "Сохраняю…" : "Сохранить"}
          </Button>
        </div>
      </Card>
    </div>
  );
}

// Строка таблицы задачи (desktop). Открывается по клику; элементы управления
// (кнопка удаления, селект статуса) клик не «проглатывают».
function TaskRow({ task, goals = [], onEdit, onDelete, onStatusChange }) {
  const [changing, setChanging] = useState(false);
  const goal = task.goal_id != null ? goals.find((g) => g.id === task.goal_id) : null;

  const handleRowClick = (e) => {
    if (e.target.closest("button, a, input, [role='combobox']")) return;
    onEdit(task);
  };

  const handleStatus = async (status) => {
    setChanging(true);
    try {
      await onStatusChange(status);
    } finally {
      setChanging(false);
    }
  };

  return (
    <tr
      onClick={handleRowClick}
      title="Открыть задачу"
      className="cursor-pointer border-b border-border/60 last:border-0 hover:bg-muted/30"
    >
      <td className="min-w-0 px-3 py-2 align-top">
        <p className="wrap-break-word font-medium text-foreground">{task.title}</p>
        {task.description && (
          <p className="wrap-break-word text-xs text-muted-foreground">
            {task.description}
          </p>
        )}
        {goal && (
          <p className="mt-1 flex items-center gap-1 text-xs text-muted-foreground">
            <Target className="size-3 shrink-0" />
            <span className="wrap-break-word">{goal.title}</span>
          </p>
        )}
      </td>
      <td className="px-1.5 py-2 align-top">
        <Badge variant="outline">{task.category || "Прочее"}</Badge>
      </td>
      <td className="px-1.5 py-2 align-top tabular-nums">{fmtHours(task.planned_hours)}</td>
      <td className="px-1.5 py-2 align-top tabular-nums">{fmtHours(task.actual_hours)}</td>
      <td className="px-1.5 py-2 align-top whitespace-nowrap text-muted-foreground">
        {task.deadline ? <DateDisplay date={task.deadline} /> : "—"}
      </td>
      <td className="px-1.5 py-2 align-top">
        <Select
          value={task.status}
          onValueChange={handleStatus}
          disabled={changing}
        >
          <SelectTrigger size="sm" className="w-full">
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
      </td>
      <td className="px-2 py-2 align-top text-right">
        <Button
          variant="destructive"
          size="icon-sm"
          onClick={() => onDelete(task)}
        >
          <Trash2 />
        </Button>
      </td>
    </tr>
  );
}

// Карточка задачи (mobile). Открывается по клику на содержимое;
// кнопка удаления и селект статуса работают сами по себе.
function TaskCard({ task, goals = [], onEdit, onDelete, onStatusChange }) {
  const [changing, setChanging] = useState(false);
  const goal = task.goal_id != null ? goals.find((g) => g.id === task.goal_id) : null;

  const handleCardClick = (e) => {
    if (e.target.closest("button, a, input, [role='combobox']")) return;
    onEdit(task);
  };

  const handleStatus = async (status) => {
    setChanging(true);
    try {
      await onStatusChange(status);
    } finally {
      setChanging(false);
    }
  };

  return (
    <Card
      className="my-3 cursor-pointer"
      size="sm"
      onClick={handleCardClick}
      title="Открыть задачу"
    >
      <CardContent className="space-y-3 pt-4">
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0">
            <p className="font-medium text-foreground">{task.title}</p>
            <div className="mt-1 flex flex-wrap items-center gap-1.5">
              <Badge variant="outline">{task.category || "Прочее"}</Badge>
              <Badge variant={statusVariant(task.status)}>
                {statusLabel(task.status)}
              </Badge>
              {goal && (
                <Badge variant="outline">
                  <Target className="mr-1 size-3" />
                  {goal.title}
                </Badge>
              )}
            </div>
          </div>
          <div className="flex shrink-0 gap-1">
            <Button
              variant="destructive"
              size="sm"
              onClick={() => onDelete(task)}
            >
              <Trash2 />
            </Button>
          </div>
        </div>

        {task.description && (
          <p className="whitespace-pre-wrap text-sm leading-relaxed text-muted-foreground">
            {task.description}
          </p>
        )}

        <div className="grid grid-cols-2 gap-2 text-sm">
          <div className="flex items-center gap-1.5 text-muted-foreground">
            <Hourglass className="size-3.5" />
            План:{" "}
            <span className="font-medium tabular-nums text-foreground">
              {fmtHours(task.planned_hours)} ч
            </span>
          </div>
          <div className="flex items-center gap-1.5 text-muted-foreground">
            <Clock className="size-3.5" />
            Факт:{" "}
            <span className="font-medium tabular-nums text-foreground">
              {fmtHours(task.actual_hours)} ч
            </span>
          </div>
          <div className="flex items-center gap-1.5 text-muted-foreground">
            <CalendarDays className="size-3.5" />
            {task.deadline ? <DateDisplay date={task.deadline} /> : "Без дедлайна"}
          </div>
        </div>

        <div className="flex items-center justify-between gap-2 border-t pt-3">
          <Select
            value={task.status}
            onValueChange={handleStatus}
            disabled={changing}
          >
            <SelectTrigger className="h-8 w-36">
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
          {task.updated && (
            <span className="flex items-center gap-1 text-xs text-muted-foreground">
              <Clock className="size-3" />
              <DateDisplay date={task.updated} />
            </span>
          )}
        </div>
      </CardContent>
    </Card>
  );
}

// Главный компонент: раздел «Задачи» — категории, планируемое и фактическое
// время, дедлайн, статус. Таблица на десктопе, карточки на мобильных,
// фильтры по категории и статусу.
function Tasks() {
  const queryClient = useQueryClient();
  const tasksQuery = useTasks(true);

  const [catFilter, setCatFilter] = useState("all");
  const [statusFilter, setStatusFilter] = useState("all");
  const [modal, setModal] = useState(null); // null | { task: null } | { task }

  const data = tasksQuery.data;
  const tasks = data?.tasks || [];
  const categories = data?.categories || [];
  const goals = data?.goals || [];

  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: ["tasks"] });

  // Фильтрация по категории и статусу.
  const filtered = useMemo(
    () =>
      tasks.filter(
        (t) =>
          (catFilter === "all" || t.category === catFilter) &&
          (statusFilter === "all" || t.status === statusFilter),
      ),
    [tasks, catFilter, statusFilter],
  );

  const handleStatusChange = async (task, status) => {
    await updateTask(task.id, taskPayload(task, { status }));
    refresh();
  };

  const handleDelete = async (task) => {
    if (!window.confirm(`Удалить задачу «${task.title}»?`)) return;
    try {
      await deleteTask(task.id);
      refresh();
    } catch (err) {
      window.alert(err.message || "Не удалось удалить задачу");
    }
  };

  if (tasksQuery.isLoading) {
    return (
      <p className="text-sm text-muted-foreground">
        <Loader2 className="mr-1 inline size-4 animate-spin" />
        Загрузка задач…
      </p>
    );
  }
  if (tasksQuery.isError) {
    return (
      <p className="text-sm text-destructive">
        Ошибка: {tasksQuery.error?.message}
      </p>
    );
  }

  return (
    <section>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="flex items-center gap-2 text-xl font-semibold">
          <ListTodo className="size-5 text-muted-foreground" />
          Задачи
        </h2>
        <Button onClick={() => setModal({ task: null })}>
          <Plus />
          Новая задача
        </Button>
      </div>

      {/* Фильтры по категории и статусу */}
      <Card className="my-3" size="sm">
        <CardContent className="flex flex-wrap items-center gap-3 pt-4">
          <span className="flex items-center gap-1.5 text-sm text-muted-foreground">
            <Filter className="size-4" />
            Фильтры:
          </span>
          <Select value={catFilter} onValueChange={setCatFilter}>
            <SelectTrigger className="w-40">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">Все категории</SelectItem>
              {categories.map((c) => (
                <SelectItem key={c} value={c}>
                  {c}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select value={statusFilter} onValueChange={setStatusFilter}>
            <SelectTrigger className="w-40">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">Все статусы</SelectItem>
              {STATUSES.map((s) => (
                <SelectItem key={s.value} value={s.value}>
                  {s.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <span className="text-sm text-muted-foreground">
            Найдено: {filtered.length} из {tasks.length}
          </span>
        </CardContent>
      </Card>

      {tasks.length === 0 ? (
        <Card className="my-3" size="sm">
          <CardContent>
            <p className="flex items-center gap-2 text-sm text-muted-foreground">
              <ListTodo className="size-4" />
              Пока нет ни одной задачи. Добавьте первую.
            </p>
          </CardContent>
        </Card>
      ) : filtered.length === 0 ? (
        <Card className="my-3" size="sm">
          <CardContent>
            <p className="text-sm text-muted-foreground">
              Нет задач, подходящих под выбранные фильтры.
            </p>
          </CardContent>
        </Card>
      ) : (
        <>
          {/* Таблица — desktop (md и шире). Без горизонтального скролла:
              table-fixed + фиксированные ширины колонок; длинные названия
              переносятся по словам (wrap-break-word). */}
          <Card className="my-3 hidden overflow-hidden md:block" size="sm">
            <table className="w-full table-fixed text-sm">
              <colgroup>
                <col className="w-[40%]" />
                <col className="w-[10%]" />
                <col className="w-[7%]" />
                <col className="w-[7%]" />
                <col className="w-[11%]" />
                <col className="w-[14%]" />
                <col className="w-[11%]" />
              </colgroup>
              <thead>
                <tr className="border-b bg-muted/40 text-left text-[11px] uppercase tracking-wide text-muted-foreground">
                  <th className="px-3 py-2 align-middle font-bold">Название</th>
                  <th className="px-1.5 py-2 align-middle font-bold">Категория</th>
                  <th className="px-1.5 py-2 align-middle font-bold">План, ч</th>
                  <th className="px-1.5 py-2 align-middle font-bold">Факт, ч</th>
                  <th className="px-1.5 py-2 align-middle font-bold">Дедлайн</th>
                  <th className="px-1.5 py-2 align-middle font-bold">Статус</th>
                  <th className="px-2 py-2 text-right align-middle font-bold">
                    Удалить
                  </th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((t) => (
                  <TaskRow
                    key={t.id}
                    task={t}
                    goals={goals}
                    onEdit={(task) => setModal({ task })}
                    onDelete={handleDelete}
                    onStatusChange={(status) => handleStatusChange(t, status)}
                  />
                ))}
              </tbody>
            </table>
          </Card>

          {/* Карточки — mobile (< md) */}
          <div className="md:hidden">
            {filtered.map((t) => (
              <TaskCard
                key={t.id}
                task={t}
                goals={goals}
                onEdit={(task) => setModal({ task })}
                onDelete={handleDelete}
                onStatusChange={(status) => handleStatusChange(t, status)}
              />
            ))}
          </div>
        </>
      )}

      {modal && (
        <TaskFormModal
          initial={modal.task}
          categories={categories}
          goals={goals}
          onClose={() => setModal(null)}
          onSaved={refresh}
        />
      )}
    </section>
  );
}

export default Tasks;
