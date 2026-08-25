import { useState } from "react";
import {
  useAppTasks,
  createAppTask,
  updateAppTask,
  deleteAppTask,
  requestTaskDeploy,
  requestTaskRollback,
} from "./api.js";
import { useQueryClient } from "@tanstack/react-query";
import DateDisplay from "@/components/DateDisplay.jsx";

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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Plus,
  Save,
  Trash2,
  Loader2,
  AlertCircle,
  Wrench,
  Edit,
  X,
  Clock,
  Rocket,
  Check,
  Undo2,
} from "lucide-react";

// Статусы задач: метка + вариант бейджа.
const STATUSES = [
  { value: "new", label: "Новая" },
  { value: "in_progress", label: "В работе" },
  { value: "done", label: "Готова" },
  { value: "failed", label: "Не выполнена" },
  { value: "cancelled", label: "Отменена" },
];

const STATUS_META = {
  new: { label: "Новая", variant: "outline" },
  in_progress: { label: "В работе", variant: "default" },
  done: { label: "Готова", variant: "secondary" },
  failed: { label: "Не выполнена", variant: "destructive" },
  cancelled: { label: "Отменена", variant: "outline" },
};

const statusLabel = (s) => STATUS_META[s]?.label || s;
const statusVariant = (s) => STATUS_META[s]?.variant || "outline";

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

