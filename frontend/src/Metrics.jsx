import { Fragment, useEffect, useRef, useState } from "react";
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
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
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

// Колонки календарной сетки: неделя с понедельника.
const WEEKDAYS = ["Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс"];

// Раскладывает даты (по возрастанию) в недели по 7 ячеек (Пн…Вс);
// null — пустая ячейка до начала/после конца диапазона.
function buildWeeks(dates) {
  const weeks = [];
  let week = null;
  const weekday = (iso) =>
    (new Date(iso + "T00:00:00Z").getUTCDay() + 6) % 7; // Пн = 0
  for (const d of dates) {
    if (!week || week.cells.length === 7) {
      week = { cells: [] };
      weeks.push(week);
      if (weeks.length === 1) {
        for (let i = 0; i < weekday(d); i += 1) week.cells.push(null);
      }
    }
    week.cells.push(d);
  }
  if (week) {
    while (week.cells.length < 7) week.cells.push(null);
  }
  return weeks;
}

// Название месяца по дате (с годом, если он не текущий).
function monthNameOf(iso, currentYear) {
  const name = MONTHS[Number(iso.slice(5, 7)) - 1] || "";
  const year = Number(iso.slice(0, 4));
  return year === currentYear ? name : `${name} ${year}`;
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

// Модалка значения метрики за конкретную дату: изменить или удалить.
function MetricValueModal({ def, date, value, onSaved, onClose }) {
  const [draft, setDraft] = useState(value ?? "");
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState("");

  const hasValue = value !== undefined && value !== "";
  const canSave =
    def.type === "bool"
      ? draft === "true" || draft === "false"
      : draft.trim() !== "";

  const handleSave = async () => {
    if (!canSave) return;
    setSaving(true);
    setError("");
    try {
      await setUserMetricValue(def.id, date, draft);
      onSaved();
      onClose();
    } catch (err) {
      setError(err.message || "Не удалось сохранить значение");
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    setDeleting(true);
    setError("");
    try {
      await deleteUserMetricValue(def.id, date);
      onSaved();
      onClose();
    } catch (err) {
      setError(err.message || "Не удалось удалить значение");
      setDeleting(false);
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      onClick={onClose}
    >
      <Card
        className="flex w-full max-w-sm flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <BarChart3 className="size-4 text-muted-foreground" />
            {def.name}
          </CardTitle>
          <CardDescription className="flex items-center gap-1.5">
            <CalendarDays className="size-3.5" />
            <DateDisplay date={date} />
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="space-y-1.5">
            <Label>Значение</Label>
            {def.type === "bool" ? (
              <BoolToggle value={draft} onChange={setDraft} />
            ) : (
              <Input
                type="number"
                step={def.type === "int" ? 1 : "any"}
                value={draft}
                autoFocus
                onChange={(e) => setDraft(e.target.value)}
                placeholder={def.type === "int" ? "Целое число" : "Число"}
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
        </CardContent>
        <div className="flex items-center justify-between gap-2 border-t p-4">
          <Button
            variant="destructive"
            onClick={handleDelete}
            disabled={!hasValue || saving || deleting}
            title={
              hasValue
                ? "Удалить значение за этот день"
                : "За этот день значения нет"
            }
          >
            {deleting ? <Loader2 className="animate-spin" /> : <Trash2 />}
            {deleting ? "Удаляю…" : "Удалить"}
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
            <Button onClick={handleSave} disabled={!canSave || saving || deleting}>
              {saving ? <Loader2 className="animate-spin" /> : <Save />}
              {saving ? "Сохраняю…" : "Сохранить"}
            </Button>
          </div>
        </div>
      </Card>
    </div>
  );
}

// Карточка метрики (мобильные). Два режима отображения значений:
// • «простое» (по умолчанию для да/нет) — последовательность цветных чисел:
//   цвет = значение (да/нет/пусто), число = длина серии одинаковых значений;
// • «подробное» — календарная сетка по дням (7 колонок — дни недели).
// Клик по элементу открывает модалку значения за этот день.
function MetricCard({ def, values, columns = [], onChanged, onEdit }) {
  // Свёрнута по умолчанию: видно название, тип и число дней.
  const [open, setOpen] = useState(false);
  // Дата, для которой открыта модалка значения.
  const [editDate, setEditDate] = useState(null);
  // Простое отображение — по умолчанию для да/нет.
  const [simple, setSimple] = useState(def.type === "bool");

  const daysCount = Object.keys(values).length;
  // Все даты — как в таблице, от первой (старой) к последней (сегодня).
  const dates = [...columns];

  // Подсказка с датой: hover/фокус (Radix) или долгое нажатие (тач).
  // Ключ подсказки: "d:<дата>" для плитки дня, "r:<дата>" для серии.
  const [tipKey, setTipKey] = useState(null);
  const pressTimer = useRef(null);
  const hideTimer = useRef(null);
  const longPressed = useRef(false);

  useEffect(
    () => () => {
      clearTimeout(pressTimer.current);
      clearTimeout(hideTimer.current);
    },
    [],
  );

  const startLongPress = (key) => {
    longPressed.current = false;
    clearTimeout(pressTimer.current);
    clearTimeout(hideTimer.current);
    pressTimer.current = setTimeout(() => {
      longPressed.current = true;
      setTipKey(key);
      hideTimer.current = setTimeout(() => setTipKey(null), 2000);
    }, 450);
  };

  const cancelLongPress = () => clearTimeout(pressTimer.current);

  // После долгого нажатия клик только скрывает подсказку (модалку не открываем).
  const handleTileClick = (d) => {
    if (longPressed.current) {
      longPressed.current = false;
      setTipKey(null);
      return;
    }
    setEditDate(d);
  };

  // Подробное отображение: каждый месяц — отдельный блок сетки (новая строка),
  // внутри месяца недели по 7 дней; более поздние недели — выше.
  const currentYear = new Date().getUTCFullYear();
  const monthBlocks = [];
  for (const d of dates) {
    const key = d.slice(0, 7);
    let block = monthBlocks[monthBlocks.length - 1];
    if (!block || block.key !== key) {
      block = { key, dates: [] };
      monthBlocks.push(block);
    }
    block.dates.push(d);
  }
  for (const block of monthBlocks) {
    block.label = monthNameOf(block.dates[0], currentYear);
    block.weeks = buildWeeks(block.dates).reverse();
  }
  // Месяцы — тоже от поздних к ранним.
  monthBlocks.reverse();

  // Плитка одного дня (значение + подсказка с датой).
  const renderTile = (d) => {
    const v = values[d];
    const has = v !== undefined && v !== "";
    const key = `d:${d}`;
    return (
      <Tooltip
        key={key}
        open={tipKey === key}
        onOpenChange={(v) =>
          setTipKey(v ? key : (prev) => (prev === key ? null : prev))
        }
      >
        <TooltipTrigger asChild>
          <button
            type="button"
            onClick={() => handleTileClick(d)}
            onPointerDown={() => startLongPress(key)}
            onPointerUp={cancelLongPress}
            onPointerLeave={cancelLongPress}
            onPointerCancel={cancelLongPress}
            onContextMenu={(e) => e.preventDefault()}
            aria-label={`${def.name} за ${formatDateDmy(d)}`}
            className={cn(
              "flex size-10 touch-manipulation select-none items-center justify-center border text-xs font-medium tabular-nums transition-colors",
              !has &&
                "border-border/60 text-muted-foreground/60 hover:bg-muted/50",
              has &&
                def.type === "bool" &&
                (v === "true"
                  ? "border-emerald-500/40 bg-emerald-500/15 text-emerald-700 hover:bg-emerald-500/25 dark:text-emerald-400"
                  : "border-destructive/40 bg-destructive/15 text-destructive hover:bg-destructive/25"),
              has &&
                def.type !== "bool" &&
                "border-border/60 bg-muted/40 text-foreground hover:bg-muted/60",
            )}
          >
            {!has ? "—" : def.type === "bool" ? (v === "true" ? "Д" : "Н") : v}
          </button>
        </TooltipTrigger>
        <TooltipContent>{formatDateDmy(d)}</TooltipContent>
      </Tooltip>
    );
  };

  // Простое отображение: серии одинаковых значений подряд — от новых к старым.
  // Цвет — значение (да/нет/пусто), число — длина серии.
  // Пустой сегодняшний день в последовательность не берём (день ещё не заполнен).
  const todayIso = new Date().toISOString().slice(0, 10);
  const runs = [];
  for (const d of [...dates].reverse()) {
    const v = values[d];
    const state = v === "true" ? "yes" : v === "false" ? "no" : "empty";
    if (state === "empty" && d === todayIso) continue;
    const last = runs[runs.length - 1];
    if (last && last.state === state) {
      last.count += 1;
      if (d < last.min) last.min = d;
      if (d > last.max) last.max = d;
    } else {
      runs.push({ state, count: 1, min: d, max: d });
    }
  }

  const renderRun = (r) => {
    const key = `r:${r.max}`;
    const range =
      r.min === r.max
        ? formatDateDmy(r.min)
        : `${formatDateDmy(r.min)} — ${formatDateDmy(r.max)}`;
    // Логарифмический рост: 1 → как сейчас (0.875rem), 1000 и больше → в 5 раз
    // крупнее. Растёт быстро на малых значениях и медленнее на больших.
    const t = Math.min(
      Math.max(Math.log(Math.max(r.count, 1)) / Math.log(1000), 0),
      1,
    );
    const fontSize = `${(0.875 * (1 + 4 * t)).toFixed(3)}rem`;
    // Насыщенность тоже растёт со значением: 600 → 900.
    const fontWeight = Math.round(600 + 300 * t);
    return (
      <Tooltip
        key={key}
        open={tipKey === key}
        onOpenChange={(v) =>
          setTipKey(v ? key : (prev) => (prev === key ? null : prev))
        }
      >
        <TooltipTrigger asChild>
          <button
            type="button"
            onClick={() => handleTileClick(r.max)}
            onPointerDown={() => startLongPress(key)}
            onPointerUp={cancelLongPress}
            onPointerLeave={cancelLongPress}
            onPointerCancel={cancelLongPress}
            onContextMenu={(e) => e.preventDefault()}
            aria-label={`${range}: ${r.count}`}
            className={cn(
              "touch-manipulation select-none px-0.5 tabular-nums transition-opacity hover:opacity-70",
              r.state === "yes" && "text-emerald-700 dark:text-emerald-400",
              r.state === "no" && "text-destructive",
              r.state === "empty" && "text-muted-foreground/60",
            )}
            style={{ fontSize, fontWeight, lineHeight: 1.1 }}
          >
            {r.count}
          </button>
        </TooltipTrigger>
        <TooltipContent>{range}</TooltipContent>
      </Tooltip>
    );
  };

  return (
    <>
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
          <CardContent>
            {dates.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                Пока нет ни одного дня для отображения.
              </p>
            ) : (
              <>
                {/* Переключение «простое» / «подробное» (для да/нет). */}
                {def.type === "bool" && (
                  <div className="mb-2 flex justify-end">
                    <Button
                      variant="outline"
                      size="sm"
                      className="bg-background"
                      onClick={() => setSimple((s) => !s)}
                    >
                      {simple ? "Подробнее…" : "Простое…"}
                    </Button>
                  </div>
                )}

                {def.type === "bool" && simple ? (
                  <div className="mx-auto flex w-fit flex-wrap items-center justify-center gap-x-2 gap-y-0.5">
                    {runs.map(renderRun)}
                  </div>
                ) : (
                  <div className="mx-auto w-fit">
                    {/* Шапка: пустая ячейка под месяц + дни недели */}
                    <div className="grid grid-cols-[3.25rem_repeat(7,2.5rem)] items-center gap-1">
                      <span aria-hidden="true" />
                      {WEEKDAYS.map((w) => (
                        <span
                          key={w}
                          className="text-center text-[10px] text-muted-foreground"
                        >
                          {w}
                        </span>
                      ))}
                    </div>

                    {/* Каждый месяц — отдельным блоком-строкой */}
                    <div className="mt-1 space-y-1">
                      {monthBlocks.map((block) => (
                        <div
                          key={block.key}
                          className="grid grid-cols-[3.25rem_repeat(7,2.5rem)] items-center gap-1"
                        >
                          {block.weeks.map((wk, wi) => (
                            <Fragment key={wi}>
                              <span className="pr-1 text-right text-[10px] leading-tight text-muted-foreground capitalize">
                                {wi === 0 ? block.label : ""}
                              </span>
                              {wk.cells.map((d, ci) =>
                                d === null ? (
                                  <span key={`empty-${ci}`} aria-hidden="true" />
                                ) : (
                                  renderTile(d)
                                ),
                              )}
                            </Fragment>
                          ))}
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </>
            )}
          </CardContent>
        )}
      </Card>

      {editDate && (
        <MetricValueModal
          def={def}
          date={editDate}
          value={values[editDate]}
          onSaved={onChanged}
          onClose={() => setEditDate(null)}
        />
      )}
    </>
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
                columns={columns}
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
              columns={columns}
              onChanged={refresh}
              onEdit={setEditTarget}
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
