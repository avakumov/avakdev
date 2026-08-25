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

// Отображение типа в бейдже: для числовых метрик — единица измерения
// (кг, мин, км…), если она задана, иначе название типа; для «да/нет» — «Да / Нет».
function typeBadgeLabel(def) {
  if (def.type === "bool") return "Да / Нет";
  return def.unit || TYPE_LABELS[def.type];
}

// Переключатель «Да / Нет» для boolean-метрик.
// По высоте совпадает с Input/Button/Select (h-8), не растягивается на всю
// ширину колонки — компактный сегмент-переключатель.
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

// Карточка одной метрики: сворачивается по умолчанию — в свёрнутом виде
// показывает название, число зафиксированных дней и тип/единицу измерения.
// В развёрнутом — ввод значения за день + история.
// За день фиксируется один показатель — повторное сохранение перезаписывает.
function MetricCard({ def, values, onChanged }) {
  const today = new Date().toISOString().slice(0, 10);
  const [date, setDate] = useState(today);
  const [value, setValue] = useState("");
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState("");

  // Редактирование названия и единицы измерения.
  const [editing, setEditing] = useState(false);
  const [editName, setEditName] = useState("");
  const [editUnit, setEditUnit] = useState("");
  const [savingEdit, setSavingEdit] = useState(false);

  const startEdit = () => {
    setEditName(def.name);
    setEditUnit(def.unit || "");
    setError("");
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setError("");
  };

  const handleSaveEdit = async () => {
    setSavingEdit(true);
    setError("");
    try {
      await updateUserMetric(def.id, editName, editUnit);
      setEditing(false);
      onChanged();
    } catch (err) {
      setError(err.message || "Не удалось изменить метрику");
    } finally {
      setSavingEdit(false);
    }
  };

  // При выборе даты подставляем уже записанное значение (если есть).
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

  // Последние значения (новые сверху), для истории показываем до 14.
  const history = Object.entries(values)
    .map(([d, v]) => ({ date: d, value: v }))
    .sort((a, b) => (a.date < b.date ? 1 : -1))
    .slice(0, 14);

  const daysCount = Object.keys(values).length;
  const [open, setOpen] = useState(false);

  const toggle = () => {
    if (editing) return; // не сворачиваем посреди редактирования
    setOpen(!open);
  };

  return (
    <Card className="my-3" size="sm">
      <CardHeader className="cursor-pointer select-none" onClick={toggle}>
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

      {open && (
        <CardContent className="space-y-3">
        {editing && (
          <div className="space-y-3 rounded-lg border bg-muted/40 p-3">
            <p className="text-sm font-medium">Редактирование метрики</p>
            <div className="flex flex-col gap-2 sm:flex-row">
              <Input
                value={editName}
                onChange={(e) => setEditName(e.target.value)}
                placeholder="Название метрики"
                className="sm:flex-1"
              />
              {def.type !== "bool" && (
                <Input
                  value={editUnit}
                  onChange={(e) => setEditUnit(e.target.value)}
                  placeholder="Единица: кг, мин, км…"
                  className="sm:w-44"
                />
              )}
            </div>
            <div className="flex gap-2">
              <Button onClick={handleSaveEdit} disabled={savingEdit || !editName.trim()}>
                {savingEdit ? <Loader2 className="animate-spin" /> : <Save />}
                {savingEdit ? "Сохраняю…" : "Сохранить"}
              </Button>
              <Button variant="ghost" onClick={cancelEdit}>
                <X />
                Отмена
              </Button>
            </div>
          </div>
        )}

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

// Главный компонент: список метрик пользователя + форма добавления новой.
function Metrics() {
  const queryClient = useQueryClient();
  const metricsQuery = useUserMetrics(true);

  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: ["user-metrics"] });

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
        defs.map((d) => (
          <MetricCard
            key={d.id}
            def={d}
            values={valuesByMetric[d.id] || {}}
            onChanged={refresh}
          />
        ))
      )}
    </section>
  );
}

export default Metrics;
