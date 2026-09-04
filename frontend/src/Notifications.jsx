import { useState } from "react";
import {
  useNotifications,
  useNotificationInbox,
  createNotification,
  deleteNotification,
  dismissNotification,
} from "./api.js";
import { useQueryClient } from "@tanstack/react-query";
import DateDisplay from "@/components/DateDisplay.jsx";
import DateInput from "@/components/DateInput.jsx";
import TimeInput from "@/components/TimeInput.jsx";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
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
  Bell,
  BellRing,
  Plus,
  Save,
  Trash2,
  Loader2,
  AlertCircle,
  X,
  Repeat,
  Send,
  Inbox,
  Check,
} from "lucide-react";

// Русская плюрализация: ruPlural(5, ["день","дня","дней"]) -> "дней".
function ruPlural(n, [one, few, many]) {
  const d10 = n % 10;
  const d100 = n % 100;
  if (d10 === 1 && d100 !== 11) return one;
  if (d10 >= 2 && d10 <= 4 && (d100 < 12 || d100 > 14)) return few;
  return many;
}

// Читаемая подпись периода: (unit, value) -> "1 месяц", "2 года"…
function formatPeriod(unit, value) {
  const n = Number(value) || 1;
  switch (unit) {
    case "minute":
      return `${n} ${ruPlural(n, ["минуту", "минуты", "минут"])}`;
    case "hour":
      return `${n} ${ruPlural(n, ["час", "часа", "часов"])}`;
    case "day":
      return `${n} ${ruPlural(n, ["день", "дня", "дней"])}`;
    case "month":
      return `${n} ${ruPlural(n, ["месяц", "месяца", "месяцев"])}`;
    case "year":
      return `${n} ${ruPlural(n, ["год", "года", "лет"])}`;
    default:
      return String(n);
  }
}

// Единицы периода для формы.
const PERIOD_UNITS = [
  { value: "minute", label: "Минут" },
  { value: "hour", label: "Часов" },
  { value: "day", label: "Дней" },
  { value: "month", label: "Месяцев" },
  { value: "year", label: "Лет" },
];

const TYPE_LABELS = {
  once: "Однократно",
  periodic: "Периодически",
};

const CHANNEL_LABELS = {
  app: "Уведомления",
  telegram: "Telegram",
};

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

