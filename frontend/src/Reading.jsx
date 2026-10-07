import { useEffect, useRef, useState } from "react";
import {
  useBooks,
  fetchBook,
  uploadBook,
  deleteBook,
  fetchBookmarks,
  addBookmark,
  deleteBookmark,
  fetchHighlights,
  addHighlight,
  deleteHighlight,
  fetchReadingTime,
  addReadingTime,
  setBookFinished,
} from "./api.js";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { useAppStore } from "./store.js";
import { cn } from "@/lib/utils";
import { IS_LOCAL_DEV } from "@/lib/env";
import { todayStr, formatClock } from "@/lib/formatDate.js";
import DateDisplay from "@/components/DateDisplay.jsx";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardDescription,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
  BookOpenText,
  Upload,
  Loader2,
  AlertCircle,
  Trash2,
  X,
  Minus,
  Plus,
  Sun,
  Moon,
  Bookmark,
  BookmarkPlus,
  BookCheck,
} from "lucide-react";

// Допустимый размер шрифта книги (px) и шаг изменения.
const FONT_MIN = 12;
const FONT_MAX = 32;
const FONT_STEP = 1;
const FONT_DEFAULT = 16;

// Счётчик чтения: без прокрутки дольше этого времени отсчёт встаёт на паузу.
const READING_IDLE_MS = 5 * 60 * 1000;
// Автосохранение времени чтения: раз в 5 минут сбрасываем накопленное на
// сервер. Если деплой/перезапуск сервера совпадёт с закрытием книги, потеряется
// максимум этот интервал, а не вся сессия.
const READING_AUTOSAVE_MS = 5 * 60 * 1000;
// Повторные попытки отправки, если сервер не ответил (например, перезапускался):
// сколько раз и с каким шагом (10 с, 20 с, …).
const READING_SEND_RETRIES = 3;
const READING_SEND_RETRY_MS = 10 * 1000;
// Запас цели дня на случай, если сервер её не отдал (по умолчанию — 1 час).
const READING_GOAL_FALLBACK = 3600;

// Сколько календарных дней заняло чтение книги (включительно): от первой
// закладки (started_at) до отметки «прочитана» (finished_at). null — книга
// не прочитана или начало чтения неизвестно.
function readingDays(startedAt, finishedAt) {
  if (!startedAt || !finishedAt) return null;
  const start = new Date(startedAt);
  const end = new Date(finishedAt);
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) return null;
  const a = Date.UTC(start.getFullYear(), start.getMonth(), start.getDate());
  const b = Date.UTC(end.getFullYear(), end.getMonth(), end.getDate());
  const days = Math.round((b - a) / 86400000) + 1;
  return days > 1 ? days : 1;
}

// Русская форма слова «день»: 1 день, 2 дня, 5 дней.
function dayWord(n) {
  const d10 = n % 10;
  const d100 = n % 100;
  if (d10 === 1 && d100 !== 11) return "день";
  if (d10 >= 2 && d10 <= 4 && (d100 < 12 || d100 > 14)) return "дня";
  return "дней";
}

// Стили текста книги (HTML приходит с сервера уже очищенным).
// Размер шрифта задаётся извне (кнопками «−/+»).
const BOOK_TEXT_CLASS = [
  "leading-relaxed",
  // Выключка по формату: строки оканчиваются на одной вертикали.
  // Последняя строка абзаца остаётся слева, переносы — браузерные.
  "text-justify [text-align-last:start]",
  "[hyphens:auto] [-webkit-hyphens:auto] [overflow-wrap:break-word]",
  // Первая строка абзаца — отступ в три символа.
  "[&_p]:mb-3 [&_p]:[text-indent:3ch]",
  "[&_h1]:mt-6 [&_h1]:mb-2 [&_h1]:text-xl [&_h1]:font-semibold",
  "[&_h2]:mt-6 [&_h2]:mb-2 [&_h2]:text-lg [&_h2]:font-semibold",
  "[&_h3]:mt-6 [&_h3]:mb-2 [&_h3]:text-lg [&_h3]:font-semibold",
  "[&_h4]:mt-4 [&_h4]:mb-2 [&_h4]:font-medium",
  "[&_img]:mx-auto [&_img]:my-4 [&_img]:max-w-full [&_img]:rounded-none",
  "[&_blockquote]:my-3 [&_blockquote]:border-l-2 [&_blockquote]:border-border [&_blockquote]:pl-3 [&_blockquote]:text-muted-foreground",
  "[&_table]:my-3 [&_table]:w-full",
  "[&_td]:border [&_td]:border-border/60 [&_td]:px-2 [&_td]:py-1 [&_td]:align-top",
  "[&_th]:border [&_th]:border-border/60 [&_th]:px-2 [&_th]:py-1 [&_th]:text-left",
  "[&_em]:italic [&_strong]:font-semibold",
  "[&_.poem]:my-4 [&_.poem]:italic [&_.stanza]:mb-3",
  "[&_.verse]:pl-4 [&_.verse]:[text-indent:0] [&_.verse]:text-left",
  "[&_.text-author]:text-right [&_.text-author]:text-muted-foreground [&_.text-author]:[text-indent:0]",
  "[&_.epigraph]:my-4 [&_.epigraph]:pl-4 [&_.epigraph]:text-muted-foreground",
  "[&_.chapter]:mb-8",
].join(" ");

// ==== Закладки ====
// Место в книге хранится как число символов от начала текста (как считает JS).
// HTML книги неизменен, поэтому то же место всегда находится тем же способом.

