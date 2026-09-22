import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { useFeedItems, markFeedItemShown, reactToFeedItem } from "./api.js";
import { useSwipeNav } from "./lib/useSwipeNav.js";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Loader2, AlertCircle, Check, X } from "lucide-react";
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
      {/* Невидимый дубль для замера: повторяет раскрытое состояние (раздел,
          вопрос, ответ) — поэтому кегль считается точно. */}
      <span
        ref={measureRef}
        aria-hidden="true"
        className="invisible pointer-events-none absolute inset-x-6 top-6 flex flex-col gap-[0.6em] leading-snug"
      >
        {item.topic && (
          <span style={{ fontSize: "0.75em" }}>{item.topic}</span>
        )}
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
          {/* Раздел (область) — чтобы по короткому вопросу было понятно,
              откуда он. Мелким кеглем и приглушённо. */}
          {item.topic && (
            <span
              className="text-muted-foreground"
              style={{ fontSize: "0.75em" }}
            >
              {item.topic}
            </span>
          )}

          <span className="font-medium wrap-break-word text-foreground">
            {item.question}
          </span>

          {/* Подсказка не нужна: пока ответ скрыт, под вопросом ничего нет,
              ответ растёт по высоте при касании (0fr → 1fr). */}
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
          {/* Галочка — «знаю»: не «нравится», а «знаю ответ». */}
          <Check className="size-7 text-emerald-600 dark:text-emerald-400" />
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
          {/* Крестик — «не знаю». */}
          <X className="size-7 text-destructive" />
          <span className="absolute -top-1 -right-1 flex h-5 min-w-5 items-center justify-center rounded-full bg-foreground px-1 text-[10px] font-semibold tabular-nums text-background">
            {counts.unknown}
          </span>
        </Button>
      </div>
    </div>
  );
}

// Случайный порядок (Фишер–Йетс). Копию перемешиваем — исходные данные
// react-query не трогаем: в редакторе ленты тот же список нужен по порядку.
function shuffled(list) {
  const out = [...list];
  for (let i = out.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [out[i], out[j]] = [out[j], out[i]];
  }
  return out;
}

// Раздел «Лента» (просмотр) — мобильный экран, который открывается свайпом
// влево (обратно — свайпом вправо). В боковом меню его нет: раздел не привязан
// к разделам с сервера (sections) и живёт только на клиенте. Наполняется он
// в разделе меню «Лента» (FeedEdit.jsx).
// Показываем ровно один элемент в случайном порядке: свайп вверх — следующий,
// вниз — предыдущий. Карточка едет за пальцем, при отпускании — либо возврат,
// либо переход: текущая уезжает в сторону свайпа, следующая приезжает с другой.

// Сдвиг карточки идёт медленнее пальца (как в пейджере), переход — 200 мс.
const DRAG_DAMP = 0.55;
const DRAG_DAMP_EDGE = 0.15; // на краях списка — почти не двигаем
const SWIPE_COMMIT = 60; // порог сдвига карточки для перехода, px
const SLIDE_MS = 200;

function Feed() {
  const feedQuery = useFeedItems(true);

  // Порядок перемешивается один раз на заход в ленту (и при обновлении
  // данных) и дальше стабилен, чтобы свайпы ходили по одному и тому же списку.
  const items = useMemo(
    () => (feedQuery.data ? shuffled(feedQuery.data) : []),
    [feedQuery.data],
  );
  const count = items.length;

  const [index, setIndex] = useState(0);
  // Индекс держим в границах: лента могла измениться (элемент удалили).
  const current = count > 0 ? Math.min(index, count - 1) : 0;

  // Фаза жеста: idle → drag → out → in → idle.
  const phase = useRef("idle");
  const [dragY, setDragY] = useState(0); // текущий сдвиг карточки, px
  const [animated, setAnimated] = useState(false); // включать ли transition

  const move = (step) =>
    setIndex(Math.max(0, Math.min(current + step, count - 1)));

  // Карточка едет за пальцем (с демпфированием).
  const handleDrag = ({ dy }) => {
    if (phase.current === "out" || phase.current === "in") return;
    phase.current = "drag";
    const forward = dy < 0; // вверх — следующий
    const canMove = forward ? current < count - 1 : current > 0;
    setAnimated(false);
    setDragY(dy * (canMove ? DRAG_DAMP : DRAG_DAMP_EDGE));
  };

  // Отпустили: либо возвращаем на место, либо уводим карточку в сторону свайпа.
  const handleDragEnd = ({ dy, flick }) => {
    if (phase.current !== "drag") {
      // Быстрый флик на прокручиваемой странице — переходим сразу (§ без анимации входа).
      if (flick && Math.abs(dy) >= SWIPE_COMMIT) {
        move(dy < 0 ? 1 : -1);
        setAnimated(false);
        setDragY(0);
      }
      return;
    }
    const forward = dy < 0;
    const canMove = forward ? current < count - 1 : current > 0;
    // Порог считаем от хода пальца, а не от dragY: не зависим от того,
    // успел ли React перерисоваться к моменту отпускания.
    const moved = Math.abs(dy) * (canMove ? DRAG_DAMP : DRAG_DAMP_EDGE);
    if (!canMove || moved < SWIPE_COMMIT) {
      // Не дотянули — возвращаем на место.
      phase.current = "idle";
      setAnimated(true);
      setDragY(0);
      return;
    }
    // Уводим текущую карточку за край в сторону свайпа.
    phase.current = "out";
    setAnimated(true);
    setDragY(forward ? -window.innerHeight : window.innerHeight);
  };

  // Уехавшая карточка доехала до края: меняем элемент и вводим новую
  // с противоположной стороны. Реагируем только на свой transition —
  // события детей (анимация ответа, кнопки) сюда тоже всплывают.
  const handleSlideOut = (e) => {
    if (e && e.target !== e.currentTarget) return;
    if (phase.current !== "out") return;
    const forward = dragY < 0;
    phase.current = "in";
    setAnimated(false);
    move(forward ? 1 : -1);
    setDragY((forward ? 1 : -1) * window.innerHeight * 0.35);
    // Два кадра на отрисовку стартовой позиции — потом плавно на место.
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        phase.current = "idle";
        setAnimated(true);
        setDragY(0);
      });
    });
  };

  useSwipeNav({
    enabled: count > 1,
    onDrag: handleDrag,
    onDragEnd: handleDragEnd,
  });

  // Пока открыта лента, глушим штатное «потянуть вниз для обновления»
  // (pull-to-refresh в Chrome на Android). Иначе жест вниз, который у нас
  // листает к предыдущему элементу, вместо этого перезагружает страницу.
  // Ставим на корневой скролл — именно он отвечает за этот жест.
  useEffect(() => {
    const html = document.documentElement;
    const body = document.body;
    const prevHtml = html.style.overscrollBehaviorY;
    const prevBody = body.style.overscrollBehaviorY;
    html.style.overscrollBehaviorY = "contain";
    body.style.overscrollBehaviorY = "contain";
    return () => {
      html.style.overscrollBehaviorY = prevHtml;
      body.style.overscrollBehaviorY = prevBody;
    };
  }, []);

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

  return (
    <div
      className="min-h-dvh w-full"
      style={{
        transform: `translateY(${dragY}px)`,
        transition: animated ? `transform ${SLIDE_MS}ms ease-out` : "none",
      }}
      onTransitionEnd={handleSlideOut}
    >
      <FeedItem key={items[current].id} item={items[current]} />
    </div>
  );
}

export default Feed;
