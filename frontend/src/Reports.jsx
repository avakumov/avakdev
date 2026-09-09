import { useEffect, useState } from "react";
import {
  useReports,
  useUserMetrics,
  fetchDayPlan,
  fetchDayHistory,
} from "./api.js";
import DateDisplay from "@/components/DateDisplay.jsx";

import { Card, CardHeader, CardContent } from "@/components/ui/card";
import {
  ChevronDown,
  ChevronRight,
  Loader2,
  CalendarDays,
  FileText,
  BarChart3,
  ListTodo,
  BookOpen,
  CheckCircle2,
} from "lucide-react";
import { cn } from "@/lib/utils";

// Минуты: 65 -> "1 ч 5 мин".
const fmtMin = (m) => {
  if (!m) return "0 мин";
  if (m < 60) return `${m} мин`;
  const h = Math.floor(m / 60);
  const r = m % 60;
  return r ? `${h} ч ${r} мин` : `${h} ч`;
};

// Значение метрики (Да/Нет) в виде «залитой» плашки, как активная кнопка в Дне.
function MetricValue({ def, value }) {
  if (value == null || value === "") {
    return <span className="text-sm text-muted-foreground">—</span>;
  }
  if (def.type === "bool") {
    const yes = value === "true";
    return (
      <span
        className={cn(
          "inline-flex min-w-14 items-center justify-center gap-1 rounded-md px-2 py-0.5 text-xs font-medium text-white",
          yes
            ? "bg-emerald-600"
            : "bg-destructive",
        )}
      >
        {yes && <CheckCircle2 className="size-3.5" />}
        {yes ? "Да" : "Нет"}
      </span>
    );
  }
  return (
    <span className="text-sm font-medium tabular-nums">{value}</span>
  );
}

// Один день в отчётах — «снимок» дня как на странице «День», но без
// редактирования: план (задачи/повторения), метрики и текст отчёта.
function DayCard({ date, day, report, open, onOpenChange }) {
  const [items, setItems] = useState(null); // null — ещё не загружено
  const metricsQuery = useUserMetrics(true);

  // Состав дня подгружаем только при раскрытии карточки.
  useEffect(() => {
    if (!open || items !== null) return;
    if (!day) {
      setItems([]);
      return;
    }
    fetchDayPlan(date)
      .then((p) => setItems(p.items || []))
      .catch(() => setItems([]));
  }, [open, date, day, items]);

  const toggle = () => onOpenChange(!open);
  const hasContent = Boolean(report?.content);

  const defs = metricsQuery.data?.definitions || [];
  const values = metricsQuery.data?.values || [];
  const valueFor = (id) =>
    values.find((v) => v.metric_id === id && v.date === date)?.value;

  const doneCount = (items || []).filter((i) => i.done).length;

  return (
    <Card className="my-3" size="sm">
      <CardHeader className="cursor-pointer select-none" onClick={toggle}>
        <div className="flex w-full items-center gap-2">
          {open ? (
            <ChevronDown className="size-4 text-muted-foreground" />
          ) : (
            <ChevronRight className="size-4 text-muted-foreground" />
          )}
          <CalendarDays className="size-4 text-muted-foreground" />
          <DateDisplay date={date} />
          {hasContent && (
            <FileText className="size-4 text-emerald-600 dark:text-emerald-400" />
          )}
        </div>

        {/* Сводка дня — как шапка «Дня» */}
        <p className="flex flex-wrap items-center gap-x-2 text-xs text-muted-foreground">
          {day ? (
            <>
              <span>
                {doneCount > 0
                  ? `выполнено ${doneCount} из ${day.tasks + day.notes}`
                  : `${day.tasks} задач · ${day.notes} повт.`}
              </span>
              <span>· {fmtMin(day.total_minutes)}</span>
              <span>· бюджет {fmtMin(day.budget_minutes)}</span>
            </>
          ) : (
            <span>Плана на этот день не было</span>
          )}
        </p>
      </CardHeader>

      {open && (
        <CardContent className="space-y-3">
          {/* План дня — строки как в «Дне» */}
          <div className="space-y-1.5">
            <p className="flex items-center gap-1.5 text-sm font-medium">
              <ListTodo className="size-4 text-muted-foreground" />
              План дня
            </p>
            {!day ? (
              <p className="text-sm text-muted-foreground">
                Плана на этот день не было.
              </p>
            ) : items === null ? (
              <p className="text-sm text-muted-foreground">
                <Loader2 className="mr-1 inline size-4 animate-spin" />
                Загрузка…
              </p>
            ) : items.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                План дня был пуст.
              </p>
            ) : (
              items.map((it, i) => (
                <div
                  key={`${it.kind}-${it.ref_id}-${i}`}
                  className={cn(
                    "flex w-full items-start gap-2.5 rounded-lg border px-3 py-2 transition-colors",
                    it.done
                      ? "border-emerald-500/50 bg-emerald-500/10"
                      : "border-border/60",
                  )}
                >
                  {it.kind === "task" ? (
                    <ListTodo className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
                  ) : (
                    <BookOpen className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
                  )}
                  <span className="min-w-0 flex-1">
                    <span className="wrap-break-word block text-sm font-medium text-foreground">
                      {it.title}
                    </span>
                    {it.meta && (
                      <span className="block text-xs text-muted-foreground">
                        {it.meta}
                      </span>
                    )}
                    {it.done && (
                      <span className="mt-0.5 flex items-center gap-1 text-xs font-medium text-emerald-600 dark:text-emerald-400">
                        <CheckCircle2 className="size-3.5" />
                        {it.kind === "task" ? "Готова" : "Повторено"}
                      </span>
                    )}
                  </span>
                  <span className="shrink-0 text-xs font-medium tabular-nums text-muted-foreground">
                    {fmtMin(it.minutes)}
                  </span>
                </div>
              ))
            )}
          </div>

          {/* Метрики за день — как в «Дне» (зелёные, если проставлены) */}
          {defs.length > 0 && (
            <div className="space-y-1.5">
              <p className="flex items-center gap-1.5 text-sm font-medium">
                <BarChart3 className="size-4 text-muted-foreground" />
                Метрики за день
              </p>
              {defs.map((def) => {
                const value = valueFor(def.id);
                const hasValue = value != null && value !== "";
                return (
                  <div
                    key={def.id}
                    className={cn(
                      "flex items-center justify-between gap-2 rounded-lg border px-3 py-2 transition-colors",
                      hasValue
                        ? "border-emerald-500/50 bg-emerald-500/10"
                        : "border-border/60",
                    )}
                  >
                    <span className="min-w-0 text-sm font-medium">
                      {def.name}
                      {def.type !== "bool" && def.unit && (
                        <span className="text-muted-foreground">
                          {" "}
                          {def.unit}
                        </span>
                      )}
                    </span>
                    <MetricValue def={def} value={value} />
                  </div>
                );
              })}
            </div>
          )}

          {/* Текст отчёта — только просмотр */}
          <div className="space-y-1.5">
            <p className="flex items-center gap-1.5 text-sm font-medium">
              <FileText className="size-4 text-muted-foreground" />
              Отчёт за день
            </p>
            {hasContent ? (
              <div className="rounded-lg border border-border/60 px-3 py-2">
                <p className="whitespace-pre-wrap text-sm leading-relaxed">
                  {report.content}
                </p>
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">
                Отчёта за этот день не было.
              </p>
            )}
          </div>
        </CardContent>
      )}
    </Card>
  );
}

