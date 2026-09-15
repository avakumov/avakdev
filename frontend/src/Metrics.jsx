import { useEffect, useRef, useState } from "react";
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
  Minus,
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

// Модалка изменения метрики: название (+ единица для числовых) и удаление.
function MetricEditModal({ def, onSaved, onClose, onDeleted }) {
  const [editName, setEditName] = useState(def.name);
  const [editUnit, setEditUnit] = useState(def.unit || "");
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
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

  // Удаление метрики со всеми её показателями.
  const handleDelete = async () => {
    if (!window.confirm(`Удалить метрику «${def.name}» и все её показатели?`)) {
      return;
    }
    setDeleting(true);
    setError("");
    try {
      await deleteUserMetric(def.id);
      onSaved();
      onDeleted?.();
      onClose();
    } catch (err) {
      setError(err.message || "Не удалось удалить метрику");
      setDeleting(false);
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
        <div className="flex items-center justify-between gap-2 border-t p-4">
          <Button
            variant="destructive"
            onClick={handleDelete}
            disabled={saving || deleting}
          >
            {deleting ? <Loader2 className="animate-spin" /> : <Trash2 />}
            {deleting ? "Удаляю…" : "Удалить метрику"}
          </Button>
          <div className="flex gap-2">
            <Button
              variant="ghost"
              onClick={onClose}
              disabled={saving || deleting}
            >
              <X />
              Отмена
            </Button>
            <Button
              onClick={handleSave}
              disabled={saving || deleting || !editName.trim()}
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

// Строка таблицы (десктоп): метрика + значения по дням. Всё видно и
// редактируется прямо в таблице — ячейки дней кликабельны.
// Клик по названию метрики открывает её карточку (onOpenDetail).
function MetricRow({ def, values, columns, borders, onChanged, onOpenDetail }) {
  const [editDate, setEditDate] = useState(null); // дата редактируемой ячейки
  const [editText, setEditText] = useState("");
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

  return (
    <tr className="group border-b border-border/60 last:border-0 hover:bg-muted/30">
      <td className="sticky left-0 z-10 w-44 border-r border-border/70 bg-card px-3 py-1.5 align-top transition-colors group-hover:bg-[color-mix(in_oklch,var(--muted)_30%,var(--card))]">
        <button
          type="button"
          onClick={() => onOpenDetail(def)}
          title="Открыть карточку метрики"
          className="flex w-full cursor-pointer items-start gap-1.5 rounded-md text-left"
        >
          <BarChart3 className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
          <span className="min-w-0">
            <span className="wrap-break-word font-medium text-foreground transition-colors group-hover:text-primary">
              {def.name}
            </span>
            <span className="block text-[10px] leading-tight text-muted-foreground">
              {typeBadgeLabel(def)}
            </span>
          </span>
        </button>
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
          const isBoundary = borders?.has(d) ?? false;
          if (def.type === "bool") {
            const label =
              v === undefined
                ? "Пусто. Клик — записать «да»"
                : v === "true"
                  ? "Да. Клик — «нет»"
                  : "Нет. Клик — очистить";
            return (
              <td
                key={d}
                className={cn(
                  "px-0.5 py-1 text-center align-middle",
                  isBoundary && "border-r border-border/70",
                )}
              >
                <button
                  type="button"
                  onClick={() => cycleBool(d)}
                  title={label}
                  aria-label={label}
                  className={cn(
                    "inline-flex h-7 w-full cursor-pointer items-center justify-center transition-colors",
                    v === undefined
                      ? "hover:bg-muted/50"
                      : v === "true"
                        ? "bg-emerald-500/15 text-emerald-700 hover:bg-emerald-500/25 dark:text-emerald-400"
                        : "bg-destructive/15 text-destructive hover:bg-destructive/25",
                  )}
                >
                  {v === undefined ? (
                    <Minus className="size-3.5 text-muted-foreground/50" />
                  ) : (
                    <span className="text-xs font-medium tabular-nums">
                      {v === "true" ? "Д" : "Н"}
                    </span>
                  )}
                </button>
              </td>
            );
          }
          if (editDate === d) {
            return (
              <td
                key={d}
                className={cn(
                  "px-0.5 py-1 align-middle",
                  isBoundary && "border-r border-border/70",
                )}
              >
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
            <td
              key={d}
              className={cn(
                "px-0.5 py-1 text-center align-middle",
                isBoundary && "border-r border-border/70",
              )}
            >
              <button
                type="button"
                onClick={() => startCellEdit(d)}
                title={
                  v !== undefined
                    ? `Изменить: ${displayValue(def.type, v)}`
                    : "Добавить значение"
                }
                className={cn(
                  "inline-flex h-7 w-full cursor-pointer items-center justify-center rounded-md whitespace-nowrap tabular-nums transition-colors",
                  v === undefined
                    ? "text-sm text-muted-foreground/50 hover:bg-muted/50"
                    : "text-sm hover:bg-muted/50",
                )}
              >
                {v !== undefined ? (
                  <span className="text-foreground">
                    {displayValue(def.type, v)}
                  </span>
                ) : (
                  "—"
                )}
              </button>
            </td>
          );
        })}
      </tr>
  );
}

// Карточка метрики (мобильные): сворачивается — в свёрнутом виде название,
// число зафиксированных дней и тип/единица; в развёрнутом — ввод значения.
function MetricCard({ def, values, onChanged, onEdit, initialOpen = false }) {
  const today = new Date().toISOString().slice(0, 10);
  const [date, setDate] = useState(today);
  const [value, setValue] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [open, setOpen] = useState(initialOpen);

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
              title="Редактировать метрику"
              aria-label="Редактировать метрику"
            >
              <Edit />
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
                      {def.type === "bool" ? (
                        <span
                          className={cn(
                            "inline-flex size-6 items-center justify-center text-xs font-medium",
                            h.value === "true"
                              ? "bg-emerald-500/15 text-emerald-700 dark:text-emerald-400"
                              : "bg-destructive/15 text-destructive",
                          )}
                        >
                          {h.value === "true" ? "Д" : "Н"}
                        </span>
                      ) : (
                        <span className="font-medium tabular-nums text-foreground">
                          {h.value}
                        </span>
                      )}
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
  const [metricTarget, setMetricTarget] = useState(null); // карточка метрики в модалке

  // Данные (во время загрузки — пустые, но хуки должны быть до return).
  const defs = metricsQuery.data?.definitions || [];
  const valuesByMetric = {};
  for (const v of metricsQuery.data?.values || []) {
    (valuesByMetric[v.metric_id] ||= {})[v.date] = v.value;
  }

  // Столбцы таблицы — все даты с записями (+ сегодня). Если не влезают —
  // таблица прокручивается по горизонтали.
  const todayIso = new Date().toISOString().slice(0, 10);
  const columns = collectDates(valuesByMetric, defs);
  const currentYear = Number(todayIso.slice(0, 4));
  const monthGroups = groupColumns(columns);
  // Даты, после которых заканчивается месяц (кроме последней группы):
  // после них рисуем вертикальный разделитель на всю высоту таблицы.
  const groupBorders = new Set(
    monthGroups
      .slice(0, -1)
      .map((g) => g.dates[g.dates.length - 1]),
  );
  // Минимальная ширина: название (176px) + колонки дат (~30px каждая —
  // после уменьшения в 1,5 раза ещё на 20%). Действия убраны из таблицы.
  const tableMinWidth = 176 + columns.length * 30;

  // Прокрутка таблицы всегда в конец (к последним датам): при открытии
  // и при появлении новых колонок-дат.
  const scrollRef = useRef(null);
  useEffect(() => {
    const el = scrollRef.current;
    if (el) el.scrollLeft = el.scrollWidth;
  }, [columns.length]);

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
            <div className="overflow-x-auto" ref={scrollRef}>
              <table
                className="table-fixed text-sm"
                style={{ width: tableMinWidth }}
              >
                <thead>
                  <tr className="bg-muted/40 text-left text-[11px] tracking-wide text-muted-foreground">
                    <th
                      rowSpan={2}
                      className="sticky left-0 z-20 w-44 border-r border-border/70 bg-[color-mix(in_oklch,var(--muted)_40%,var(--card))] px-3 py-1.5 align-middle font-bold uppercase"
                    >
                      Метрика
                    </th>
                    {monthGroups.map((g, gi) => (
                      <th
                        key={`${g.year}-${g.month}`}
                        colSpan={g.dates.length}
                        className={cn(
                          "overflow-hidden px-0.5 py-1 text-center align-middle text-xs font-semibold whitespace-nowrap capitalize text-ellipsis",
                          gi < monthGroups.length - 1 &&
                            "border-r border-border/70",
                        )}
                      >
                        {monthLabel(g, currentYear)}
                      </th>
                    ))}
                  </tr>
                  <tr className="border-b bg-muted/40 text-left text-[11px] uppercase tracking-wide text-muted-foreground">
                    {columns.map((d) => (
                      <th
                        key={d}
                        title={formatDateDmy(d)}
                        className={cn(
                          "px-0.5 py-1 text-center align-middle font-bold tabular-nums",
                          d === todayIso && "text-primary",
                          groupBorders.has(d) && "border-r border-border/70",
                        )}
                      >
                        {String(Number(d.slice(8, 10)))}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {defs.map((d) => (
                    <MetricRow
                      key={d.id}
                      def={d}
                      values={valuesByMetric[d.id] || {}}
                      columns={columns}
                      borders={groupBorders}
                      onChanged={refresh}
                      onOpenDetail={setMetricTarget}
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

      {/* Карточка метрики по клику на название в таблице (desktop):
          модалка с «мобильной» версией карточки. */}
      {metricTarget && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
          onClick={() => setMetricTarget(null)}
        >
          <div
            className="max-h-[85vh] w-full max-w-lg overflow-y-auto rounded-lg"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="mb-1 flex justify-end">
              <Button
                variant="ghost"
                size="icon"
                onClick={() => setMetricTarget(null)}
                aria-label="Закрыть"
              >
                <X />
              </Button>
            </div>
            <MetricCard
              def={metricTarget}
              values={valuesByMetric[metricTarget.id] || {}}
              onChanged={refresh}
              onEdit={setEditTarget}
              initialOpen
            />
          </div>
        </div>
      )}

      {editTarget && (
        <MetricEditModal
          def={editTarget}
          onSaved={refresh}
          onClose={() => setEditTarget(null)}
          onDeleted={() => setMetricTarget(null)}
        />
      )}
    </section>
  );
}

// Прибавляет n дней к дате 'YYYY-MM-DD'.
function isoAddDays(iso, n) {
  const d = new Date(iso + "T00:00:00Z");
  d.setUTCDate(d.getUTCDate() + n);
  return d.toISOString().slice(0, 10);
}

// Все даты таблицы — непрерывный диапазон от самой ранней даты (создание
// метрики или первое значение) до сегодня, включая дни без показателей.
function collectDates(valuesByMetric, defs = []) {
  const dateRe = /^\d{4}-\d{2}-\d{2}$/;
  let earliest = null;
  const consider = (d) => {
    if (!dateRe.test(d)) return;
    if (!earliest || d < earliest) earliest = d;
  };

  for (const byDate of Object.values(valuesByMetric)) {
    for (const d in byDate) consider(d);
  }
  for (const def of defs) {
    consider((def.created || "").slice(0, 10));
  }

  const today = new Date().toISOString().slice(0, 10);
  const start = earliest && earliest < today ? earliest : today;
  const out = [];
  for (let d = start; d <= today; d = isoAddDays(d, 1)) out.push(d);
  return out;
}

const MONTHS = [
  "январь",
  "февраль",
  "март",
  "апрель",
  "май",
  "июнь",
  "июль",
  "август",
  "сентябрь",
  "октябрь",
  "ноябрь",
  "декабрь",
];

// Группирует отсортированные даты по (месяц, год) для шапки таблицы.
function groupColumns(columns) {
  const groups = [];
  for (const d of columns) {
    const [y, m] = d.split("-");
    const last = groups[groups.length - 1];
    if (last && last.year === y && last.month === m) {
      last.dates.push(d);
    } else {
      groups.push({ year: y, month: m, dates: [d] });
    }
  }
  return groups;
}

// Название месяца в шапке: к прошлым годам добавляется год.
function monthLabel(group, currentYear) {
  const name = MONTHS[Number(group.month) - 1] || group.month;
  return Number(group.year) === currentYear ? name : `${name} ${group.year}`;
}

export default Metrics;
