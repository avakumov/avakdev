import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { useFeedItems, markFeedItemShown, reactToFeedItem } from "./api.js";
import { useSwipeNav } from "./lib/useSwipeNav.js";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Loader2, AlertCircle, ThumbsUp, ThumbsDown } from "lucide-react";
import { cn } from "@/lib/utils";

// Пределы адаптивного кегля (px) и запас (px), который вычитаем из высоты
// окна помимо полей карточки и строки кнопок: люфт на расхождения dvh/innerHeight
// и отступ между текстом и кнопками.
const MIN_FONT = 11;
const MAX_FONT = 32;
const FONT_MARGIN = 48 + 12 + 8;

// Один элемент ленты: сначала вопрос, ответ — после касания.
//
// Кегль подбирается под экран: ищем максимальный размер, при котором вопрос и
// ответ целиком помещаются в один экран (вместе с нижней строкой кнопок).
// Меряем невидимый дубль, в котором всегда есть и вопрос, и ответ, — поэтому
// размер не «прыгает» при раскрытии ответа.
//
// При показе элемента отмечаем показ на сервере (счётчик «показов»);
// ref-защёлка не даёт посчитать один показ дважды (в т.ч. в StrictMode).
// Пересоздаётся по key=id (см. Feed), поэтому состояние «раскрыт» и реакции
// сбрасываются сами при переходе к следующему элементу.
function FeedItem({ item }) {
  const [open, setOpen] = useState(false);
  const [font, setFont] = useState(MAX_FONT);
  const counted = useRef(false);
  const measureRef = useRef(null);
  const rowRef = useRef(null);

  // Реакция текущего показа: одна за показ и только одна из двух кнопок.
  const [voted, setVoted] = useState(null); // "know" | "unknown" | null
  const [counts, setCounts] = useState({
    know: item.know_count || 0,
    unknown: item.unknown_count || 0,
  });

  useEffect(() => {
    if (counted.current) return;
    counted.current = true;
    // Показ — фоновая отметка: ошибка не должна ломать чтение ленты.
    markFeedItemShown(item.id).catch(() => {});
  }, [item.id]);

  // Подбор кегля: двоичный поиск по размеру с замером высоты дубля.
  // useLayoutEffect — замер и перерисовка происходят до отрисовки кадра,
  // поэтому промежуточный (слишком крупный) размер не мелькает.
  useLayoutEffect(() => {
    const box = measureRef.current;
    if (!box) return;

    const fit = () => {
      // Высота строки кнопок меряется по факту: на узком экране подписи
      // могут переноситься, и тогда она выше.
      const rowHeight = rowRef.current?.offsetHeight || 0;
      const available = window.innerHeight - FONT_MARGIN - rowHeight;
      let lo = MIN_FONT;
      let hi = MAX_FONT;
      let best = MIN_FONT;
      while (hi - lo > 0.5) {
        const mid = (lo + hi) / 2;
        box.style.fontSize = `${mid}px`;
        if (box.scrollHeight <= available) {
          best = mid;
          lo = mid;
        } else {
          hi = mid;
        }
      }
      box.style.fontSize = `${best}px`;
      setFont(best);
    };

    fit();
    window.addEventListener("resize", fit);
    window.addEventListener("orientationchange", fit);
    return () => {
      window.removeEventListener("resize", fit);
      window.removeEventListener("orientationchange", fit);
    };
  }, [item.id, item.question, item.answer]);

  // Отметка «знаю»/«не знаю»: оптимистично обновляем счётчик, при ошибке
  // откатываем и разрешаем нажать снова.
  const react = async (value) => {
    if (voted) return;
    setVoted(value);
    setCounts((c) => ({ ...c, [value]: c[value] + 1 }));
    try {
      await reactToFeedItem(item.id, value);
    } catch {
      setVoted(null);
      setCounts((c) => ({ ...c, [value]: Math.max(0, c[value] - 1) }));
    }
  };

  return (
    <div className="relative flex min-h-dvh w-full flex-col bg-card p-6">
      {/* Невидимый дубль для замера: всегда содержит вопрос и ответ целиком
          (по нему считается кегль; на экран не влияет). */}
      <span
        ref={measureRef}
        aria-hidden="true"
        className="invisible pointer-events-none absolute inset-x-6 top-6 flex flex-col gap-[0.6em] leading-snug"
      >
        <span className="font-medium wrap-break-word">{item.question}</span>
        <span className="whitespace-pre-wrap wrap-break-word">
          {item.answer}
        </span>
      </span>

      {/* Область чтения: тап раскрывает/скрывает ответ. Цвет не меняем. */}
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        title={open ? "Скрыть ответ" : "Показать ответ"}
        className="flex flex-1 cursor-pointer flex-col overflow-hidden text-center"
      >
        <span
          className="m-auto flex w-full flex-col gap-[0.6em] leading-snug"
          style={{ fontSize: `${font}px` }}
        >
          <span className="font-medium wrap-break-word text-foreground">
            {item.question}
          </span>

          {/* Подсказка и ответ живут в двух grid-контейнерах, которые едут
              навстречу: подсказка схлопывается (1fr → 0fr), ответ растёт
              (0fr → 1fr). Переход по grid-template-rows анимирует именно
              высоту, поэтому текст растёт плавно и без скачка — а заодно
              за счёт этого не «прыгает» и центрирование блока. */}
          <span
            className="grid transition-[grid-template-rows] duration-500 ease-out"
            style={{ gridTemplateRows: open ? "0fr" : "1fr" }}
            aria-hidden={open}
          >
            <span className="min-h-0 overflow-hidden">
              <span
                className="block text-muted-foreground"
                style={{ fontSize: "0.75em" }}
              >
                Нажмите, чтобы увидеть ответ
              </span>
            </span>
          </span>

          <span
            className="grid transition-[grid-template-rows] duration-500 ease-out"
            style={{ gridTemplateRows: open ? "1fr" : "0fr" }}
            aria-hidden={!open}
          >
            <span className="min-h-0 overflow-hidden whitespace-pre-wrap wrap-break-word text-muted-foreground">
              {item.answer}
            </span>
          </span>
        </span>
      </button>

      {/* Реакции: одна за показ, счётчики — в индикаторе на кнопке. */}
      <div
        ref={rowRef}
        className="mt-3 flex flex-wrap items-center justify-center gap-20"
      >
        <Button
          variant="outline"
          size="icon-lg"
          className={cn(
            "relative size-14 rounded-full",
            // Выбранная — с цветной заливкой и без общего «погашения»
            // неактивной кнопки (иначе выбор не видно).
            voted === "know" &&
              "border-emerald-500/60 bg-emerald-500/15 disabled:opacity-100 dark:border-emerald-500/60 dark:bg-emerald-500/20",
          )}
          onClick={() => react("know")}
          disabled={Boolean(voted)}
          title="Знаю"
          aria-label={`Знаю (${counts.know})`}
          aria-pressed={voted === "know"}
        >
          <ThumbsUp className="size-7 text-emerald-600 dark:text-emerald-400" />
          <span className="absolute -top-1 -right-1 flex h-5 min-w-5 items-center justify-center rounded-full bg-foreground px-1 text-[10px] font-semibold tabular-nums text-background">
            {counts.know}
          </span>
        </Button>
        <Button
          variant="outline"
          size="icon-lg"
          className={cn(
            "relative size-14 rounded-full",
            voted === "unknown" &&
              "border-destructive/60 bg-destructive/15 disabled:opacity-100 dark:border-destructive/60 dark:bg-destructive/20",
          )}
          onClick={() => react("unknown")}
          disabled={Boolean(voted)}
          title="Не знаю"
          aria-label={`Не знаю (${counts.unknown})`}
          aria-pressed={voted === "unknown"}
        >
          <ThumbsDown className="size-7 text-destructive" />
          <span className="absolute -top-1 -right-1 flex h-5 min-w-5 items-center justify-center rounded-full bg-foreground px-1 text-[10px] font-semibold tabular-nums text-background">
            {counts.unknown}
          </span>
        </Button>
      </div>
    </div>
  );
}

