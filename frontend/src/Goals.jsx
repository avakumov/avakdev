import { useState } from "react";
import {
  useGoals,
  createGoal,
  updateGoal,
  deleteGoal,
  useTasks,
  updateTask,
} from "./api.js";
import { TaskFormModal, taskPayload, statusLabel, statusVariant } from "./Tasks.jsx";
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
} from "lucide-react";

// Статусы целей.
const STATUSES = [
  { value: "active", label: "В работе", variant: "default" },
  { value: "paused", label: "Приостановлена", variant: "outline" },
  { value: "achieved", label: "Достигнута", variant: "secondary" },
  { value: "cancelled", label: "Отменена", variant: "destructive" },
];

const statusMeta = (s) => STATUSES.find((x) => x.value === s) || STATUSES[0];

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
function GoalFormModal({ initial, onClose, onSaved }) {
  const [title, setTitle] = useState(initial?.title || "");
  const [description, setDescription] = useState(initial?.description || "");
  const [targetDate, setTargetDate] = useState(initial?.target_date || "");
  const [status, setStatus] = useState(initial?.status || "active");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const handleSave = async () => {
    setSaving(true);
    setError("");
    const payload = {
      title,
      description,
      target_date: targetDate,
      status,
    };
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
  onDelete,
  onStatusChange,
  onAddTask,
  onAttach,
  onUnlink,
}) {
  const meta = statusMeta(goal.status);
  // Прогресс цели считается на сервере по задачам; здесь — подпись для бара.
  const activeTasks = goalTasks.filter((t) => t.status !== "cancelled");
  const doneTasks = activeTasks.filter((t) => t.status === "done");
  return (
    <Card className="my-3" size="sm">
      <CardContent className="space-y-3 pt-4">
        <div className="flex items-start justify-between gap-2">
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
          <div className="flex shrink-0 gap-1">
            <Button variant="outline" size="icon-sm" onClick={() => onEdit(goal)}>
              <Edit />
            </Button>
            <Button
              variant="destructive"
              size="icon-sm"
              onClick={() => onDelete(goal)}
            >
              <Trash2 />
            </Button>
          </div>
        </div>

        {goal.description && (
          <p className="whitespace-pre-wrap text-sm leading-relaxed text-muted-foreground">
            {goal.description}
          </p>
        )}

        <div className="space-y-1">
          <div className="flex items-center gap-3">
            <Progress value={goal.progress ?? 0} className="flex-1" />
            <span className="text-sm font-medium tabular-nums">
              {goal.progress ?? 0}%
            </span>
          </div>
          {activeTasks.length > 0 ? (
            <p className="text-xs text-muted-foreground">
              Выполнено {doneTasks.length} из {activeTasks.length} задач
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

        <div className="border-t pt-3">
          <Select value={goal.status} onValueChange={(s) => onStatusChange(goal, s)}>
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

          {goalTasks.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              Задач пока нет — добавьте новую или привяжите существующую.
            </p>
          ) : (
            <ul className="space-y-1.5">
              {goalTasks.map((t) => (
                <li
                  key={t.id}
                  className="flex items-start justify-between gap-2 rounded-lg border border-border/60 px-2.5 py-1.5"
                >
                  <div className="min-w-0">
                    <p className="wrap-break-word text-sm font-medium text-foreground">
                      {t.title}
                    </p>
                    <div className="mt-0.5 flex flex-wrap items-center gap-1.5">
                      <Badge variant={statusVariant(t.status)}>
                        {statusLabel(t.status)}
                      </Badge>
                      <span className="text-xs text-muted-foreground">
                        {t.category || "Прочее"}
                        {Number(t.planned_hours) > 0 &&
                          ` · План: ${t.planned_hours} ч`}
                      </span>
                    </div>
                  </div>
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
              ))}
            </ul>
          )}
        </div>
      </CardContent>
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

  const refreshGoals = () =>
    queryClient.invalidateQueries({ queryKey: ["goals"] });
  const refreshTasks = () =>
    queryClient.invalidateQueries({ queryKey: ["tasks"] });

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

  const handleDelete = async (goal) => {
    if (!window.confirm(`Удалить цель «${goal.title}»?`)) return;
    try {
      await deleteGoal(goal.id);
      refreshGoals();
      refreshTasks(); // ссылки задач на удалённую цель сброшены на сервере
    } catch (err) {
      window.alert(err.message || "Не удалось удалить цель");
    }
  };

  const handleUnlink = async (task) => {
    if (!window.confirm(`Отвязать задачу «${task.title}» от цели?`)) return;
    try {
      await updateTask(task.id, taskPayload(task, { goal_id: null }));
      refreshTasks();
    } catch (err) {
      window.alert(err.message || "Не удалось отвязать задачу");
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
            goalTasks={allTasks.filter((t) => t.goal_id === g.id)}
            onEdit={(goal) => setModal({ goal })}
            onDelete={handleDelete}
            onStatusChange={handleStatusChange}
            onAddTask={(goal) => setTaskModalGoal(goal)}
            onAttach={(goal) => setAttachGoal(goal)}
            onUnlink={handleUnlink}
          />
        ))
      )}

      {modal && (
        <GoalFormModal
          initial={modal.goal}
          onClose={() => setModal(null)}
          onSaved={refreshGoals}
        />
      )}

      {taskModalGoal && (
        <TaskFormModal
          categories={categories}
          goals={goals}
          presetGoalId={taskModalGoal.id}
          onClose={() => setTaskModalGoal(null)}
          onSaved={refreshTasks}
        />
      )}

      {attachGoal && (
        <AttachTasksModal
          goal={attachGoal}
          tasks={allTasks}
          onClose={() => setAttachGoal(null)}
          onSaved={refreshTasks}
        />
      )}
    </section>
  );
}

export default Goals;
