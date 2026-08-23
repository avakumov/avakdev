import { useState } from "react";
import { useUserMetrics, setUserMetricValue } from "./api.js";
import { useQueryClient } from "@tanstack/react-query";
import { formatDateRu } from "./lib/formatDate.js";
import { BoolToggle } from "./Metrics.jsx";

import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardDescription,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Loader2, Save, X, AlertCircle, CalendarDays, BarChart3 } from "lucide-react";

// Модальное окно заполнения метрик за конкретный день (показывается после
// сохранения отчёта за сегодняшний день). Дата показывается один раз сверху.
// Каждая метрика — строка с именем и полем значения (число или «Да / Нет»);
// сохраняются только заполненные значения, повторное сохранение за тот же
// день перезаписывает показатель.
function MetricsTodayModal({ date, onClose }) {
  const queryClient = useQueryClient();
  const metricsQuery = useUserMetrics(true);

  // Черновик правок пользователя: metric_id -> новое значение.
  const [draft, setDraft] = useState({});
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const data = metricsQuery.data;
  const defs = data?.definitions || [];

  // Значение метрики за выбранный день из сохранённых данных.
  const savedValue = (defId) =>
    data?.values?.find((v) => v.metric_id === defId && v.date === date)?.value ||
    "";

  // Текущее значение в форме: правка пользователя или сохранённое.
  const currentValue = (defId) => draft[defId] ?? savedValue(defId);

  const handleSave = async () => {
    setSaving(true);
    setError("");
    try {
      for (const def of defs) {
        const value = currentValue(def.id);
        if (value === "") continue;
        await setUserMetricValue(def.id, date, value);
      }
      queryClient.invalidateQueries({ queryKey: ["user-metrics"] });
      onClose();
    } catch (err) {
      setError(err.message || "Не удалось сохранить метрики");
      setSaving(false);
    }
  };

  const isLoading = metricsQuery.isLoading;
  const isError = metricsQuery.isError;

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
            <span className="flex size-8 items-center justify-center rounded-lg bg-primary text-primary-foreground">
              <BarChart3 className="size-4" />
            </span>
            Метрики за день
          </CardTitle>
          <CardDescription className="flex items-center gap-1.5">
            <CalendarDays className="size-3.5" />
            {formatDateRu(date)}
          </CardDescription>
        </CardHeader>

        <CardContent className="flex-1 space-y-4 overflow-y-auto">
          {isLoading ? (
            <p className="flex items-center gap-2 text-sm text-muted-foreground">
              <Loader2 className="size-4 animate-spin" />
              Загрузка метрик…
            </p>
          ) : isError ? (
            <p className="text-sm text-destructive">
              Ошибка: {metricsQuery.error?.message}
            </p>
          ) : defs.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              У вас пока нет метрик. Добавьте их в разделе «Метрики».
            </p>
          ) : (
            <div className="space-y-3">
              {defs.map((def) => (
                <div
                  key={def.id}
                  className="flex flex-wrap items-center justify-between gap-3 rounded-lg border p-3"
                >
                  <div className="min-w-0">
                    <p className="text-sm font-medium">{def.name}</p>
                    {def.unit && (
                      <p className="text-xs text-muted-foreground">{def.unit}</p>
                    )}
                  </div>
                  {def.type === "bool" ? (
                    <BoolToggle
                      value={currentValue(def.id)}
                      onChange={(v) =>
                        setDraft((d) => ({ ...d, [def.id]: v }))
                      }
                    />
                  ) : (
                    <Input
                      type="number"
                      step={def.type === "int" ? 1 : "any"}
                      value={currentValue(def.id)}
                      onChange={(e) =>
                        setDraft((d) => ({ ...d, [def.id]: e.target.value }))
                      }
                      placeholder={def.type === "int" ? "Целое число" : "Число"}
                      className="w-32"
                    />
                  )}
                </div>
              ))}
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

        {!isLoading && !isError && (
          <div className="flex justify-end gap-2 border-t p-4">
            <Button variant="ghost" onClick={onClose} disabled={saving}>
              <X />
              Отмена
            </Button>
            <Button onClick={handleSave} disabled={saving}>
              {saving ? <Loader2 className="animate-spin" /> : <Save />}
              {saving ? "Сохраняю…" : "Сохранить"}
            </Button>
          </div>
        )}
      </Card>
    </div>
  );
}

export default MetricsTodayModal;