// Главный компонент: «Отчёты» — снимки прошедших дней (план + метрики +
// текст отчёта), только для просмотра. День появляется здесь, если на него
// есть сохранённый план (раздел «День») или текст отчёта.
function Reports() {
  const reportsQuery = useReports(true);
  const [history, setHistory] = useState(null);
  const [openDate, setOpenDate] = useState(null);

  useEffect(() => {
    fetchDayHistory()
      .then((d) => setHistory(d.days || []))
      .catch(() => {});
  }, []);

  // Объединяем даты из планов дней и текстовых отчётов.
  const days = [];
  const seen = new Set();
  for (const d of history || []) {
    if (!seen.has(d.date)) {
      seen.add(d.date);
      days.push({ date: d.date, day: d, report: null });
    }
  }
  for (const r of reportsQuery.data || []) {
    const ex = days.find((x) => x.date === r.date);
    if (ex) {
      ex.report = r;
    } else {
      seen.add(r.date);
      days.push({ date: r.date, day: null, report: r });
    }
  }
  days.sort((a, b) => (a.date < b.date ? 1 : -1));

  return (
    <section>
      <h2 className="flex items-center gap-2 text-xl font-semibold">
        <FileText className="size-5 text-muted-foreground" />
        Отчёты
        {history === null && (
          <Loader2 className="size-4 animate-spin text-muted-foreground" />
        )}
        <span className="text-sm font-normal text-muted-foreground">
          · дни
        </span>
      </h2>

      {days.length === 0 ? (
        <Card className="my-3" size="sm">
          <CardContent>
            {reportsQuery.isLoading ? (
              <p className="text-sm text-muted-foreground">Загрузка…</p>
            ) : (
              <p className="flex items-center gap-2 text-sm text-muted-foreground">
                <CalendarDays className="size-4" />
                Пока нет ни одного дня. Сформируйте день в разделе «День» —
                он появится здесь.
              </p>
            )}
          </CardContent>
        </Card>
      ) : (
        days.map(({ date, day, report }) => (
          <DayCard
            key={date}
            date={date}
            day={day}
            report={report}
            open={openDate === date}
            onOpenChange={(v) => setOpenDate(v ? date : null)}
          />
        ))
      )}
    </section>
  );
}

export default Reports;
