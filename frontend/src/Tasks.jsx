import { useMemo, useState } from "react";
import { useTasks, createTask, updateTask, deleteTask, setDayItemSpent } from "./api.js";
import { useQueryClient } from "@tanstack/react-query";
import DateDisplay from "@/components/DateDisplay.jsx";
import DateInput from "@/components/DateInput.jsx";
import { todayStr } from "@/lib/formatDate.js";
import { cn } from "@/lib/utils";
import ModalClose from "@/components/ModalClose.jsx";

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
// Фактическое время здесь не передаётся: оно складывается из фактов по дням
// (раздел «День») и доступно только для чтения (actual_hours в ответе).
export function taskPayload(t, extra = {}) {
  return {
    category: t.category || "Прочее",
    title: t.title,
    description: t.description || "",
    planned_hours: Number(t.planned_hours) || 0,
    deadline: t.deadline || "",
    status: t.status || "todo",
    goal_id: t.goal_id != null ? t.goal_id : null,
    // За какой день задача выполнена (ГГГГ-ММ-ДД); пусто — сервер возьмёт сегодня.
    completed_date: t.completed_date || "",
    ...extra,
  };
}

// Часы: 2 -> "2", 2.5 -> "2.5", пусто/0 -> "0".
export const fmtHours = (h) => {
  const n = Number(h || 0);
  return Number.isInteger(n) ? String(n) : String(Math.round(n * 100) / 100);
};

// Минуты: 65 -> "1 ч 5 мин" (для разбивки потраченного времени по дням).
const fmtMin = (m) => {
  if (!m) return "0 мин";
  if (m < 60) return `${m} мин`;
  const h = Math.floor(m / 60);
  const r = m % 60;
  return r ? `${h} ч ${r} мин` : `${h} ч`;
};