// Точка (узел, смещение) → число символов от начала контейнера с текстом.
function offsetOfPoint(root, node, offset) {
  const range = document.createRange();
  range.selectNodeContents(root);
  range.setEnd(node, offset);
  return range.toString().length;
}

// Число символов от начала контейнера → точка (узел, смещение).
function pointAtOffset(root, target) {
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  let acc = 0;
  let last = null;
  for (let node = walker.nextNode(); node; node = walker.nextNode()) {
    const len = node.nodeValue.length;
    if (target <= acc + len) {
      return { node, offset: Math.max(0, target - acc) };
    }
    acc += len;
    last = node;
  }
  return last ? { node: last, offset: last.nodeValue.length } : null;
}

// Подсветка найденного фрагмента: рисуем поверх текста полосы (по одной на
// строку) и убираем их — сам текст книги при этом не меняется.
function flashRange(range, layer) {
  if (!range || !layer) return;
  const layerRect = layer.getBoundingClientRect();
  for (const r of range.getClientRects()) {
    const el = document.createElement("div");
    el.className = "book-flash";
    el.style.left = `${r.left - layerRect.left}px`;
    el.style.top = `${r.top - layerRect.top}px`;
    el.style.width = `${Math.max(2, r.width)}px`;
    el.style.height = `${r.height}px`;
    layer.appendChild(el);
    setTimeout(() => el.remove(), 1400);
  }
}

// ==== Постоянные выделения цветом ====
// Текст книги React не перерисовывает (dangerouslySetInnerHTML), поэтому
// выделения рисуем сами: оборачиваем нужные куски текста в <span class="book-hl">.
// Обёртка не меняет сам текст, поэтому смещения в символах остаются валидными.

// Снять прежние выделения и слить обратно разделённые текстовые узлы.
function clearHighlightSpans(root) {
  for (const el of root.querySelectorAll("span.book-hl")) {
    const parent = el.parentNode;
    while (el.firstChild) parent.insertBefore(el.firstChild, el);
    parent.removeChild(el);
  }
  root.normalize();
}

// Обернуть диапазон [start, end) в <span class="book-hl"> заданного цвета.
function wrapHighlight(root, start, end, id, color) {
  // Сначала (не меняя DOM) собираем текстовые узлы, попадающие в диапазон.
  const targets = [];
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  let acc = 0;
  for (let node = walker.nextNode(); node; node = walker.nextNode()) {
    const nodeStart = acc;
    const nodeEnd = acc + node.nodeValue.length;
    acc = nodeEnd;
    if (nodeEnd <= start || nodeStart >= end) continue;
    targets.push({
      node,
      from: Math.max(start, nodeStart) - nodeStart,
      to: Math.min(end, nodeEnd) - nodeStart,
    });
  }
  // Затем отделяем нужные куски и оборачиваем их.
  for (const { node, from, to } of targets) {
    if (to < node.nodeValue.length) node.splitText(to);
    let mid = node;
    if (from > 0) mid = node.splitText(from);
    const span = document.createElement("span");
    span.className = "book-hl";
    span.dataset.hlId = String(id);
    span.dataset.color = color;
    mid.parentNode.insertBefore(span, mid);
    span.appendChild(mid);
  }
}

// Порог «дошли до конца книги» для вопроса о прочтении (px).
const BOOK_END_THRESHOLD = 40;

// Ширина всплывающего меню выделения (px) — задана явно, чтобы не вылезти за
// границы текста при расчёте позиции.
const SELECTION_MENU_W = 236;

// Палитра подсветки. Пока только визуальная заготовка: цвета не сохраняются.
const HIGHLIGHT_COLORS = [
  { id: "yellow", label: "жёлтый", className: "bg-amber-300" },
  { id: "green", label: "зелёный", className: "bg-emerald-300" },
  { id: "blue", label: "голубой", className: "bg-sky-300" },
  { id: "pink", label: "розовый", className: "bg-pink-300" },
];

// Позиция меню выделения — всегда снизу: от курсора (при протяжке) или от
// прямоугольника выделения. По горизонтали — по центру, с зажимом внутрь
// границ текста.
function selectionMenuStyle(sel, pos) {
  const desired =
    (pos ? pos.left : sel.left + sel.width / 2) - SELECTION_MENU_W / 2;
  const maxLeft = Math.max(8, sel.layerWidth - SELECTION_MENU_W - 8);
  const left = Math.min(Math.max(8, desired), maxLeft);
  const top = pos ? pos.top + 16 : sel.bottom + 6;
  return { left, top, width: SELECTION_MENU_W };
}

