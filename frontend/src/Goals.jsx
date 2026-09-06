import { useState } from "react";
import { useGoals, createGoal, updateGoal, deleteGoal } from "./api.js";
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
  const [progress, setProgress] = useState(
    initial ? String(initial.progress ?? 0) : "0",
  );
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const num = (v) => {
    const n = Number(v);
    if (!Number.isFinite(n)) return 0;
    return Math.max(0, Math.min(100, Math.round(n)));
  };

  const handleSave = async () => {
    setSaving(true);
    setError("");
    const payload = {
      title,
      description,
      target_date: targetDate,
      status,
      progress: num(progress),
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
          <CardDescription>Дедлайн, статус и прогресс 0–100%</CardDescription>
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

          <div className="space-y-1.5">
            <Label>Прогресс, %</Label>
            <div className="flex items-center gap-3">
              <Input
                type="number"
                min="0"
                max="100"
                value={progress}
                onChange={(e) => setProgress(e.target.value)}
                className="w-24"
              />
              <Progress value={num(progress)} className="flex-1" />
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

// Карточка цели.
function GoalCard({ goal, onEdit, onDelete, onStatusChange }) {
  const meta = statusMeta(goal.status);
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

        <div className="flex items-center gap-3">
          <Progress value={goal.progress ?? 0} className="flex-1" />
          <span className="text-sm font-medium tabular-nums">
            {goal.progress ?? 0}%
          </span>
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
      </CardContent>
    </Card>
  );
}

// Раздел «Цели»: список целей с прогрессом + добавление/редактирование.
function Goals() {
  const queryClient = useQueryClient();
  const goalsQuery = useGoals(true);
  const [modal, setModal] = useState(null);

  const refresh = () => queryClient.invalidateQueries({ queryKey: ["goals"] });

  const handleStatusChange = async (goal, status) => {
    try {
      await updateGoal(goal.id, {
        title: goal.title,
        description: goal.description,
        target_date: goal.target_date,
        status,
        progress: goal.progress ?? 0,
      });
      refresh();
    } catch (err) {
      window.alert(err.message || "Не удалось изменить статус");
    }
  };

  const handleDelete = async (goal) => {
    if (!window.confirm(`Удалить цель «${goal.title}»?`)) return;
    try {
      await deleteGoal(goal.id);
      refresh();
    } catch (err) {
      window.alert(err.message || "Не удалось удалить цель");
    }
  };

  if (goalsQuery.isLoading) {
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

  const items = goalsQuery.data?.goals || [];

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
            onEdit={(goal) => setModal({ goal })}
            onDelete={handleDelete}
            onStatusChange={handleStatusChange}
          />
        ))
      )}

      {modal && (
        <GoalFormModal
          initial={modal.goal}
          onClose={() => setModal(null)}
          onSaved={refresh}
        />
      )}
    </section>
  );
}

export default Goals;