// Подсказка для «Факт, ч»: разбивка потраченного времени по дням.
const spentTitle = (t) => {
  const days = Array.isArray(t.spent_by_day) ? t.spent_by_day : [];
  if (days.length === 0) return "";
  return days.map((d) => `${d.date}: ${fmtMin(d.minutes)}`).join("\n");
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
// Удаление задачи живёт здесь же: отдельной кнопки в списке нет.
export function TaskFormModal({
  initial,
  categories,
  goals = [],
  presetGoalId,
  onClose,
  onSaved,
  onDeleted,
}) {
  const [category, setCategory] = useState(initial?.category || "");
  const [title, setTitle] = useState(initial?.title || "");
  const [description, setDescription] = useState(initial?.description || "");
  const [plannedHours, setPlannedHours] = useState(
    initial ? fmtHours(initial.planned_hours) : "",
  );
  const [deadline, setDeadline] = useState(initial?.deadline || "");
  const [status, setStatus] = useState(initial?.status || "todo");
  // Дата выполнения: по умолчанию сегодня; пользователь может указать прошлый
  // день, чтобы отметить забытую задачу задним числом.
  const [completedDate, setCompletedDate] = useState(
    initial?.completed_date || todayStr(),
  );
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
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState("");

  // Потраченное время: итог и разбивка по дням — только для чтения.
  // Факт «за сегодня» можно задать прямо здесь: он пишется в позицию
  // сегодняшнего дня, и задача при необходимости автоматически попадает
  // в «План дня» (см. «День»).
  const spentByDay = Array.isArray(initial?.spent_by_day)
    ? initial.spent_by_day
    : [];
  const today = todayStr();
  const serverTodayMinutes =
    spentByDay.find((d) => d.date === today)?.minutes || 0;
  const [spentToday, setSpentToday] = useState(
    serverTodayMinutes > 0 ? fmtHours(serverTodayMinutes / 60) : "",
  );

  // Живой итог: «старая база» без разбивки + прочие дни + то, что ввели сейчас.
  const enteredMinutes = (() => {
    const h = Number(String(spentToday).replace(",", "."));
    return Number.isFinite(h) && h > 0 ? Math.min(1440, Math.round(h * 60)) : 0;
  })();
  const otherDays = spentByDay.filter((d) => d.date !== today);
  const otherDaysMinutes = otherDays.reduce((s, d) => s + d.minutes, 0);
  const daysMinutes = spentByDay.reduce((s, d) => s + d.minutes, 0);
  const legacyMinutes = Math.max(
    0,
    Math.round((Number(initial?.actual_hours) || 0) * 60) - daysMinutes,
  );
  const spentTotalMinutes = legacyMinutes + otherDaysMinutes + enteredMinutes;

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
      deadline,
      status,
      goal_id: goalKey === "none" ? null : Number(goalKey),
      completed_date: status === "done" ? completedDate : "",
    };
    try {
      const saved = initial
        ? await updateTask(initial.id, payload)
        : await createTask(payload);
      // Факт за сегодня пишем в позицию дня. Если задачи в плане дня не было,
      // сервер добавит её (и сам план) автоматически.
      if (saved?.id != null && enteredMinutes !== serverTodayMinutes) {
        await setDayItemSpent(today, "task", saved.id, enteredMinutes);
      }
      onSaved(saved);
      onClose();
    } catch (err) {
      setError(err.message || "Не удалось сохранить задачу");
      setSaving(false);
    }
  };

  // Удаление доступно только в режиме редактирования (нечего удалять в новой).
  const handleDelete = async () => {
    if (!window.confirm(`Удалить задачу «${initial.title}»?`)) return;
    setDeleting(true);
    setError("");
    try {
      await deleteTask(initial.id);
      (onDeleted || onSaved)?.();
      onClose();
    } catch (err) {
      setError(err.message || "Не удалось удалить задачу");
      setDeleting(false);
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      onClick={onClose}
    >
      <Card
        className="relative flex max-h-[85vh] w-full max-w-lg flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        <ModalClose onClose={onClose} />
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <ListTodo className="size-4 text-muted-foreground" />
            {initial ? "Редактировать задачу" : "Новая задача"}
          </CardTitle>
          <CardDescription>
            Категория, планируемое время в часах, дедлайн и статус. Потраченное
            время складывается из фактов по дням в разделе «День».
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
              <Label>Фактическое время сегодня, ч</Label>
              <Input
                type="number"
                min="0"
                step="0.25"
                value={spentToday}
                onChange={(e) => setSpentToday(e.target.value)}
                placeholder="0"
              />
            </div>
          </div>

          {initial && (
            <div className="space-y-1">
              <div className="flex items-center justify-between gap-3 text-sm">
                <span className="text-muted-foreground">Потрачено всего</span>
                <span className="font-medium tabular-nums">
                  {spentTotalMinutes > 0
                    ? fmtMin(spentTotalMinutes)
                    : "—"}
                </span>
              </div>
              {(otherDays.length > 0 || legacyMinutes > 0) && (
                <ul className="space-y-0.5 text-xs text-muted-foreground">
                  {otherDays.map((d) => (
                    <li
                      key={d.date}
                      className="flex items-center justify-between gap-3"
                    >
                      <DateDisplay date={d.date} />
                      <span className="tabular-nums">{fmtMin(d.minutes)}</span>
                    </li>
                  ))}
                  {legacyMinutes > 0 && (
                    <li className="flex items-center justify-between gap-3">
                      <span>Без разбивки по дням</span>
                      <span className="tabular-nums">
                        {fmtMin(legacyMinutes)}
                      </span>
                    </li>
                  )}
                </ul>
              )}
              <p className="text-xs text-muted-foreground">
                Время за сегодня можно указать здесь — задача попадёт в «План
                дня». Остальные дни — в разделе «День».
              </p>
            </div>
          )}

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

          {status === "done" && (
            <div className="space-y-1.5">
              <Label>Дата выполнения</Label>
              <DateInput value={completedDate} onChange={setCompletedDate} />
              <p className="text-xs text-muted-foreground">
                За какой день задача выполнена — можно указать прошлый день,
                если забыли отметить вовремя.
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

        <div className="flex flex-wrap items-center gap-2 border-t p-4">
          {initial && (
            <Button
              variant="destructive"
              onClick={handleDelete}
              disabled={saving || deleting}
            >
              {deleting ? <Loader2 className="animate-spin" /> : <Trash2 />}
              {deleting ? "Удаляю…" : "Удалить задачу"}
            </Button>
          )}
          <div className="ml-auto flex flex-wrap items-center gap-2">
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

// Строка таблицы задачи (desktop). Открывается по клику на любую ячейку;
// селект статуса клик не «проглатывает».
function TaskRow({ task, goals = [], onEdit, onStatusChange }) {
  const [changing, setChanging] = useState(false);
  const goal = task.goal_id != null ? goals.find((g) => g.id === task.goal_id) : null;
  // Выполненные задачи — в конце списка и серые, как неактивные.
  const done = task.status === "done";

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
      className={cn(
        "cursor-pointer border-b border-border/60 last:border-0 hover:bg-muted/30",
        done && "text-muted-foreground opacity-70",
      )}
    >
      <td className="min-w-0 px-3 py-2 align-top">
        <p
          className={cn(
            "wrap-break-word font-medium",
            done ? "text-muted-foreground" : "text-foreground",
          )}
        >
          {task.title}
        </p>
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
      <td className="px-1.5 py-2 align-top tabular-nums" title={spentTitle(task)}>
        {fmtHours(task.actual_hours)}
      </td>
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
    </tr>
  );
}

// Карточка задачи (mobile). Открывается по клику на содержимое;
// селект статуса работает сам по себе.
function TaskCard({ task, goals = [], onEdit, onStatusChange }) {
  const [changing, setChanging] = useState(false);
  const goal = task.goal_id != null ? goals.find((g) => g.id === task.goal_id) : null;
  // Выполненные задачи — в конце списка и серые, как неактивные.
  const done = task.status === "done";

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
      className={cn(
        "my-3 cursor-pointer",
        done && "text-muted-foreground opacity-70",
      )}
      size="sm"
      onClick={handleCardClick}
      title="Открыть задачу"
    >
      <CardContent className="space-y-3 pt-4">
        <div className="min-w-0">
          <p
            className={cn(
              "font-medium",
              done ? "text-muted-foreground" : "text-foreground",
            )}
          >
            {task.title}
          </p>
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
            <span
              className="font-medium tabular-nums text-foreground"
              title={spentTitle(task)}
            >
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

  // Фильтрация по категории и статусу. Выполненные — в конце списка
  // (сортировка стабильная, внутри групп порядок сохраняется).
  const filtered = useMemo(
    () =>
      tasks
        .filter(
          (t) =>
            (catFilter === "all" || t.category === catFilter) &&
            (statusFilter === "all" || t.status === statusFilter),
        )
        .sort(
          (a, b) =>
            (a.status === "done" ? 1 : 0) - (b.status === "done" ? 1 : 0),
        ),
    [tasks, catFilter, statusFilter],
  );

  const handleStatusChange = async (task, status) => {
    // При закрытии задачи проставляем дату выполнения (сегодняшнюю локальную).
    const extra = { status };
    if (status === "done") extra.completed_date = todayStr();
    await updateTask(task.id, taskPayload(task, extra));
    refresh();
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
                <col className="w-[45%]" />
                <col className="w-[12%]" />
                <col className="w-[8%]" />
                <col className="w-[8%]" />
                <col className="w-[12%]" />
                <col className="w-[15%]" />
              </colgroup>
              <thead>
                <tr className="border-b bg-muted/40 text-left text-[11px] uppercase tracking-wide text-muted-foreground">
                  <th className="px-3 py-2 align-middle font-bold">Название</th>
                  <th className="px-1.5 py-2 align-middle font-bold">Категория</th>
                  <th className="px-1.5 py-2 align-middle font-bold">План, ч</th>
                  <th className="px-1.5 py-2 align-middle font-bold">Факт, ч</th>
                  <th className="px-1.5 py-2 align-middle font-bold">Дедлайн</th>
                  <th className="px-1.5 py-2 align-middle font-bold">Статус</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((t) => (
                  <TaskRow
                    key={t.id}
                    task={t}
                    goals={goals}
                    onEdit={(task) => setModal({ task })}
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
          onDeleted={refresh}
        />
      )}
    </section>
  );
}

export default Tasks;
