import { useEffect, useRef, useState } from "react";

// Подписка на CSS media query — нужна, чтобы свайп работал только на мобильном
// макете (на десктопе меню и так всё время на виду).
export function useMediaQuery(query) {
  const [matches, setMatches] = useState(() =>
    typeof window === "undefined" ? false : window.matchMedia(query).matches,
  );

  useEffect(() => {
    const mq = window.matchMedia(query);
    const onChange = () => setMatches(mq.matches);
    onChange();
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, [query]);

  return matches;
}

// Жесты, которые перехватывать нельзя: поля ввода (горизонтальный жест —
// это выделение текста) и ползунки (перетаскивание бегунка).
const IGNORE_SELECTOR =
  "input, select, textarea, [contenteditable='true'], [role='slider'], [data-slot='slider']";

// Минимальная длина свайпа в px и во сколько раз движение по ведущей оси
// должно превышать движение по второй, чтобы жест считался одноосным.
const MIN_SWIPE = 60;
const RATIO = 2;
// Если страница прокручивается, вертикальный жест может быть обычной
// прокруткой: на него реагируем только быстрым «фликом».
const FLICK_MS = 250;

// Прокручивается ли страница целиком. Небольшой допуск — браузеры иногда
// дают расхождение в пару пиксел из-за дробных размеров (dvh и т.п.).
function pageScrollable() {
  const el = document.scrollingElement || document.documentElement;
  return el.scrollHeight > el.clientHeight + 8;
}

// Жест начался внутри всплывающего слоя? Модалки, боковое меню, док заметок и
// редактор книги в проекте — position: fixed, поэтому достаточно проверить
// позиционирование. Плюс явная ручка [data-swipe-ignore] для блоков с
// собственной горизонтальной прокруткой (таблица метрик).
function startsInOverlay(target) {
  if (target instanceof Element && target.closest(IGNORE_SELECTOR)) return true;
  for (let node = target; node && node !== document.body; node = node.parentElement) {
    if (node.hasAttribute?.("data-swipe-ignore")) return true;
    if (getComputedStyle(node).position === "fixed") return true;
  }
  return false;
}

// Свайп по горизонтали и вертикали:
//   вправо — onPrev, влево — onNext;
//   вверх — onUp (следующее), вниз — onDown (предыдущее).
// Обработчики вешаются на document: раскладка страницы может появляться позже
// (загрузка сессии, гейт «важного» сообщения), а слушатели должны пережить это.
export function useSwipeNav({ enabled = true, onPrev, onNext, onUp, onDown } = {}) {
  // Колбэки держим в ref, чтобы не переподписываться на каждый рендер.
  const cbs = useRef({ onPrev, onNext, onUp, onDown });
  cbs.current = { onPrev, onNext, onUp, onDown };

  useEffect(() => {
    if (!enabled) return;

    let start = null; // { x, y, t, ignore } текущего жеста
    let done = false; // жест уже сработал — ждём конца касания

    const reset = () => {
      start = null;
      done = false;
    };

    const onTouchStart = (e) => {
      if (e.touches.length !== 1) return reset();
      const t = e.touches[0];
      start = {
        x: t.clientX,
        y: t.clientY,
        t: Date.now(),
        ignore: startsInOverlay(e.target),
      };
      done = false;
    };

    const onTouchMove = (e) => {
      if (!start || done || e.touches.length !== 1) return;
      const t = e.touches[0];
      const dx = t.clientX - start.x;
      const dy = t.clientY - start.y;
      const adx = Math.abs(dx);
      const ady = Math.abs(dy);

      // Горизонтальный жест.
      if (adx >= MIN_SWIPE && adx >= ady * RATIO) {
        done = true;
        if (start.ignore) return;
        if (dx > 0) cbs.current.onPrev?.();
        else cbs.current.onNext?.();
        return;
      }

      // Вертикальный жест: вверх — следующее, вниз — предыдущее.
      if (ady < MIN_SWIPE || ady < adx * RATIO) return;
      // Страница прокручивается — значит жест, скорее всего, прокрутка:
      // сработаем только на быстрый флик.
      if (pageScrollable() && Date.now() - start.t > FLICK_MS) return;
      done = true;
      if (start.ignore) return;
      if (dy < 0) cbs.current.onUp?.();
      else cbs.current.onDown?.();
    };

    document.addEventListener("touchstart", onTouchStart, { passive: true });
    document.addEventListener("touchmove", onTouchMove, { passive: true });
    document.addEventListener("touchend", reset, { passive: true });
    document.addEventListener("touchcancel", reset, { passive: true });
    return () => {
      document.removeEventListener("touchstart", onTouchStart);
      document.removeEventListener("touchmove", onTouchMove);
      document.removeEventListener("touchend", reset);
      document.removeEventListener("touchcancel", reset);
    };
  }, [enabled]);
}