// Форма добавления новой задачи по модификации приложения.
function NewTaskForm({ onSaved }) {
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const handleAdd = async () => {
    setSaving(true);
    setError("");
    try {
      await createAppTask(title, description);
      setTitle("");
      setDescription("");
      onSaved();
    } catch (err) {
      setError(err.message || "Не удалось создать задачу");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card className="my-3" size="sm">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Plus className="size-4 text-muted-foreground" />
          Новая задача
        </CardTitle>
        <CardDescription>
          Опишите, что нужно исправить, изменить или добавить в приложении:
          «исправь ошибку», «измени отступ», «добавь функционал»…
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <Input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="Заголовок: исправить отступ в карточке метрики…"
        />
        <Textarea
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          placeholder="Подробности (необязательно): что именно и где исправить…"
        />
        {error && (
          <p
            className="flex items-center gap-1.5 text-sm text-destructive"
            role="alert"
          >
            <AlertCircle className="size-4" />
            {error}
          </p>
        )}
        <Button onClick={handleAdd} disabled={saving || !title.trim()}>
          {saving ? <Loader2 className="animate-spin" /> : <Plus />}
          {saving ? "Добавляю…" : "Добавить задачу"}
        </Button>
      </CardContent>
    </Card>
  );
}

// Карточка задачи: заголовок, статус (меняется селектом), описание,
// редактирование и удаление.
function TaskCard({ task, onChanged }) {
  const [editing, setEditing] = useState(false);
  const [editTitle, setEditTitle] = useState("");
  const [editDescription, setEditDescription] = useState("");
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deploying, setDeploying] = useState(false);
  const [rollingBack, setRollingBack] = useState(false);
  const [error, setError] = useState("");

  const startEdit = () => {
    setEditTitle(task.title);
    setEditDescription(task.description || "");
    setError("");
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setError("");
  };

  const handleSave = async () => {
    setSaving(true);
    setError("");
    try {
      await updateAppTask(task.id, {
        title: editTitle,
        description: editDescription,
        status: task.status,
      });
      setEditing(false);
      onChanged();
    } catch (err) {
      setError(err.message || "Не удалось обновить задачу");
    } finally {
      setSaving(false);
    }
  };

  const handleStatusChange = async (status) => {
    setError("");
    try {
      await updateAppTask(task.id, {
        title: task.title,
        description: task.description,
        status,
      });
      onChanged();
    } catch (err) {
      setError(err.message || "Не удалось изменить статус");
    }
  };

  const handleDelete = async () => {
    if (!window.confirm(`Удалить задачу «${task.title}»?`)) return;
    setDeleting(true);
    setError("");
    try {
      await deleteAppTask(task.id);
      onChanged();
    } catch (err) {
      setError(err.message || "Не удалось удалить задачу");
      setDeleting(false);
    }
  };

  const handleDeploy = async () => {
    if (
      !window.confirm(
        `Запросить деплой задачи «${task.title}»? Агент закоммитит изменения и запустит make deploy.`,
      )
    ) {
      return;
    }
    setDeploying(true);
    setError("");
    try {
      await requestTaskDeploy(task);
      onChanged();
    } catch (err) {
      setError(err.message || "Не удалось запросить деплой");
      setDeploying(false);
    }
  };

  // Deploy доступен для готовых задач, у которых нет активного запроса
  // и которые ещё не были задеплоены.
  const canDeploy =
    task.status === "done" && !task.deploy_requested && !task.deployed_at;
  // Откат доступен для задеплоенных задач, если откат ещё не запрошен
  // и уже не был выполнен.
  const canRollback =
    task.status === "done" &&
    !!task.deployed_at &&
    !!task.commit_hash &&
    !task.revert_requested &&
    !task.reverted_at;

  const handleRollback = async () => {
    if (
      !window.confirm(
        `Откатить задачу «${task.title}»? Агент сделает git revert коммита ${task.commit_hash.slice(0, 7)} и передеплоит.`,
      )
    ) {
      return;
    }
    setRollingBack(true);
    setError("");
    try {
      await requestTaskRollback(task);
      onChanged();
    } catch (err) {
      setError(err.message || "Не удалось запросить откат");
      setRollingBack(false);
    }
  };
  return (
    <Card className="my-3" size="sm">
      <CardHeader>
        <div className="flex w-full flex-wrap items-center justify-between gap-2">
          <CardTitle className="flex items-center gap-2 text-base">
            <Wrench className="size-4 shrink-0 text-muted-foreground" />
            {task.title}
          </CardTitle>
          <div className="flex items-center gap-2">
            <Badge variant={statusVariant(task.status)}>
              {statusLabel(task.status)}
            </Badge>
            <Button variant="outline" size="sm" onClick={startEdit}>
              <Edit />
            </Button>
            <Button
              variant="destructive"
              size="sm"
              onClick={handleDelete}
              disabled={deleting}
            >
              {deleting ? <Loader2 className="animate-spin" /> : <Trash2 />}
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-3">
        {editing ? (
          <>
            <Input
              value={editTitle}
              onChange={(e) => setEditTitle(e.target.value)}
              placeholder="Заголовок задачи"
            />
            <Textarea
              value={editDescription}
              onChange={(e) => setEditDescription(e.target.value)}
              placeholder="Подробности…"
            />
            <div className="flex gap-2">
              <Button
                onClick={handleSave}
                disabled={saving || !editTitle.trim()}
              >
                {saving ? <Loader2 className="animate-spin" /> : <Save />}
                {saving ? "Сохраняю…" : "Сохранить"}
              </Button>
              <Button variant="ghost" onClick={cancelEdit}>
                <X />
                Отмена
              </Button>
            </div>
          </>
        ) : (
          <>
            {task.description && (
              <p className="whitespace-pre-wrap text-sm leading-relaxed text-foreground">
                {task.description}
              </p>
            )}
            {task.result && (
              <div className="rounded-lg border bg-muted/40 p-3 text-xs leading-relaxed text-muted-foreground">
                <span className="font-medium text-foreground">Результат:</span>{" "}
                {task.result}
              </div>
            )}
            {task.log && (
              <details className="group rounded-lg border">
                <summary className="cursor-pointer select-none px-3 py-2 text-xs font-medium text-muted-foreground transition-colors hover:text-foreground">
                  Журнал выполнения — клик, чтобы развернуть
                </summary>
                <pre className="max-h-72 overflow-y-auto whitespace-pre-wrap border-t bg-muted/40 px-3 py-2 text-xs leading-relaxed text-foreground">
                  {task.log}
                </pre>
              </details>
            )}
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="flex flex-wrap items-center gap-2">
                <Select value={task.status} onValueChange={handleStatusChange}>
                  <SelectTrigger className="w-36">
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
                {canDeploy && (
                  <Button onClick={handleDeploy} disabled={deploying}>
                    {deploying ? (
                      <Loader2 className="animate-spin" />
                    ) : (
                      <Rocket />
                    )}
                    {deploying ? "Запрашиваю…" : "Deploy"}
                  </Button>
                )}
                {canRollback && (
                  <Button
                    variant="outline"
                    onClick={handleRollback}
                    disabled={rollingBack}
                  >
                    {rollingBack ? (
                      <Loader2 className="animate-spin" />
                    ) : (
                      <Undo2 />
                    )}
                    {rollingBack ? "Запрашиваю…" : "Откатить"}
                  </Button>
                )}
                {task.deploy_requested && (
                  <Badge variant="default">Деплой запрошен</Badge>
                )}
                {task.revert_requested && (
                  <Badge variant="default">Откат запрошен</Badge>
                )}
                {task.deployed_at && (
                  <Badge variant="secondary">
                    <Check className="size-3" />
                    Деплой: {task.deployed_at}
                  </Badge>
                )}
                {task.reverted_at && (
                  <Badge variant="destructive">
                    <Undo2 className="size-3" />
                    Откачено: {task.reverted_at}
                  </Badge>
                )}
                {task.commit_hash && (
                  <Badge variant="outline">
                    коммит: {task.commit_hash.slice(0, 7)}
                  </Badge>
                )}
              </div>
              {task.updated && (
                <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
                  <Clock className="size-3.5" />
                  обновлено: <DateDisplay date={task.updated} withTime />
                </span>
              )}
            </div>
          </>
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
    </Card>
  );
}

// Главный компонент: раздел «Приложение» — задачи по модификации приложения.
function AppTasks() {
  const queryClient = useQueryClient();
  const tasksQuery = useAppTasks(true);

  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: ["app-tasks"] });

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

  const tasks = tasksQuery.data?.tasks || [];

  return (
    <section>
      <NewTaskForm onSaved={refresh} />

      {tasks.length === 0 ? (
        <Card className="my-3" size="sm">
          <CardContent>
            <p className="flex items-center gap-2 text-sm text-muted-foreground">
              <Wrench className="size-4" />
              Пока нет ни одной задачи. Добавьте первую выше.
            </p>
          </CardContent>
        </Card>
      ) : (
        tasks.map((t) => (
          <TaskCard key={t.id} task={t} onChanged={refresh} />
        ))
      )}
    </section>
  );
}

export default AppTasks;
