import { useState } from "react";
import {
  useUserMetrics,
  createUserMetric,
  updateUserMetric,
  deleteUserMetric,
  setUserMetricValue,
  deleteUserMetricValue,
} from "./api.js";
import { useQueryClient } from "@tanstack/react-query";
import DateDisplay from "@/components/DateDisplay.jsx";
import DateInput from "@/components/DateInput.jsx";
import { cn } from "@/lib/utils";
import { formatDateDmy } from "@/lib/formatDate.js";

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
  Plus,
  Save,
  Trash2,
  Loader2,
  AlertCircle,
  BarChart3,
  X,
  CalendarDays,
  Edit,
  ChevronDown,
  ChevronRight,
} from "lucide-react";

// Типы метрик: целое число, дробное число, да/нет.
const TYPE_LABELS = {
  int: "Целое число",
  float: "Дробное число",
  bool: "Да / Нет",
};

const TYPE_OPTIONS = [
  { value: "float", label: "Дробное число" },
  { value: "int", label: "Целое число" },
  { value: "bool", label: "Да / Нет" },
];

// Отображение значения в зависимости от типа.
function displayValue(type, value) {
  if (type === "bool") return value === "true" ? "Да" : "Нет";
  return value;
}

// Отображение типа/единицы: для числовых метрик — единица измерения (кг, мин…),
// если задана, иначе название типа; для «да/нет» — «Да / Нет».
function typeBadgeLabel(def) {
  if (def.type === "bool") return "Да / Нет";
  return def.unit || TYPE_LABELS[def.type];
}

// Переключатель «Да / Нет» для boolean-метрик.
export function BoolToggle({ value, onChange }) {
  const opt = (v, label) => (
    <button
      type="button"
      key={v}
      onClick={() => onChange(v)}
      className={
        "h-full rounded-md px-4 text-sm font-medium transition-colors " +
        (value === v
          ? "bg-background text-foreground shadow-sm"
          : "text-muted-foreground hover:text-foreground")
      }
    >
      {label}
    </button>
  );
  return (
    <div className="flex h-8 w-fit items-center gap-1 rounded-lg border bg-muted/40 p-0.5">
      {opt("true", "Да")}
      {opt("false", "Нет")}
    </div>
  );
}

