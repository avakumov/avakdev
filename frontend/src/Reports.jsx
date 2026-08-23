import { useState } from "react";
import { useReports, updateReport } from "./api.js";
import { useQueryClient } from "@tanstack/react-query";
import { formatDateRu } from "./lib/formatDate.js";
import MetricsTodayModal from "./MetricsTodayModal.jsx";

import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  ChevronDown,
  ChevronRight,
  Edit,
  Plus,
  Save,
  X,
  Loader2,
  CalendarDays,
  FileText,
  AlertCircle,
} from "lucide-react";

// Строка-обёртка над обычным textarea (в стилистике shadcn/ui).
function Textarea({ className, ...props }) {
  return (
    <textarea
      data-slot="textarea"
      className={
        "w-full min-h-28 rounded-lg border border-input bg-transparent px-3 py-2 text-sm leading-relaxed text-foreground transition-colors outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-input/30 resize-y " +
        (className || "")
      }
      {...props}
    />
  );
}

// Одна карточка отчёта: показывает день, превью и разворачивается по клику.
// В развёрнутом виде доступно редактирование.
function ReportRow({ report, open, onOpenChange, onSaved, onTodaySaved }) {
  const today = new Date().toISOString().slice(0, 10);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(report.content);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const handleSave = async () => {
    setSaving(true);
    setError("");
    try {
      await updateReport(report.date, draft);
      setEditing(false);
      onSaved();
      // После сохранения отчёта за сегодня предлагаем заполнить метрики.
      if (report.date === today) onTodaySaved?.();
    } catch (err) {
      setError(err.message || "Не удалось сохранить");
    } finally {
      setSaving(false);
    }
  };

  const toggle = () => {
    if (editing) return;
    onOpenChange(!open);
  };

  return (
    <Card className="my-3" size="sm">
      <CardHeader className="cursor-pointer select-none" onClick={toggle}>
        <div className="flex w-full items-center justify-between gap-2">
          <CardTitle className="flex items-center gap-2">
            {open ? (
              <ChevronDown className="size-4 text-muted-foreground" />
            ) : (
              <ChevronRight className="size-4 text-muted-foreground" />
            )}
            <CalendarDays className="size-4 text-muted-foreground" />
            {formatDateRu(report.date)}
          </CardTitle>
          {open && !editing && (
            <div onClick={(e) => e.stopPropagation()}>
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  setDraft(report.content);
                  setEditing(true);
                }}
              >
                <Edit />
                Редактировать
              </Button>
            </div>
          )}
        </div>
      </CardHeader>

      {open && (
        <CardContent>
          {editing ? (
            <div className="space-y-3">
              <Textarea
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                placeholder="Текст отчёта…"
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
              <div className="flex gap-2">
                <Button onClick={handleSave} disabled={saving}>
                  {saving ? <Loader2 className="animate-spin" /> : <Save />}
                  {saving ? "Сохраняю…" : "Сохранить"}
                </Button>
                <Button
                  variant="ghost"
                  onClick={() => {
                    setEditing(false);
                    setError("");
                  }}
                >
                  <X />
                  Отмена
                </Button>
              </div>
            </div>
          ) : report.content ? (
            <p className="whitespace-pre-wrap text-sm leading-relaxed text-foreground">
              {report.content}
            </p>
          ) : (
            <p className="text-sm text-muted-foreground">Отчёт пуст.</p>
          )}
        </CardContent>
      )}
    </Card>
  );
}

// Форма создания нового отчёта за конкретный день (по умолчанию — сегодня).
function NewReportForm({ onSaved, reports, onOpen, onTodaySaved }) {
  const today = new Date().toISOString().slice(0, 10);
  const [date, setDate] = useState(today);
  const [content, setContent] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const existingReport = reports?.find((r) => r.date === date);

  const handleSave = async () => {
    if (!date) {
      setError("Укажите дату");
      return;
    }
    setSaving(true);
    setError("");
    try {
      await updateReport(date, content);
      setContent("");
      onSaved();
      // После сохранения отчёта за сегодня предлагаем заполнить метрики.
      if (date === today) onTodaySaved?.();
    } catch (err) {
      setError(err.message || "Не удалось сохранить");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card className="my-3" size="sm">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Plus className="size-4 text-muted-foreground" />
          Новый отчёт за день
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="space-y-1.5">
          <Input
            type="date"
            value={date}
            max={today}
            onChange={(e) => setDate(e.target.value)}
          />
        </div>
        <Textarea
          value={content}
          onChange={(e) => setContent(e.target.value)}
          placeholder="Что произошло за этот день…"
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
        {existingReport && (
          <p className="flex items-center gap-1.5 text-sm text-amber-600">
            <AlertCircle className="size-4" />
            На эту дату уже есть отчёт. Сохранение перезапишет его.
          </p>
        )}
        <div className="flex flex-wrap gap-2">
          <Button onClick={handleSave} disabled={saving || !date}>
            {saving ? <Loader2 className="animate-spin" /> : <Save />}
            {saving
              ? "Сохраняю…"
              : existingReport
                ? "Заменить отчёт"
                : "Сохранить отчёт"}
          </Button>
          {existingReport && (
            <Button
              variant="outline"
              onClick={() => onOpen && onOpen(existingReport.date)}
            >
              <Edit />
              Открыть существующий
            </Button>
          )}
        </div>
      </CardContent>
    </Card>
  );
}

// Главный компонент: список дневных отчётов (сворачиваемые, редактируемые)
// + форма добавления нового отчёта.
function Reports() {
  const queryClient = useQueryClient();
  const reportsQuery = useReports(true);
  const [openDate, setOpenDate] = useState(null);
  // Модалка с метриками за сегодня — после сохранения сегодняшнего отчёта.
  const [metricsModalOpen, setMetricsModalOpen] = useState(false);
  const today = new Date().toISOString().slice(0, 10);

  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: ["reports"] });

  return (
    <section>
      <NewReportForm
        onSaved={refresh}
        reports={reportsQuery.data}
        onOpen={(date) => setOpenDate(date)}
        onTodaySaved={() => setMetricsModalOpen(true)}
      />

      {reportsQuery.isLoading ? (
        <p className="text-sm text-muted-foreground">Загрузка отчётов…</p>
      ) : reportsQuery.isError ? (
        <p className="text-sm text-destructive">
          Ошибка: {reportsQuery.error?.message}
        </p>
      ) : reportsQuery.data && reportsQuery.data.length > 0 ? (
        reportsQuery.data.map((r) => (
          <ReportRow
            key={r.date}
            report={r}
            open={openDate === r.date}
            onOpenChange={(v) => setOpenDate(v ? r.date : null)}
            onSaved={refresh}
            onTodaySaved={() => setMetricsModalOpen(true)}
          />
        ))
      ) : (
        <Card className="my-3" size="sm">
          <CardContent>
            <p className="flex items-center gap-2 text-sm text-muted-foreground">
              <FileText className="size-4" />
              Пока нет ни одного отчёта. Создайте первый выше.
            </p>
          </CardContent>
        </Card>
      )}

      {metricsModalOpen && (
        <MetricsTodayModal
          date={today}
          onClose={() => setMetricsModalOpen(false)}
        />
      )}
    </section>
  );
}

export default Reports;
