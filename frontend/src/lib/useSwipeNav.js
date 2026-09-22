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
// С какого смещения начинаем отдавать вертикальный жест наружу (карточка
// едет за пальцем). Чуть больше браузерного допуска для тапа (~10 px): тогда
// любой жест, сдвинувший карточку, уже не считается кликом по ней.
const DRAG_MIN = 12;
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
//   вертикаль отдаётся наружу как поток: onDrag({dy}) на каждом движении
//   (если страница не прокручивается — карточка едет за пальцем) и
//   onDragEnd({dy, flick}) на отпускании. Если страница прокручивается,
//   карточка за пальцем не едет: срабатывает только быстрый флик.
// Обработчики вешаются на document: раскладка страницы может появляться позже
// (загрузка сессии, гейт «важного» сообщения), а слушатели должны пережить это.
export function useSwipeNav({ enabled = true, onPrev, onNext, onDrag, onDragEnd } = {}) {
  // Колбэки держим в ref, чтобы не переподписываться на каждый рендер.
  const cbs = useRef({ onPrev, onNext, onDrag, onDragEnd });
  cbs.current = { onPrev, onNext, onDrag, onDragEnd };

  useEffect(() => {
    if (!enabled) return;

    // Текущий жест: { x, y, t, ignore, scrollable, dy, dragging, done }
    let g = null;

    const reset = () => {
      g = null;
    };

    const onTouchStart = (e) => {
      if (e.touches.length !== 1) return reset();
      const t = e.touches[0];
      g = {
        x: t.clientX,
        y: t.clientY,
        t: Date.now(),
        ignore: startsInOverlay(e.target),
        // Прокручиваемость фиксируем на начало жеста — по ней решаем, отдавать
        // ли карточку за пальцем или не мешать прокрутке.
        scrollable: pageScrollable(),
        dy: 0,
        dragging: false,
        done: false,
      };
    };

    const onTouchMove = (e) => {
      if (!g || g.done || e.touches.length !== 1) return;
      const t = e.touches[0];
      const dx = t.clientX - g.x;
      const dy = t.clientY - g.y;
      const adx = Math.abs(dx);
      const ady = Math.abs(dy);
      g.dy = dy;

      // Горизонтальный жест.
      if (adx >= MIN_SWIPE && adx >= ady * RATIO) {
        g.done = true;
        if (g.ignore) return;
        if (dx > 0) cbs.current.onPrev?.();
        else cbs.current.onNext?.();
        return;
      }

      // Вертикальный жест: только когда он явно вертикальнее горизонтали.
      if (ady < DRAG_MIN || ady < adx) return;
      if (g.ignore) return;

      if (g.scrollable) {
        // Страница прокручивается — жест, скорее всего, прокрутка: сработаем
        // только на быстрый флик.
        if (ady < MIN_SWIPE || ady < adx * RATIO) return;
        if (Date.now() - g.t > FLICK_MS) return;
        g.done = true;
        cbs.current.onDragEnd?.({ dy, flick: true });
        return;
      }

      g.dragging = true;
      cbs.current.onDrag?.({ dy });
    };

    const onTouchEnd = () => {
      const gesture = g;
      g = null;
      if (!gesture || gesture.done || !gesture.dragging) return;
      cbs.current.onDragEnd?.({ dy: gesture.dy, flick: false });
    };

    document.addEventListener("touchstart", onTouchStart, { passive: true });
    document.addEventListener("touchmove", onTouchMove, { passive: true });
    document.addEventListener("touchend", onTouchEnd, { passive: true });
    document.addEventListener("touchcancel", onTouchEnd, { passive: true });
    return () => {
      document.removeEventListener("touchstart", onTouchStart);
      document.removeEventListener("touchmove", onTouchMove);
      document.removeEventListener("touchend", onTouchEnd);
      document.removeEventListener("touchcancel", onTouchEnd);
    };
  }, [enabled]);
}