// Форма создания новой метрики: название + тип (+ единица для числовых).
function NewMetricForm({ onSaved }) {
  const [name, setName] = useState("");
  const [type, setType] = useState("float");
  const [unit, setUnit] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const handleAdd = async () => {
    setSaving(true);
    setError("");
    try {
      await createUserMetric(name, type, unit);
      setName("");
      setUnit("");
      onSaved();
    } catch (err) {
      setError(err.message || "Не удалось создать метрику");
    } finally {
      setSaving(false);
    }
  };

  const isNumeric = type !== "bool";

  return (
    <Card className="my-3" size="sm">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Plus className="size-4 text-muted-foreground" />
          Новая метрика
        </CardTitle>
        <CardDescription>
          Добавьте свою метрику: вес, калории, принимаемые лекарства, время
          прогулки, количество отжиманий, курил ли сегодня — и другие.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="flex flex-col gap-2 sm:flex-row">
          <Input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Название: Вес, Количество отжиманий..."
            className="sm:flex-1"
          />
          <Select value={type} onValueChange={setType}>
            <SelectTrigger className="w-full sm:w-60">
              <SelectValue placeholder="Тип" />
            </SelectTrigger>
            <SelectContent>
              {TYPE_OPTIONS.map((o) => (
                <SelectItem key={o.value} value={o.value}>
                  {o.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {isNumeric && (
            <Input
              value={unit}
              onChange={(e) => setUnit(e.target.value)}
              placeholder="Единица: кг, мин, км…"
              className="sm:w-44"
            />
          )}
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
        <Button onClick={handleAdd} disabled={saving || !name.trim()}>
          {saving ? <Loader2 className="animate-spin" /> : <Plus />}
          {saving ? "Добавляю…" : "Добавить метрику"}
        </Button>
      </CardContent>
    </Card>
  );
}

// Русская плюрализация для дней: 1 день, 2 дня, 5 дней, 21 день…
function pluralDays(n) {
  const d10 = n % 10;
  const d100 = n % 100;
  if (d10 === 1 && d100 !== 11) return "день";
  if (d10 >= 2 && d10 <= 4 && (d100 < 12 || d100 > 14)) return "дня";
  return "дней";
}

// Модалка изменения метрики: название (+ единица для числовых).
function MetricEditModal({ def, onSaved, onClose }) {
  const [editName, setEditName] = useState(def.name);
  const [editUnit, setEditUnit] = useState(def.unit || "");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const handleSave = async () => {
    setSaving(true);
    setError("");
    try {
      await updateUserMetric(def.id, editName, editUnit);
      onSaved();
      onClose();
    } catch (err) {
      setError(err.message || "Не удалось изменить метрику");
      setSaving(false);
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      onClick={onClose}
    >
      <Card
        className="flex max-h-[85vh] w-full max-w-md flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <BarChart3 className="size-4 text-muted-foreground" />
            Редактировать метрику
          </CardTitle>
          <CardDescription>Название и единица измерения</CardDescription>
        </CardHeader>
        <CardContent className="flex-1 space-y-4 overflow-y-auto">
          <div className="space-y-1.5">
            <Label>Название</Label>
            <Input
              value={editName}
              onChange={(e) => setEditName(e.target.value)}
              placeholder="Название метрики"
            />
          </div>
          {def.type !== "bool" && (
            <div className="space-y-1.5">
              <Label>Единица измерения</Label>
              <Input
                value={editUnit}
                onChange={(e) => setEditUnit(e.target.value)}
                placeholder="кг, мин, км… (необязательно)"
              />
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
          <Button variant="ghost" onClick={onClose} disabled={saving}>
            <X />
            Отмена
          </Button>
          <Button
            onClick={handleSave}
            disabled={saving || !editName.trim()}
          >
            {saving ? <Loader2 className="animate-spin" /> : <Save />}
            {saving ? "Сохраняю…" : "Сохранить"}
          </Button>
        </div>
      </Card>
    </div>
  );
}

// Строка таблицы (десктоп): метрика + значения по дням. Всё видно и
// редактируется прямо в таблице — ячейки дней кликабельны.
function MetricRow({ def, values, columns, onChanged, onEdit }) {
  const [editDate, setEditDate] = useState(null); // дата редактируемой ячейки
  const [editText, setEditText] = useState("");
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState("");

  const startCellEdit = (d) => {
    if (def.type === "bool") return; // да/нет переключается кликом
    setEditDate(d);
    setEditText(values[d] ?? "");
    setError("");
  };

  // Сохранение числового значения: пустое поле удаляет показатель за день.
  const commitCell = async (d) => {
    const text = editText.trim();
    setEditDate(null);
    if (text === "" && values[d] === undefined) return; // ничего не вводили
    setError("");
    try {
      if (text === "") {
        await deleteUserMetricValue(def.id, d);
      } else {
        await setUserMetricValue(def.id, d, text);
      }
      onChanged();
    } catch (err) {
      setError(err.message || "Не удалось сохранить показатель");
    }
  };

  // Да/Нет: клик по ячейке циклически меняет: — → Да → Нет → —.
  const cycleBool = async (d) => {
    const cur = values[d];
    const next = cur === undefined ? "true" : cur === "true" ? "false" : undefined;
    setError("");
    try {
      if (next === undefined) {
        await deleteUserMetricValue(def.id, d);
      } else {
        await setUserMetricValue(def.id, d, next);
      }
      onChanged();
    } catch (err) {
      setError(err.message || "Не удалось сохранить показатель");
    }
  };

  const handleDelete = async () => {
    if (!window.confirm(`Удалить метрику «${def.name}» и все её показатели?`)) {
      return;
    }
    setDeleting(true);
    setError("");
    try {
      await deleteUserMetric(def.id);
      onChanged();
    } catch (err) {
      setError(err.message || "Не удалось удалить метрику");
      setDeleting(false);
    }
  };

  return (
    <>
      <tr className="border-b border-border/60 last:border-0 hover:bg-muted/30">
        <td className="w-48 px-3 py-2 align-top">
          <div className="flex items-start gap-1.5">
            <BarChart3 className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
            <span className="min-w-0">
              <span className="wrap-break-word font-medium text-foreground">
                {def.name}
              </span>
              <span className="block text-[11px] text-muted-foreground">
                {typeBadgeLabel(def)}
              </span>
            </span>
          </div>
          {error && (
            <p
              className="mt-1 flex items-center gap-1 text-xs text-destructive"
              role="alert"
            >
              <AlertCircle className="size-3.5" />
              {error}
            </p>
          )}
        </td>

        {columns.map((d) => {
          const v = values[d];
          if (def.type === "bool") {
            return (
              <td
                key={d}
                className="px-1 py-1.5 text-center align-top"
              >
                <button
                  type="button"
                  onClick={() => cycleBool(d)}
                  title="Клик — сменить: — → Да → Нет → —"
                  className={cn(
                    "flex w-full cursor-pointer items-center justify-center rounded-md py-1 text-sm font-medium whitespace-nowrap tabular-nums transition-colors",
                    v === undefined && "text-muted-foreground/60 hover:bg-muted/50",
                    v !== undefined && "bg-muted/40 hover:bg-muted",
                  )}
                >
                  {v !== undefined ? displayValue(def.type, v) : "—"}
                </button>
              </td>
            );
          }
          if (editDate === d) {
            return (
              <td key={d} className="px-1 py-1 align-top">
                <Input
                  type="number"
                  step={def.type === "int" ? 1 : "any"}
                  value={editText}
                  autoFocus
                  onChange={(e) => setEditText(e.target.value)}
                  onBlur={() => commitCell(d)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") commitCell(d);
                    if (e.key === "Escape") setEditDate(null);
                  }}
                  aria-label={`Значение ${def.name} за день`}
                  className="h-7 w-full min-w-0 px-1 text-center text-sm tabular-nums"
                />
              </td>
            );
          }
          return (
            <td key={d} className="px-1 py-1.5 text-center align-top">
              <button
                type="button"
                onClick={() => startCellEdit(d)}
                title={
                  v !== undefined
                    ? `Изменить: ${displayValue(def.type, v)}`
                    : "Добавить значение"
                }
                className={cn(
                  "flex w-full cursor-pointer items-center justify-center rounded-md py-1 text-sm whitespace-nowrap tabular-nums transition-colors",
                  v === undefined && "text-muted-foreground/60 hover:bg-muted/50",
                  v !== undefined && "hover:bg-muted/50",
                )}
              >
                {v !== undefined ? (
                  <span className="font-medium text-foreground">
                    {displayValue(def.type, v)}
                  </span>
                ) : (
                  "—"
                )}
              </button>
            </td>
          );
        })}

        <td className="w-20 px-1 py-2 align-top text-right">
          <div className="flex justify-end gap-1">
            <Button
              variant="outline"
              size="icon-sm"
              onClick={() => onEdit(def)}
              aria-label="Редактировать метрику"
            >
              <Edit />
            </Button>
            <Button
              variant="destructive"
              size="icon-sm"
              onClick={handleDelete}
              disabled={deleting}
              aria-label="Удалить метрику"
            >
              {deleting ? <Loader2 className="animate-spin" /> : <Trash2 />}
            </Button>
          </div>
        </td>
      </tr>
    </>
  );
}

// Карточка метрики (мобильные): сворачивается — в свёрнутом виде название,
// число зафиксированных дней и тип/единица; в развёрнутом — ввод значения.
function MetricCard({ def, values, onChanged, onEdit }) {
  const today = new Date().toISOString().slice(0, 10);
  const [date, setDate] = useState(today);
  const [value, setValue] = useState("");
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState("");
  const [open, setOpen] = useState(false);

  const handleDateChange = (d) => {
    setDate(d);
    setValue(values[d] ?? "");
  };

  const handleSave = async () => {
    setSaving(true);
    setError("");
    try {
      await setUserMetricValue(def.id, date, value);
      onChanged();
    } catch (err) {
      setError(err.message || "Не удалось сохранить показатель");
    } finally {
      setSaving(false);
    }
  };

  const handleDeleteValue = async (d) => {
    setError("");
    try {
      await deleteUserMetricValue(def.id, d);
      if (d === date) setValue("");
      onChanged();
    } catch (err) {
      setError(err.message || "Не удалось удалить показатель");
    }
  };

  const handleDelete = async () => {
    if (!window.confirm(`Удалить метрику «${def.name}» и все её показатели?`)) {
      return;
    }
    setDeleting(true);
    setError("");
    try {
      await deleteUserMetric(def.id);
      onChanged();
    } catch (err) {
      setError(err.message || "Не удалось удалить метрику");
      setDeleting(false);
    }
  };

  const history = Object.entries(values)
    .map(([d, v]) => ({ date: d, value: v }))
    .sort((a, b) => (a.date < b.date ? 1 : -1))
    .slice(0, 14);

  const daysCount = Object.keys(values).length;

  return (
    <Card className="my-3" size="sm">
      <CardHeader
        className="cursor-pointer select-none"
        onClick={() => setOpen(!open)}
      >
        <div className="flex w-full flex-wrap items-center justify-between gap-2">
          <CardTitle className="flex items-center gap-2">
            {open ? (
              <ChevronDown className="size-4 text-muted-foreground" />
            ) : (
              <ChevronRight className="size-4 text-muted-foreground" />
            )}
            <BarChart3 className="size-4 text-muted-foreground" />
            {def.name}
          </CardTitle>
          <div
            className="flex items-center gap-2"
            onClick={(e) => e.stopPropagation()}
          >
            <Badge variant="secondary">{typeBadgeLabel(def)}</Badge>
            {daysCount > 0 && (
              <Badge variant="secondary">
                {daysCount} {pluralDays(daysCount)}
              </Badge>
            )}
            <Button
              variant="outline"
              size="sm"
              onClick={() => onEdit(def)}
            >
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

      {open && (
        <CardContent className="space-y-3">
          <div className="flex flex-wrap items-end gap-2">
            <div className="space-y-1.5">
              <Label>Дата</Label>
              <DateInput value={date} max={today} onChange={handleDateChange} />
            </div>
            <div className="min-w-40 flex-1 space-y-1.5">
              <Label>Значение</Label>
              {def.type === "bool" ? (
                <BoolToggle value={value} onChange={setValue} />
              ) : (
                <Input
                  type="number"
                  step={def.type === "int" ? 1 : "any"}
                  value={value}
                  onChange={(e) => setValue(e.target.value)}
                  placeholder={def.type === "int" ? "Целое число" : "Число"}
                />
              )}
            </div>
            <Button onClick={handleSave} disabled={saving}>
              {saving ? <Loader2 className="animate-spin" /> : <Save />}
              {saving ? "Сохраняю…" : "Сохранить"}
            </Button>
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

          {history.length > 0 && (
            <div>
              <p className="mb-1.5 text-xs font-medium text-muted-foreground">
                Последние значения:
              </p>
              <ul className="divide-y rounded-lg border">
                {history.map((h) => (
                  <li
                    key={h.date}
                    className="flex items-center justify-between gap-2 px-3 py-1.5 text-sm"
                  >
                    <button
                      type="button"
                      onClick={() => handleDateChange(h.date)}
                      className="flex items-center gap-2 text-left text-muted-foreground hover:text-foreground"
                    >
                      <CalendarDays className="size-3.5" />
                      <DateDisplay date={h.date} className="tabular-nums" />
                      <span className="font-medium tabular-nums text-foreground">
                        {displayValue(def.type, h.value)}
                      </span>
                    </button>
                    <button
                      type="button"
                      onClick={() => handleDeleteValue(h.date)}
                      className="text-muted-foreground hover:text-destructive"
                      aria-label="Удалить значение за этот день"
                    >
                      <X className="size-4" />
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </CardContent>
      )}
    </Card>
  );
}

// Главный компонент: метрики пользователя + форма добавления новой.
// На десктопе (md+) — таблица «метрика × даты» с редактированием в ячейках,
// на мобильных — сворачиваемые карточки.
function Metrics() {
  const queryClient = useQueryClient();
  const metricsQuery = useUserMetrics(true);

  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: ["user-metrics"] });

  const [editTarget, setEditTarget] = useState(null); // метрика в модалке

  if (metricsQuery.isLoading) {
    return (
      <p className="text-sm text-muted-foreground">
        <Loader2 className="mr-1 inline size-4 animate-spin" />
        Загрузка метрик…
      </p>
    );
  }
  if (metricsQuery.isError) {
    return (
      <p className="text-sm text-destructive">
        Ошибка: {metricsQuery.error?.message}
      </p>
    );
  }

  const defs = metricsQuery.data?.definitions || [];
  const valuesByMetric = {};
  for (const v of metricsQuery.data?.values || []) {
    (valuesByMetric[v.metric_id] ||= {})[v.date] = v.value;
  }

  // Столбцы таблицы — календарные дни: последняя неделя, сегодня справа.
  const todayIso = new Date().toISOString().slice(0, 10);
  const columns = lastNDays(7, todayIso);

  return (
    <section>
      <NewMetricForm onSaved={refresh} />

      {defs.length === 0 ? (
        <Card className="my-3" size="sm">
          <CardContent>
            <p className="flex items-center gap-2 text-sm text-muted-foreground">
              <BarChart3 className="size-4" />
              Пока нет ни одной метрики. Добавьте первую выше.
            </p>
          </CardContent>
        </Card>
      ) : (
        <>
          {/* Таблица — desktop (md и шире): редактирование прямо в ячейках. */}
          <Card className="my-3 hidden overflow-hidden md:block" size="sm">
            <div className="overflow-x-auto">
              <table className="w-full table-fixed text-sm">
                <thead>
                  <tr className="border-b bg-muted/40 text-left text-[11px] uppercase tracking-wide text-muted-foreground">
                    <th className="w-48 px-3 py-2 align-middle font-bold">
                      Метрика
                    </th>
                    {columns.map((d) => (
                      <th
                        key={d}
                        title={formatDateDmy(d)}
                        className={cn(
                          "px-1 py-2 text-center align-middle font-bold tabular-nums",
                          d === todayIso && "text-primary",
                        )}
                      >
                        {formatDateDmy(d).slice(0, 5)}
                      </th>
                    ))}
                    <th className="w-20 px-1 py-2 text-right align-middle font-bold">
                      <span className="sr-only">Действия</span>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {defs.map((d) => (
                    <MetricRow
                      key={d.id}
                      def={d}
                      values={valuesByMetric[d.id] || {}}
                      columns={columns}
                      onChanged={refresh}
                      onEdit={setEditTarget}
                    />
                  ))}
                </tbody>
              </table>
            </div>
          </Card>

          {/* Карточки — mobile (< md) */}
          <div className="md:hidden">
            {defs.map((d) => (
              <MetricCard
                key={d.id}
                def={d}
                values={valuesByMetric[d.id] || {}}
                onChanged={refresh}
                onEdit={setEditTarget}
              />
            ))}
          </div>
        </>
      )}

      {editTarget && (
        <MetricEditModal
          def={editTarget}
          onSaved={refresh}
          onClose={() => setEditTarget(null)}
        />
      )}
    </section>
  );
}

// Последние n календарных дней включительно с endIso (старые слева).
function lastNDays(n, endIso) {
  const arr = [];
  const end = new Date(endIso + "T00:00:00Z");
  for (let i = n - 1; i >= 0; i--) {
    const d = new Date(end);
    d.setUTCDate(end.getUTCDate() - i);
    arr.push(d.toISOString().slice(0, 10));
  }
  return arr;
}

export default Metrics;