// Модалка чтения книги: занимает всё окно, сверху — название, автор,
// кнопки размера шрифта и закрытие, ниже — прокручиваемый текст.
function BookModal({ book, initialJump = null, onClose }) {
  const theme = useAppStore((s) => s.theme);
  const toggleTheme = useAppStore((s) => s.toggleTheme);
  // Поверх книги может быть открыт редактор заметки — тогда Esc закрывает его.
  const draftOpen = useAppStore((s) => Boolean(s.draftEditor));
  const queryClient = useQueryClient();
  const [data, setData] = useState(null);
  const [error, setError] = useState("");
  const [fontSize, setFontSize] = useState(FONT_DEFAULT);
  // Закладки книги и текущее выделение в тексте (позиция, текст и геометрия
  // для всплывающего меню — см. readSelection).
  const [bookmarks, setBookmarks] = useState([]);
  const [sel, setSel] = useState(null);
  // Позиция меню выделения в координатах слоя текста (у курсора при протяжке).
  const [menuPos, setMenuPos] = useState(null);
  // Постоянные выделения книги и меню действий над выбранным выделением.
  const [highlights, setHighlights] = useState([]);
  const [hlMenu, setHlMenu] = useState(null); // { id, left, top }
  const [panelOpen, setPanelOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [pendingJump, setPendingJump] = useState(initialJump); // закладка для перехода
  // Прочитана ли книга и спрашиваем ли об этом (дошли до конца текста).
  const [finished, setFinished] = useState(Boolean(book.finished_at));
  const [askFinished, setAskFinished] = useState(false);
  const [savingFinished, setSavingFinished] = useState(false);
  const endAskedRef = useRef(false);
  const scrollRef = useRef(null);
  const layerRef = useRef(null);
  const textRef = useRef(null);
  // Зажата ли кнопка указателя — пока да, меню следует за курсором.
  const pointerDownRef = useRef(false);
  // Счётчик чтения: секунды этой сессии, сумма за сегодня и цель дня.
  // Сумма за сегодня берётся из базы при открытии книги (см. эффект ниже).
  const [sessionSeconds, setSessionSeconds] = useState(0);
  const [daySeconds, setDaySeconds] = useState(0);
  const [goalSeconds, setGoalSeconds] = useState(READING_GOAL_FALLBACK);
  const [running, setRunning] = useState(false);
  // Не удалось загрузить/уточнить время за сегодня — показываем это, а не нули.
  const [dayUnknown, setDayUnknown] = useState(false);
  const accMsRef = useRef(0); // накопленное время сессии, мс
  const runStartRef = useRef(0); // начало текущего отсчёта (0 — пауза)
  const idleTimerRef = useRef(null);
  const sentSecondsRef = useRef(0); // сколько секунд сессии уже отправлено
  const dayRef = useRef(todayStr());

  // Уведомления показываем всплывающими (Sonner) — они не сдвигают текст книги.
  const showNotice = (text, isError = false) => {
    if (isError) toast.error(text);
    else toast.success(text);
  };

  // Накопленное время сессии, секунды (включая идущий отсчёт).
  const sessionMs = () =>
    accMsRef.current + (runStartRef.current ? Date.now() - runStartRef.current : 0);

  // Фиксируем идущий отсчёт в накопленном и останавливаем его.
  const stopReading = () => {
    if (runStartRef.current) {
      accMsRef.current += Date.now() - runStartRef.current;
      runStartRef.current = 0;
    }
    clearTimeout(idleTimerRef.current);
  };

  // Отправляем ещё не сохранённые секунды чтения за сегодня. Отсчёт при этом
  // НЕ останавливаем: счётчик продолжает расти, а сколько уже отправлено,
  // помним отдельно (sentSecondsRef).
  const flushReadingTime = (keepalive = false, attempt = 0) => {
    const total = Math.floor(sessionMs() / 1000);
    const delta = total - sentSecondsRef.current;
    if (delta <= 0) return;
    sentSecondsRef.current = total;
    sendReadingDelta(delta, total, keepalive, attempt);
  };

  // Одна попытка отправки. При сбое возвращаем секунды в несохранённые и
  // пробуем ещё раз (сервер мог перезапускаться — например, при деплое).
  const sendReadingDelta = (delta, total, keepalive, attempt) => {
    addReadingTime(dayRef.current, delta, keepalive)
      .then((d) => {
        // Сервер отдаёт новую сумму за день — берём её как источник истины:
        // «сегодня» останется верным, даже если первая загрузка не удалась.
        if (typeof d?.seconds === "number") {
          setDaySeconds(Math.max(0, d.seconds - total));
          if (d.goal_seconds) setGoalSeconds(d.goal_seconds);
          setDayUnknown(false);
        }
        // Обновляем сегодняшнюю сумму в кэше — её показывает карточка
        // «Чтение» в «Дне».
        queryClient.invalidateQueries({ queryKey: ["reading-time", dayRef.current] });
      })
      .catch(() => {
        sentSecondsRef.current -= delta;
        // Страница уходит (keepalive) — повторять уже негде и незачем.
        if (keepalive) return;
        if (attempt < READING_SEND_RETRIES) {
          // Повторяем целиком: flush заново посчитает накопленное.
          setTimeout(
            () => flushReadingTime(false, attempt + 1),
            READING_SEND_RETRY_MS * (attempt + 1),
          );
          return;
        }
        showNotice("Не удалось сохранить время чтения", true);
      });
  };

  // Закрытие книги: сначала отправляем накопленное время чтения.
  const handleClose = () => {
    flushReadingTime();
    onClose();
  };

  // Отсчёт встаёт на паузу (нет прокрутки больше READING_IDLE_MS).
  const pauseReading = () => {
    stopReading();
    setSessionSeconds(Math.floor(accMsRef.current / 1000));
    setRunning(false);
  };

  // Прокрутка — признак чтения: начинаем или продолжаем отсчёт.
  const resumeReading = () => {
    if (!runStartRef.current) {
      runStartRef.current = Date.now();
      setRunning(true);
    }
    clearTimeout(idleTimerRef.current);
    idleTimerRef.current = setTimeout(pauseReading, READING_IDLE_MS);
  };

  useEffect(() => {
    let alive = true;
    setData(null);
    setError("");
    fetchBook(book.id)
      .then((d) => {
        if (alive) setData(d);
      })
      .catch((err) => {
        if (alive) setError(err.message || "Не удалось открыть книгу");
      });
    return () => {
      alive = false;
    };
  }, [book.id]);

  // Время чтения за сегодня: берём сохранённое в базе при открытии книги,
  // дальше сверху показываем «сегодня» + текущая сессия.
  useEffect(() => {
    let alive = true;
    fetchReadingTime(dayRef.current)
      .then((d) => {
        if (!alive) return;
        setDaySeconds(d.seconds || 0);
        if (d.goal_seconds) setGoalSeconds(d.goal_seconds);
        setDayUnknown(false);
      })
      .catch(() => {
        // Время не загрузилось: не показываем нули как «сброшенное» время.
        if (alive) setDayUnknown(true);
      });
    return () => {
      alive = false;
    };
  }, []);

  // Автосохранение: раз в 5 минут сбрасываем накопленное на сервер.
  useEffect(() => {
    const id = setInterval(() => flushReadingTime(), READING_AUTOSAVE_MS);
    return () => clearInterval(id);
  }, []);

  // Тик раз в секунду — только для отображения (время считается по Date.now).
  useEffect(() => {
    if (!running) return;
    setSessionSeconds(Math.floor(sessionMs() / 1000));
    const id = setInterval(() => {
      setSessionSeconds(Math.floor(sessionMs() / 1000));
    }, 1000);
    return () => clearInterval(id);
  }, [running]);

  // Прокрутка пользователя запускает/продолжает отсчёт. Намеренно слушаем
  // действия (колесо, тач, клавиши, полоса прокрутки), а не событие scroll:
  // программный переход к закладке не должен запускать таймер.
  useEffect(() => {
    const scroller = scrollRef.current;
    if (!scroller) return;
    const scrollKeys = new Set([
      "ArrowUp",
      "ArrowDown",
      "PageUp",
      "PageDown",
      "Home",
      "End",
      " ",
    ]);
    const onKey = (e) => {
      if (scrollKeys.has(e.key)) resumeReading();
    };
    scroller.addEventListener("wheel", resumeReading, { passive: true });
    scroller.addEventListener("touchmove", resumeReading, { passive: true });
    scroller.addEventListener("mousedown", resumeReading);
    window.addEventListener("keydown", onKey);
    return () => {
      scroller.removeEventListener("wheel", resumeReading);
      scroller.removeEventListener("touchmove", resumeReading);
      scroller.removeEventListener("mousedown", resumeReading);
      window.removeEventListener("keydown", onKey);
    };
  }, []);

  // Уход со страницы (закрытие вкладки, перезагрузка, выход из сессии) —
  // тоже сохраняем накопленное время.
  useEffect(
    () => () => {
      clearTimeout(idleTimerRef.current);
      flushReadingTime(true);
    },
    [],
  );

  // Esc закрывает книгу (если сверху нет редактора заметки).
  useEffect(() => {
    const onKey = (e) => {
      if (e.key !== "Escape" || draftOpen) return;
      handleClose();
    };
    document.addEventListener("keydown", onKey);
    const prevOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = prevOverflow;
    };
  }, [onClose, draftOpen]);

  // Загрузка закладок и выделений при открытии книги.
  useEffect(() => {
    let alive = true;
    setBookmarks([]);
    setHighlights([]);
    setHlMenu(null);
    setSel(null);
    setPanelOpen(false);
    fetchBookmarks(book.id)
      .then((list) => {
        if (alive) setBookmarks(list);
      })
      .catch(() => {
        // Закладки не критичны для чтения — молча оставляем список пустым.
      });
    fetchHighlights(book.id)
      .then((list) => {
        if (alive) setHighlights(list);
      })
      .catch(() => {
        // Выделения тоже не критичны — оставляем список пустым.
      });
    return () => {
      alive = false;
    };
  }, [book.id]);

  // Текущее выделение внутри текста книги (или null).
  const readSelection = () => {
    const root = textRef.current;
    const layer = layerRef.current;
    const s = window.getSelection();
    if (!root || !layer || !s || s.rangeCount === 0 || s.isCollapsed) return null;
    const range = s.getRangeAt(0);
    if (!root.contains(range.commonAncestorContainer)) return null;
    // Позицию считаем в координатах слоя текста (layerRef), поэтому меню
    // «приклеено» к тексту и не уезжает при прокрутке.
    const r = range.getBoundingClientRect();
    const lr = layer.getBoundingClientRect();
    return {
      anchor: offsetOfPoint(root, range.startContainer, range.startOffset),
      end: offsetOfPoint(root, range.endContainer, range.endOffset),
      text: s.toString(),
      left: r.left - lr.left,
      bottom: r.bottom - lr.top,
      width: r.width,
      layerWidth: layer.clientWidth,
    };
  };

  // Следим за выделением: мышь, клавиатура, тач — всё приходит сюда.
  useEffect(() => {
    const track = () => {
      const next = readSelection();
      setSel(next);
      // Началось новое выделение — меню существующей подсветки закрываем.
      if (next) setHlMenu(null);
    };
    document.addEventListener("selectionchange", track);
    return () => document.removeEventListener("selectionchange", track);
  }, []);

  // Смена кегля меняет раскладку текста — прежняя позиция меню станет неверной.
  useEffect(() => {
    setSel(null);
  }, [fontSize]);

  // Держим актуальным состояние «кнопка указателя нажата».
  useEffect(() => {
    const down = () => {
      pointerDownRef.current = true;
    };
    const up = () => {
      pointerDownRef.current = false;
    };
    window.addEventListener("pointerdown", down);
    window.addEventListener("pointerup", up);
    window.addEventListener("pointercancel", up);
    return () => {
      window.removeEventListener("pointerdown", down);
      window.removeEventListener("pointerup", up);
      window.removeEventListener("pointercancel", up);
    };
  }, []);

  const selActive = Boolean(sel);

  // Пока выделение активно, при протяжке держим меню у курсора (в координатах
  // слоя текста). После отпускания кнопки меню остаётся на последней позиции.
  useEffect(() => {
    if (!selActive) {
      setMenuPos(null);
      return;
    }
    const layer = layerRef.current;
    if (!layer) return;
    const onMove = (e) => {
      if (!pointerDownRef.current) return;
      const lr = layer.getBoundingClientRect();
      setMenuPos({ left: e.clientX - lr.left, top: e.clientY - lr.top });
    };
    window.addEventListener("pointermove", onMove);
    return () => window.removeEventListener("pointermove", onMove);
  }, [selActive]);

  // Рисуем постоянные выделения: снимаем прежние <span> и навешиваем заново.
  useEffect(() => {
    const root = textRef.current;
    if (!root) return;
    clearHighlightSpans(root);
    const sorted = [...highlights].sort(
      (a, b) => a.start - b.start || a.end - b.end,
    );
    for (const h of sorted) {
      if (h.end > h.start) wrapHighlight(root, h.start, h.end, h.id, h.color);
    }
  }, [data, highlights]);

  // Закрываем меню выделения при клике вне него и вне самой подсветки.
  useEffect(() => {
    if (!hlMenu) return;
    const onDown = (e) => {
      if (e.target.closest?.(".book-hl") || e.target.closest?.("[data-hl-menu]")) {
        return;
      }
      setHlMenu(null);
    };
    document.addEventListener("pointerdown", onDown);
    return () => document.removeEventListener("pointerdown", onDown);
  }, [hlMenu]);

  // Поставить закладку на выделенном фрагменте.
  const handleAddBookmark = async () => {
    // Выделение берём из состояния, но если браузер успел его сбросить
    // (например, при клике) — читаем ещё раз напрямую.
    const current = sel || readSelection();
    if (!current || saving) return;
    setSaving(true);
    try {
      const created = await addBookmark(book.id, current.anchor, current.text);
      setBookmarks((list) =>
        [...list, created].sort((a, b) => a.anchor - b.anchor),
      );
      setSel(null);
      window.getSelection()?.removeAllRanges();
      // Процент прочтения в карточке книги считается по закладкам.
      queryClient.invalidateQueries({ queryKey: ["books"] });
      showNotice("Закладка добавлена");
    } catch (err) {
      showNotice(err.message || "Не удалось сохранить закладку", true);
    } finally {
      setSaving(false);
    }
  };

  // Переход к закладке. Панель закрываем сразу, а прокрутку и подсветку
  // делаем в следующем кадре — после того как макет пересобрался
  // (при открытой панели геометрия текста другая). Ждём ещё и загрузки
  // текста книги, если переход запрошен в момент открытия.
  useEffect(() => {
    if (!pendingJump || !data) return;
    const root = textRef.current;
    const scroller = scrollRef.current;
    if (!root || !scroller) return;

    const bm = pendingJump;
    setPendingJump(null);
    const start = pointAtOffset(root, bm.anchor);
    if (!start) return;
    const range = document.createRange();
    range.setStart(start.node, start.offset);
    const end = pointAtOffset(root, bm.anchor + Math.max(1, bm.excerpt.length));
    if (end) range.setEnd(end.node, end.offset);

    const rect = range.getBoundingClientRect();
    const srect = scroller.getBoundingClientRect();
    scroller.scrollTop += rect.top - srect.top - srect.height / 3;
    flashRange(range, layerRef.current);
  }, [pendingJump, data]);

  const goToBookmark = (bm) => {
    setPanelOpen(false);
    setPendingJump(bm);
  };

  const handleDeleteBookmark = async (bm) => {
    try {
      await deleteBookmark(book.id, bm.id);
      setBookmarks((list) => list.filter((x) => x.id !== bm.id));
      queryClient.invalidateQueries({ queryKey: ["books"] });
    } catch (err) {
      showNotice(err.message || "Не удалось удалить закладку", true);
    }
  };

  // Поставить выделение цветом на текущем фрагменте.
  const handleAddHighlight = async (color) => {
    const current = sel || readSelection();
    if (!current || saving || current.end <= current.anchor) return;
    setSaving(true);
    try {
      const created = await addHighlight(
        book.id,
        current.anchor,
        current.end,
        color,
        current.text,
      );
      setHighlights((list) =>
        [...list, created].sort((a, b) => a.start - b.start),
      );
      setSel(null);
      window.getSelection()?.removeAllRanges();
      showNotice("Выделение добавлено");
    } catch (err) {
      showNotice(err.message || "Не удалось сохранить выделение", true);
    } finally {
      setSaving(false);
    }
  };

  // Удалить выделение (по клику на подсветку).
  const handleDeleteHighlight = async (id) => {
    setHlMenu(null);
    try {
      await deleteHighlight(book.id, id);
      setHighlights((list) => list.filter((h) => h.id !== id));
    } catch (err) {
      showNotice(err.message || "Не удалось удалить выделение", true);
    }
  };

  // Клик по тексту: по подсветке — открываем меню действий (удалить), иначе
  // закрываем его. Во время выделения текста меню не трогаем.
  const handleTextClick = (e) => {
    const s = window.getSelection();
    if (s && !s.isCollapsed) return;
    const layer = layerRef.current;
    const el = e.target.closest?.(".book-hl");
    if (!el || !layer) {
      setHlMenu(null);
      return;
    }
    const lr = layer.getBoundingClientRect();
    const r = el.getBoundingClientRect();
    setHlMenu({
      id: Number(el.dataset.hlId),
      left: Math.min(r.left - lr.left, Math.max(8, layer.clientWidth - 200)),
      top: r.bottom - lr.top,
    });
  };

  // Дошли до конца книги — спрашиваем, прочитана ли она (один раз за сессию).
  const handleScroll = () => {
    const el = scrollRef.current;
    if (!el || finished || endAskedRef.current) return;
    if (el.scrollTop + el.clientHeight >= el.scrollHeight - BOOK_END_THRESHOLD) {
      endAskedRef.current = true;
      setAskFinished(true);
    }
  };

  // Отметить книгу прочитанной: в списке она уедет в конец, а в «Дне»
  // больше не будет предлагаться для чтения.
  const handleMarkFinished = async () => {
    setSavingFinished(true);
    try {
      await setBookFinished(book.id, true);
      setFinished(true);
      setAskFinished(false);
      queryClient.invalidateQueries({ queryKey: ["books"] });
      queryClient.invalidateQueries({ queryKey: ["last-bookmark"] });
      showNotice("Книга отмечена прочитанной");
    } catch (err) {
      showNotice(err.message || "Не удалось отметить книгу", true);
    } finally {
      setSavingFinished(false);
    }
  };

  // Цель дня достигнута (с учётом уже сохранённого за сегодня). Пока сумма за
  // день неизвестна, ничего не подсвечиваем.
  const goalReached =
    !dayUnknown && daySeconds + sessionSeconds >= goalSeconds;

  return (
    // В dev сверху висит жёлтая полоса (DevBanner, z-60). Начинаем модалку под
    // ней (top-7), иначе полоса перекрывает шапку чтения.
    <div
      className={cn(
        "fixed inset-x-0 z-50 flex flex-col bg-[color-mix(in_oklch,var(--background)_90%,var(--foreground))] text-[color-mix(in_oklch,var(--foreground)_85%,var(--background))]",
        IS_LOCAL_DEV ? "top-7 bottom-0" : "inset-y-0",
      )}
    >
      {/* Шапка чтения */}
      <div className="flex shrink-0 items-center gap-2 border-b px-4 py-3">
        <div className="min-w-0 flex-1">
          <p className="truncate font-semibold">
            {data?.title || book.title}
          </p>
          <p className="truncate text-xs text-muted-foreground">
            {data?.author || book.author || ""}
            {finished && (
              <span className="text-emerald-600 dark:text-emerald-400">
                {" · прочитана"}
              </span>
            )}
          </p>
        </div>

        {/* Время чтения сегодня: чч:мм:сс (сохранённое в базе + текущая
            сессия). Зелёное — цель дня достигнута; приглушённое — отсчёт
            на паузе (нет прокрутки больше 5 минут). */}
        <span
          title={
            dayUnknown
              ? "Время чтения за сегодня не загрузилось"
              : `Сегодня: ${formatClock(daySeconds + sessionSeconds)}` +
                ` · сессия: ${formatClock(sessionSeconds)}` +
                ` · цель дня: ${formatClock(goalSeconds)}`
          }
          className={cn(
            "w-18 shrink-0 text-center text-xs tabular-nums",
            dayUnknown
              ? "text-destructive"
              : goalReached
                ? "text-emerald-600 dark:text-emerald-400"
                : running
                  ? "text-foreground"
                  : "text-muted-foreground",
          )}
        >
          {dayUnknown ? "--:--:--" : formatClock(daySeconds + sessionSeconds)}
        </span>

        {/* Быстрая смена темы */}
        <Button
          variant="outline"
          size="icon"
          onClick={toggleTheme}
          title={theme === "dark" ? "Светлая тема" : "Тёмная тема"}
          aria-label={theme === "dark" ? "Включить светлую тему" : "Включить тёмную тему"}
        >
          {theme === "dark" ? <Sun /> : <Moon />}
        </Button>

        {/* Список закладок */}
        <Button
          variant={panelOpen ? "secondary" : "outline"}
          size="icon"
          onClick={() => setPanelOpen((v) => !v)}
          title={`Закладки (${bookmarks.length})`}
          aria-label="Список закладок"
        >
          <Bookmark />
        </Button>

        {/* Размер шрифта */}
        <Button
          variant="outline"
          size="icon"
          onClick={() => setFontSize((s) => Math.max(FONT_MIN, s - FONT_STEP))}
          disabled={fontSize <= FONT_MIN}
          title="Уменьшить шрифт"
          aria-label="Уменьшить шрифт"
        >
          <Minus />
        </Button>
        <span className="w-8 text-center text-xs tabular-nums text-muted-foreground">
          {fontSize}
        </span>
        <Button
          variant="outline"
          size="icon"
          onClick={() => setFontSize((s) => Math.min(FONT_MAX, s + FONT_STEP))}
          disabled={fontSize >= FONT_MAX}
          title="Увеличить шрифт"
          aria-label="Увеличить шрифт"
        >
          <Plus />
        </Button>

        <Button
          variant="ghost"
          size="icon"
          onClick={handleClose}
          title="Закрыть книгу"
          aria-label="Закрыть книгу"
        >
          <X />
        </Button>
      </div>

      {/* Конец книги: вопрос о прочтении */}
      {askFinished && (
        <div className="flex shrink-0 flex-wrap items-center gap-2 border-b bg-emerald-500/10 px-4 py-2 text-sm">
          <BookCheck className="size-4 text-emerald-600 dark:text-emerald-400" />
          Книга прочитана?
          <Button size="sm" onClick={handleMarkFinished} disabled={savingFinished}>
            {savingFinished ? <Loader2 className="animate-spin" /> : <BookCheck />}
            Да, прочитана
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setAskFinished(false)}
            disabled={savingFinished}
          >
            Позже
          </Button>
        </div>
      )}

      {/* Список закладок книги */}
      {panelOpen && (
        <div className="shrink-0 border-b px-4 py-2">
          <p className="mb-1 text-xs text-muted-foreground">
            Закладок: {bookmarks.length}
          </p>
          {bookmarks.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              Выделите текст в книге и нажмите кнопку с закладкой, чтобы
              сохранить место.
            </p>
          ) : (
            <ul className="space-y-0.5">
              {bookmarks.map((bm) => (
                <li key={bm.id} className="flex items-center gap-2">
                  <button
                    type="button"
                    onClick={() => goToBookmark(bm)}
                    title={bm.excerpt.trim()}
                    className="min-w-0 flex-1 truncate text-left text-sm text-muted-foreground hover:text-foreground"
                  >
                    {bm.excerpt.trim()}
                  </button>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => handleDeleteBookmark(bm)}
                    title="Удалить закладку"
                    aria-label="Удалить закладку"
                  >
                    <Trash2 />
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      {/* Текст книги — во всю ширину окна */}
      <div ref={scrollRef} className="flex-1 overflow-y-auto" onScroll={handleScroll}>
        <div ref={layerRef} className="relative w-full px-6 py-6">
          {error ? (
            <p
              className="flex items-center gap-1.5 text-sm text-destructive"
              role="alert"
            >
              <AlertCircle className="size-4" />
              {error}
            </p>
          ) : !data ? (
            <p className="text-sm text-muted-foreground">
              <Loader2 className="mr-1 inline size-4 animate-spin" />
              Загружаю книгу…
            </p>
          ) : (
            <div
              ref={textRef}
              className={BOOK_TEXT_CLASS}
              style={{ fontSize: `${fontSize}px` }}
              onClick={handleTextClick}
              dangerouslySetInnerHTML={{ __html: data.html }}
            />
          )}

          {/* Меню у выделения: закладка и палитра (цвета — пока заготовка).
              onMouseDown preventDefault — чтобы клик по меню не сбрасывал
              выделение (закладка берёт позицию из него). */}
          {sel && (
            <div
              className="absolute z-10 flex select-none items-center gap-1 border bg-background p-1 text-foreground shadow-md"
              style={selectionMenuStyle(sel, menuPos)}
              onMouseDown={(e) => e.preventDefault()}
            >
              <Button
                variant="ghost"
                size="sm"
                onClick={handleAddBookmark}
                disabled={saving}
                title="Добавить закладку"
              >
                {saving ? <Loader2 className="animate-spin" /> : <BookmarkPlus />}
                Закладка
              </Button>
              <span className="mx-0.5 h-5 w-px bg-border" />
              {HIGHLIGHT_COLORS.map((c) => (
                <button
                  key={c.id}
                  type="button"
                  title={`Выделить: ${c.label}`}
                  aria-label={`Выделить цветом: ${c.label}`}
                  onClick={() => handleAddHighlight(c.id)}
                  disabled={saving}
                  className={cn(
                    "size-5 border border-black/10 disabled:opacity-50",
                    c.className,
                  )}
                />
              ))}
            </div>
          )}

          {/* Меню существующего выделения: удалить (клик по подсветке). */}
          {hlMenu && (
            <div
              data-hl-menu
              className="absolute z-10 flex select-none items-center gap-1 border bg-background p-1 text-foreground shadow-md"
              style={{ left: hlMenu.left, top: hlMenu.top + 6 }}
              onMouseDown={(e) => e.preventDefault()}
            >
              <Button
                variant="ghost"
                size="sm"
                onClick={() => handleDeleteHighlight(hlMenu.id)}
                title="Удалить выделение"
              >
                <Trash2 />
                Удалить
              </Button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

// Раздел «Чтение»: загрузка книг FB2/EPUB (сервер конвертирует в HTML)
// и чтение их в модалке.
function Reading() {
  const queryClient = useQueryClient();
  const booksQuery = useBooks(true);
  // Запрос «открыть книгу» из другого раздела (карточка «Чтение» в «Дне»).
  const readingRequest = useAppStore((s) => s.readingRequest);
  const clearReadingRequest = useAppStore((s) => s.clearReadingRequest);
  const fileRef = useRef(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [openBook, setOpenBook] = useState(null);
  const [openJump, setOpenJump] = useState(null); // закладка для перехода
  const [deletingId, setDeletingId] = useState(null);
  const closeBook = () => {
    setOpenBook(null);
    setOpenJump(null);
  };

  const refresh = () => queryClient.invalidateQueries({ queryKey: ["books"] });

  const handlePick = async (e) => {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const book = await uploadBook(file);
      setNotice(`Книга «${book.title}» добавлена.`);
      refresh();
    } catch (err) {
      setError(err.message || "Не удалось добавить книгу");
    } finally {
      setBusy(false);
    }
  };

  const handleDelete = async (book) => {
    if (!window.confirm(`Удалить книгу «${book.title}»?`)) return;
    setDeletingId(book.id);
    setError("");
    try {
      await deleteBook(book.id);
      refresh();
    } catch (err) {
      setError(err.message || "Не удалось удалить книгу");
    } finally {
      setDeletingId(null);
    }
  };

  const books = booksQuery.data || [];

  // Запрос из «Дня»: открываем последнюю закладку (книгу с ней), а если
  // закладок нет — любую книгу (свежую — список идёт от новых к старым).
  useEffect(() => {
    if (!readingRequest) return;
    const list = booksQuery.data;
    if (!list) return;
    const target = readingRequest.bookId
      ? list.find((b) => b.id === readingRequest.bookId)
      : // «любая книга» — первая непрочитанная (прочитанные в конце списка)
        list.find((b) => !b.finished_at);
    clearReadingRequest();
    if (!target) return;
    setOpenJump(
      readingRequest.anchor != null
        ? { anchor: readingRequest.anchor, excerpt: readingRequest.excerpt || "" }
        : null,
    );
    setOpenBook(target);
  }, [readingRequest, booksQuery.data, clearReadingRequest]);

  return (
    <section>
      <h2 className="flex items-center gap-2 text-xl font-semibold">
        <BookOpenText className="size-5 text-muted-foreground" />
        Чтение
      </h2>

      {/* Загрузка книги */}
      <Card className="my-3" size="sm">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Upload className="size-4 text-muted-foreground" />
            Добавить книгу
          </CardTitle>
          <CardDescription>
            Поддерживаются FB2 (.fb2, .fb2.zip) и EPUB — сервер преобразует
            книгу в HTML вместе с картинками.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <input
            ref={fileRef}
            type="file"
            accept=".fb2,.zip,.epub,application/epub+zip"
            className="hidden"
            onChange={handlePick}
          />
          <div className="flex flex-wrap items-center gap-2">
            <Button
              onClick={() => fileRef.current?.click()}
              disabled={busy}
            >
              {busy ? <Loader2 className="animate-spin" /> : <Upload />}
              {busy ? "Обрабатываю…" : "Выбрать файл"}
            </Button>
            <span className="text-xs text-muted-foreground">
              Максимальный размер — 40 МБ
            </span>
          </div>

          {notice && (
            <p className="text-sm text-emerald-600 dark:text-emerald-400">
              {notice}
            </p>
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
      </Card>

      {/* Список книг */}
      {booksQuery.isLoading ? (
        <p className="text-sm text-muted-foreground">
          <Loader2 className="mr-1 inline size-4 animate-spin" />
          Загрузка книг…
        </p>
      ) : books.length === 0 ? (
        <Card className="my-3" size="sm">
          <CardContent>
            <p className="flex items-center gap-2 text-sm text-muted-foreground">
              <BookOpenText className="size-4" />
              Пока нет книг. Добавьте первую выше.
            </p>
          </CardContent>
        </Card>
      ) : (
        books.map((b) => {
          // Процент прочтения приходит с сервера (позиция последней закладки
          // от длины текста; отмеченная прочитанной книга — 100%).
          const percent = b.read_percent || 0;
          // Сколько дней заняло чтение — только для прочитанных книг.
          const days = readingDays(b.started_at, b.finished_at);
          return (
            <Card
              key={b.id}
              className={cn(
                "relative my-3 overflow-hidden",
                // Прочитанные — зелёные и в конце списка (сортирует сервер).
                b.finished_at && "border-emerald-500/50 bg-emerald-500/10",
              )}
              size="sm"
            >
              {/* Прогресс чтения: закрашиваем карточку слева направо. */}
              {!b.finished_at && percent > 0 && (
                <div
                  className="pointer-events-none absolute inset-y-0 left-0 bg-emerald-500/15"
                  style={{ width: `${percent}%` }}
                  aria-hidden="true"
                />
              )}
              <CardHeader className="relative">
                <div className="flex w-full items-start justify-between gap-2">
                  <div className="min-w-0">
                    <CardTitle className="wrap-break-word">{b.title}</CardTitle>
                    <CardDescription className="wrap-break-word">
                      {[
                        b.author || null,
                        b.started_at ? (
                          <span className="text-muted-foreground">
                            начато <DateDisplay date={b.started_at} />
                          </span>
                        ) : null,
                        b.finished_at ? (
                          <span className="text-emerald-600 dark:text-emerald-400">
                            прочитана <DateDisplay date={b.finished_at} />
                          </span>
                        ) : percent > 0 ? (
                          <span className="text-emerald-600 dark:text-emerald-400">
                            прочитано {percent}%
                          </span>
                        ) : null,
                        days != null ? (
                          <span className="text-muted-foreground">
                            за {days} {dayWord(days)}
                          </span>
                        ) : null,
                      ]
                        .filter(Boolean)
                        .map((part, i) => (
                          <span key={i}>
                            {i > 0 ? " · " : ""}
                            {part}
                          </span>
                        ))}
                    </CardDescription>
                  </div>
                  <div className="flex shrink-0 items-center gap-2">
                    <Button size="sm" onClick={() => setOpenBook(b)}>
                      <BookOpenText />
                      Читать
                    </Button>
                    <Button
                      variant="destructive"
                      size="sm"
                      onClick={() => handleDelete(b)}
                      disabled={deletingId === b.id}
                      title="Удалить книгу"
                      aria-label="Удалить книгу"
                    >
                      {deletingId === b.id ? (
                        <Loader2 className="animate-spin" />
                      ) : (
                        <Trash2 />
                      )}
                    </Button>
                  </div>
                </div>
              </CardHeader>
            </Card>
          );
        })
      )}

      {openBook && (
        <BookModal book={openBook} initialJump={openJump} onClose={closeBook} />
      )}
    </section>
  );
}

export default Reading;