// Модалка создания уведомления: тип (однократно/периодически), когда/период,
// канал доставки (уведомления/Telegram).
export function CreateNotificationModal({ onClose, onCreated }) {
  const [text, setText] = useState("");
  const [type, setType] = useState("once");
  const [whenDate, setWhenDate] = useState(""); // дата (DateInput) для однократного
  const [whenTime, setWhenTime] = useState("09:00"); // время для однократного
  const [periodValue, setPeriodValue] = useState("1");
  const [periodUnit, setPeriodUnit] = useState("day"); // minute|hour|day|month|year
  const [anchorDate, setAnchorDate] = useState(""); // дата для месяц/год
  const [channel, setChannel] = useState("app");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  // Минуты в одном value для быстрых единиц (минуты/часы/дни).
  const unitMinutes = {
    minute: 1,
    hour: 60,
    day: 1440,
  };

  const handleSave = async () => {
    setError("");
    const payload = { text, type, channel };

    if (type === "once") {
      if (!whenDate) {
        setError("Укажите дату срабатывания");
        return;
      }
      const time = whenTime || "09:00";
      payload.due_at = new Date(`${whenDate}T${time}:00`).toISOString();
      payload.period_unit = "";
      payload.period_value = 0;
    } else {
      const n = Math.max(1, Math.floor(Number(periodValue) || 1));
      payload.period_value = n;
      payload.period_unit = periodUnit;

      if (periodUnit === "month" || periodUnit === "year") {
        // Месяц/год: нужна дата привязки — день (и месяц) будут повторяться.
        if (!anchorDate) {
          setError("Укажите дату начала повторения");
          return;
        }
        payload.due_at = new Date(`${anchorDate}T09:00:00`).toISOString();
      } else {
        const stepMin = (unitMinutes[periodUnit] || 1) * n;
        payload.due_at = new Date(Date.now() + stepMin * 60000).toISOString();
      }
    }

    setSaving(true);
    try {
      await createNotification(payload);
      onCreated();
      onClose();
    } catch (err) {
      setError(err.message || "Не удалось создать уведомление");
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
            <BellRing className="size-4 text-muted-foreground" />
            Новое уведомление
          </CardTitle>
          <CardDescription>
            Тип, время срабатывания и способ доставки
          </CardDescription>
        </CardHeader>

        <CardContent className="flex-1 space-y-4 overflow-y-auto">
          <div className="space-y-1.5">
            <Label htmlFor="notif-text">Текст уведомления</Label>
            <Textarea
              id="notif-text"
              value={text}
              onChange={(e) => setText(e.target.value)}
              placeholder="Например: выпить лекарство, сделать зарядку…"
            />
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label>Тип</Label>
              <Select value={type} onValueChange={setType}>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="once">Однократно</SelectItem>
                  <SelectItem value="periodic">Периодически</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label>Доставка</Label>
              <Select value={channel} onValueChange={setChannel}>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="app">Уведомления</SelectItem>
                  <SelectItem value="telegram">Telegram</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          {type === "once" ? (
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-1.5">
                <Label htmlFor="notif-when-date">Когда — дата</Label>
                <DateInput
                  id="notif-when-date"
                  value={whenDate}
                  onChange={setWhenDate}
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="notif-when-time">Время</Label>
                <TimeInput
                  id="notif-when-time"
                  value={whenTime}
                  onChange={setWhenTime}
                />
              </div>
            </div>
          ) : (
            <div className="flex items-end gap-2">
              <div className="w-24 space-y-1.5">
                <Label htmlFor="notif-period-value">Каждые</Label>
                <Input
                  id="notif-period-value"
                  type="number"
                  min="1"
                  value={periodValue}
                  onChange={(e) => setPeriodValue(e.target.value)}
                />
              </div>
              <div className="flex-1 space-y-1.5">
                <Label>Период</Label>
                <Select value={periodUnit} onValueChange={setPeriodUnit}>
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {PERIOD_UNITS.map((u) => (
                      <SelectItem key={u.value} value={u.value}>
                        {u.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
          )}

          {(type === "periodic" &&
            (periodUnit === "month" || periodUnit === "year")) && (
            <div className="space-y-1.5">
              <Label htmlFor="notif-anchor-date">
                {periodUnit === "month"
                  ? "Дата начала (повтор — каждый месяц в этот день)"
                  : "Дата начала (повтор — каждый год в этот день)"}
              </Label>
              <DateInput
                id="notif-anchor-date"
                value={anchorDate}
                onChange={setAnchorDate}
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
          <Button onClick={handleSave} disabled={saving || !text.trim()}>
            {saving ? <Loader2 className="animate-spin" /> : <Save />}
            {saving ? "Сохраняю…" : "Добавить"}
          </Button>
        </div>
      </Card>
    </div>
  );
}

// Список уведомлений (используется на странице профиля и в модалке колокольчика).
function NotificationRows({ onChanged }) {
  const query = useNotifications(true);

  const remove = async (id) => {
    if (!window.confirm("Удалить уведомление?")) return;
    try {
      await deleteNotification(id);
      onChanged();
    } catch (err) {
      window.alert(err.message || "Не удалось удалить уведомление");
    }
  };

  if (query.isLoading) {
    return (
      <p className="flex items-center gap-2 text-sm text-muted-foreground">
        <Loader2 className="size-4 animate-spin" />
        Загрузка уведомлений…
      </p>
    );
  }
  if (query.isError) {
    return (
      <p className="text-sm text-destructive">
        Ошибка: {query.error?.message}
      </p>
    );
  }

  const items = query.data?.notifications || [];

  if (items.length === 0) {
    return (
      <p className="flex items-center gap-2 text-sm text-muted-foreground">
        <Bell className="size-4" />
        Уведомлений пока нет
      </p>
    );
  }

  return (
    <ul className="divide-y rounded-lg border">
      {items.map((n) => (
        <li
          key={n.id}
          className="flex items-start justify-between gap-3 px-3 py-2 text-sm"
        >
          <div className="min-w-0 space-y-1">
            <p className="wrap-break-word font-medium text-foreground">
              {n.text}
            </p>
            <div className="flex flex-wrap items-center gap-1.5 text-xs">
              <Badge variant="outline">
                {n.type === "periodic" ? (
                  <Repeat className="mr-1 size-3" />
                ) : null}
                {TYPE_LABELS[n.type] || n.type}
                {n.type === "periodic" &&
                  ` · каждые ${formatPeriod(n.period_unit, n.period_value)}`}
              </Badge>
              <Badge variant="secondary">
                {n.channel === "telegram" ? (
                  <Send className="mr-1 size-3" />
                ) : (
                  <Inbox className="mr-1 size-3" />
                )}
                {CHANNEL_LABELS[n.channel] || n.channel}
              </Badge>
              <span className="flex items-center gap-1 text-muted-foreground">
                {n.type === "periodic" ? "следующее: " : ""}
                <DateDisplay date={n.due_at} withTime className="tabular-nums" />
              </span>
            </div>
          </div>
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={() => remove(n.id)}
            aria-label="Удалить уведомление"
            className="shrink-0 text-muted-foreground hover:text-destructive"
          >
            <Trash2 />
          </Button>
        </li>
      ))}
    </ul>
  );
}

// Модалка колокольчика: показывает ТОЛЬКО наступившие уведомления
// («входящие») — они появляются по расписанию, а не все сразу.
export function NotificationsModal({ onClose }) {
  const queryClient = useQueryClient();

  const inbox = useNotificationInbox(true);

  const refreshAll = () => {
    queryClient.invalidateQueries({ queryKey: ["notifications-inbox"] });
    queryClient.invalidateQueries({ queryKey: ["notifications"] });
  };

  const dismiss = async (id) => {
    try {
      await dismissNotification(id);
      refreshAll();
    } catch (err) {
      window.alert(err.message || "Не удалось закрыть уведомление");
    }
  };

  const items = inbox.data?.inbox || [];

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
          <div className="flex items-start justify-between gap-2">
            <CardTitle className="flex items-center gap-2">
              <BellRing className="size-4 text-muted-foreground" />
              Уведомления
            </CardTitle>
            <Button
              variant="ghost"
              size="icon"
              onClick={onClose}
              aria-label="Закрыть"
              className="-mt-1 -mr-1"
            >
              <X />
            </Button>
          </div>
          <CardDescription>
            Появляются по расписанию. Нажмите «Готово», чтобы закрыть.
          </CardDescription>
        </CardHeader>

        <CardContent className="flex-1 space-y-3 overflow-y-auto">
          {inbox.isLoading ? (
            <p className="flex items-center gap-2 text-sm text-muted-foreground">
              <Loader2 className="size-4 animate-spin" />
              Загрузка…
            </p>
          ) : inbox.isError ? (
            <p className="text-sm text-destructive">
              Ошибка: {inbox.error?.message}
            </p>
          ) : items.length === 0 ? (
            <p className="flex items-center gap-2 text-sm text-muted-foreground">
              <Bell className="size-4" />
              Пока нет новых уведомлений
            </p>
          ) : (
            <ul className="divide-y rounded-lg border">
              {items.map((it) => (
                <li
                  key={it.id}
                  className="flex items-start justify-between gap-3 px-3 py-2 text-sm"
                >
                  <div className="min-w-0 space-y-1">
                    <p className="wrap-break-word font-medium text-foreground">
                      {it.text}
                    </p>
                    <span className="flex items-center gap-1 text-xs text-muted-foreground">
                      появилось:{" "}
                      <DateDisplay date={it.created} withTime className="tabular-nums" />
                    </span>
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => dismiss(it.id)}
                    className="shrink-0"
                  >
                    <Check />
                    Готово
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

// Карточка «Уведомления» для страницы профиля: добавление — над списком.
export function NotificationsCard({ onAdd }) {
  const queryClient = useQueryClient();
  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: ["notifications"] });

  return (
    <Card className="my-3" size="sm">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <BellRing className="size-4 text-muted-foreground" />
          Уведомления
        </CardTitle>
        <CardDescription>
          Напоминания, которые вы добавляете себе
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <Button onClick={onAdd}>
          <Plus />
          Добавить уведомление
        </Button>
        <NotificationRows onChanged={refresh} />
      </CardContent>
    </Card>
  );
}