// Раздел «Лента» (просмотр) — мобильный экран, который открывается свайпом
// влево (обратно — свайпом вправо). В боковом меню его нет: раздел не привязан
// к разделам с сервера (sections) и живёт только на клиенте. Наполняется он
// в разделе меню «Лента» (FeedEdit.jsx).
// Показываем ровно один элемент: свайп вверх — следующий, вниз — предыдущий.
function Feed() {
  const feedQuery = useFeedItems(true);
  const items = feedQuery.data || [];
  const count = items.length;

  const [index, setIndex] = useState(0);
  // Индекс держим в границах: лента могла измениться (элемент удалили).
  const current = count > 0 ? Math.min(index, count - 1) : 0;

  useSwipeNav({
    enabled: count > 1,
    onUp: () => setIndex(Math.min(current + 1, count - 1)),
    onDown: () => setIndex(Math.max(current - 1, 0)),
  });

  if (feedQuery.isLoading) {
    return (
      <p className="flex min-h-dvh items-center justify-center gap-1.5 p-6 text-center text-sm text-muted-foreground">
        <Loader2 className="size-4 animate-spin" />
        Загрузка ленты…
      </p>
    );
  }

  if (feedQuery.isError) {
    return (
      <p
        className="flex min-h-dvh items-center justify-center gap-1.5 p-6 text-center text-sm text-destructive"
        role="alert"
      >
        <AlertCircle className="size-4 shrink-0" />
        Не удалось загрузить ленту: {feedQuery.error?.message}
      </p>
    );
  }

  if (count === 0) {
    return (
      <div className="flex min-h-dvh items-center justify-center p-6">
        <Card size="sm" className="w-full">
          <CardContent>
            <p className="text-sm text-muted-foreground">
              В ленте пока пусто. Наполните её в разделе «Лента» меню, свайп
              вправо — вернуться назад.
            </p>
          </CardContent>
        </Card>
      </div>
    );
  }

  return <FeedItem key={items[current].id} item={items[current]} />;
}

export default Feed;
