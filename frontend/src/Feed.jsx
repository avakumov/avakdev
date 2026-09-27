import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { useFeedItems, markFeedItemShown, reactToFeedItem } from "./api.js";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import MarkdownView from "./MarkdownView.jsx";
import {
  Loader2,
  AlertCircle,
  Check,
  X,
  ChevronLeft,
  ChevronRight,
  LogOut,
} from "lucide-react";
import { cn } from "@/lib/utils";

// Пределы адаптивного кегля (px) и запас (px), который вычитаем из высоты
// окна помимо полей карточки и строки кнопок: люфт на расхождения dvh/innerHeight
// и отступ между текстом и кнопками.
const MIN_FONT = 11;
const MAX_FONT = 32;
const FONT_MARGIN = 48 + 12 + 8;

// Оформление Markdown внутри карточки: без собственной подложки и отступов
// (MarkdownView bare), кегль и цвет наследуются от карточки, переносы строк из
// исходного текста сохраняем. Вопрос, ответ и объяснение — Markdown,
// потому что в них бывают примеры кода.
// Код показываем мельче основного текста: блоки — 0.75em, инлайн — 0.8em
// (внутри блока множитель компонента сбрасываем, иначе он уменьшается дважды).
const MD_FEED =
  "leading-snug whitespace-pre-wrap [&_pre]:text-[0.75em] [&_pre_code]:text-[1em] [&_code]:text-[0.8em]";

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
function FeedItem({ item, onExit, onPrev, onNext, canPrev, canNext }) {
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

  // Объяснение и примеры: необязательное поле, показывается по кнопке
  // «подробнее…» под ответом.
  const [showDetails, setShowDetails] = useState(false);
  const hasDetails = Boolean(item.details && item.details.trim());
  const detailsOpen = showDetails && open;
  // Мельче подобранного кегля: объяснение — второстепенный текст.
  const detailsFont = Math.max(10, Math.round(font * 0.7));

  // Тап по карточке раскрывает ответ. Клики по кнопкам (реакции, «подробнее»)
  // не считаем — они делают своё дело.
  const onCardClick = (e) => {
    if (e.target.closest("button")) return;
    setOpen((v) => !v);
  };

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
    <div
      className="relative flex min-h-dvh w-full cursor-pointer flex-col bg-card p-6 text-center"
      onClick={onCardClick}
      title={open ? "Скрыть ответ" : "Показать ответ"}
    >
      {/* Невидимый дубль для замера: повторяет раскрытое состояние (раздел,
          вопрос, ответ) — поэтому кегль считается точно. */}
      <div
        ref={measureRef}
        aria-hidden="true"
        className="invisible pointer-events-none absolute inset-x-6 top-6 flex flex-col gap-[0.6em] leading-snug"
      >
        {item.topic && (
          <div style={{ fontSize: "0.75em" }}>{item.topic}</div>
        )}
        <MarkdownView bare className={MD_FEED}>
          {item.question}
        </MarkdownView>
        <MarkdownView bare className={MD_FEED}>
          {item.answer}
        </MarkdownView>
        {/* Кнопка «подробнее…» и схлопнутое объяснение тоже дают высоту в
            раскрытом состоянии — учитываем её, иначе текст не влезет в экран. */}
        {hasDetails && <div className="h-9" />}
        {hasDetails && <div className="h-0" />}
      </div>

      {/* Содержимое: раздел, вопрос, ответ. Ответ растёт по высоте при касании. */}
      <div
        className="m-auto flex w-full flex-col gap-[0.6em] leading-snug"
        style={{ fontSize: `${font}px` }}
      >
        {/* Раздел (область) — чтобы по короткому вопросу было понятно,
            откуда он. Мелким кеглем и приглушённо. */}
        {item.topic && (
          <div
            className="text-muted-foreground"
            style={{ fontSize: "0.75em" }}
          >
            {item.topic}
          </div>
        )}

        <MarkdownView bare className={cn(MD_FEED, "font-medium text-foreground")}>
          {item.question}
        </MarkdownView>

        <div
          className="grid transition-[grid-template-rows] duration-500 ease-out"
          style={{ gridTemplateRows: open ? "1fr" : "0fr" }}
          aria-hidden={!open}
        >
          <div className="min-h-0 overflow-hidden">
            <MarkdownView bare className={cn(MD_FEED, "text-muted-foreground")}>
              {item.answer}
            </MarkdownView>
          </div>
        </div>

        {hasDetails && (
          <>
            {/* Кнопка — сразу под ответом. Пока ответ скрыт, не показываем,
                но место держим: геометрия не меняется при раскрытии. */}
            <Button
              variant="outline"
              size="lg"
              className={cn("self-center", !open && "invisible")}
              onClick={() => setShowDetails((v) => !v)}
            >
              {detailsOpen ? "свернуть" : "подробнее…"}
            </Button>

            {/* Объяснение и примеры — тем же раскрытием по высоте. */}
            <div
              className="grid transition-[grid-template-rows] duration-500 ease-out"
              style={{ gridTemplateRows: detailsOpen ? "1fr" : "0fr" }}
              aria-hidden={!detailsOpen}
              // Тап внутри объяснения не должен схлопывать карточку.
              onClick={(e) => e.stopPropagation()}
            >
              <div
                className="min-h-0 overflow-hidden text-left"
                style={{ fontSize: `${detailsFont}px` }}
              >
                <MarkdownView bare className={MD_FEED}>
                  {item.details}
                </MarkdownView>
              </div>
            </div>
          </>
        )}
      </div>

      {/* Низ карточки: реакции и навигация по ленте. */}
      <div ref={rowRef} className="mt-3 flex flex-col items-center gap-3">
        <div className="flex flex-wrap items-center justify-center gap-20">
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

        {/* Навигация: выход из ленты и переход к предыдущему/следующему. */}
        <div className="flex w-full items-center justify-between gap-2">
          <Button
            variant="ghost"
            onClick={onExit}
            title="Выйти из ленты"
            aria-label="Выйти из ленты"
          >
            <LogOut />
            Выход
          </Button>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="icon"
              className="size-12 rounded-full"
              onClick={onPrev}
              disabled={!canPrev}
              title="Предыдущая"
              aria-label="Предыдущая"
            >
              <ChevronLeft className="size-6" />
            </Button>
            <Button
              variant="outline"
              size="icon"
              className="size-12 rounded-full"
              onClick={onNext}
              disabled={!canNext}
              title="Следующая"
              aria-label="Следующая"
            >
              <ChevronRight className="size-6" />
            </Button>
          </div>
        </div>
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

// Раздел «Лента» (просмотр) — отдельный экран, который открывается кнопкой
// «Запуск» в разделе меню «Лента» (FeedEdit.jsx) и закрывается кнопкой «Выход».
// Показываем ровно один элемент в случайном порядке; переход к следующему и
// предыдущему — кнопками со стрелками.

function Feed({ onExit }) {
  const feedQuery = useFeedItems(true);

  // Порядок перемешивается один раз на заход в ленту (и при обновлении
  // данных) и дальше стабилен, чтобы стрелки ходили по одному и тому же списку.
  const items = useMemo(
    () => (feedQuery.data ? shuffled(feedQuery.data) : []),
    [feedQuery.data],
  );
  const count = items.length;

  const [index, setIndex] = useState(0);
  // Индекс держим в границах: лента могла измениться (элемент удалили).
  const current = count > 0 ? Math.min(index, count - 1) : 0;

  const move = (step) =>
    setIndex(Math.max(0, Math.min(current + step, count - 1)));

  // Пока открыта лента, глушим штатное «потянуть вниз для обновления»
  // (pull-to-refresh в Chrome на Android), чтобы случайный жест не перезагружал
  // страницу. Ставим на корневой скролл — именно он отвечает за этот жест.
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
      <div className="flex min-h-dvh flex-col items-center justify-center gap-3 p-6 text-center">
        <p className="flex items-center gap-1.5 text-sm text-muted-foreground">
          <Loader2 className="size-4 animate-spin" />
          Загрузка ленты…
        </p>
        <Button variant="outline" size="sm" onClick={onExit}>
          <LogOut />
          Выход
        </Button>
      </div>
    );
  }

  if (feedQuery.isError) {
    return (
      <div className="flex min-h-dvh flex-col items-center justify-center gap-3 p-6 text-center">
        <p
          className="flex items-center gap-1.5 text-sm text-destructive"
          role="alert"
        >
          <AlertCircle className="size-4 shrink-0" />
          Не удалось загрузить ленту: {feedQuery.error?.message}
        </p>
        <Button variant="outline" size="sm" onClick={onExit}>
          <LogOut />
          Выход
        </Button>
      </div>
    );
  }

  if (count === 0) {
    return (
      <div className="flex min-h-dvh items-center justify-center p-6">
        <Card size="sm" className="w-full">
          <CardContent className="flex flex-col items-center gap-3 text-center">
            <p className="text-sm text-muted-foreground">
              В ленте пока пусто. Наполните её в разделе «Лента» меню.
            </p>
            <Button variant="outline" size="sm" onClick={onExit}>
              <LogOut />
              Выход
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <FeedItem
      key={items[current].id}
      item={items[current]}
      onExit={onExit}
      onPrev={() => move(-1)}
      onNext={() => move(1)}
      canPrev={current > 0}
      canNext={current < count - 1}
    />
  );
}

export default Feed;
