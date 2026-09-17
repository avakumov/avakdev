import { useEffect, useRef, useState } from "react";
import {
  useBooks,
  fetchBook,
  uploadBook,
  deleteBook,
  fetchBookmarks,
  addBookmark,
  deleteBookmark,
  fetchReadingTime,
  addReadingTime,
  setBookFinished,
} from "./api.js";
import { useQueryClient } from "@tanstack/react-query";
import { useAppStore } from "./store.js";
import { cn } from "@/lib/utils";
import { todayStr, formatClock } from "@/lib/formatDate.js";
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
  RotateCcw,
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

// Порог «дошли до конца книги» для вопроса о прочтении (px).
const BOOK_END_THRESHOLD = 40;

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
  // Закладки книги и текущее выделение в тексте ({ anchor, text }).
  const [bookmarks, setBookmarks] = useState([]);
  const [sel, setSel] = useState(null);
  const [panelOpen, setPanelOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState(null); // { text, error }
  const [pendingJump, setPendingJump] = useState(initialJump); // закладка для перехода
  // Прочитана ли книга и спрашиваем ли об этом (дошли до конца текста).
  const [finished, setFinished] = useState(Boolean(book.finished_at));
  const [askFinished, setAskFinished] = useState(false);
  const [savingFinished, setSavingFinished] = useState(false);
  const endAskedRef = useRef(false);
  const scrollRef = useRef(null);
  const layerRef = useRef(null);
  const textRef = useRef(null);
  const noticeTimer = useRef(null);
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

  // Сообщение над текстом («закладка добавлена», «не удалось сохранить…»).
  const showNotice = (text, isError = false) => {
    setNotice({ text, error: isError });
    clearTimeout(noticeTimer.current);
    noticeTimer.current = setTimeout(() => setNotice(null), 2500);
  };

  useEffect(() => () => clearTimeout(noticeTimer.current), []);

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

  // Загрузка закладок при открытии книги.
  useEffect(() => {
    let alive = true;
    setBookmarks([]);
    setSel(null);
    setPanelOpen(false);
    fetchBookmarks(book.id)
      .then((list) => {
        if (alive) setBookmarks(list);
      })
      .catch(() => {
        // Закладки не критичны для чтения — молча оставляем список пустым.
      });
    return () => {
      alive = false;
    };
  }, [book.id]);

  // Текущее выделение внутри текста книги (или null).
  const readSelection = () => {
    const root = textRef.current;
    const s = window.getSelection();
    if (!root || !s || s.rangeCount === 0 || s.isCollapsed) return null;
    const range = s.getRangeAt(0);
    if (!root.contains(range.commonAncestorContainer)) return null;
    return {
      anchor: offsetOfPoint(root, range.startContainer, range.startOffset),
      text: s.toString(),
    };
  };

  // Следим за выделением: мышь, клавиатура, тач — всё приходит сюда.
  useEffect(() => {
    const track = () => setSel(readSelection());
    document.addEventListener("selectionchange", track);
    return () => document.removeEventListener("selectionchange", track);
  }, []);

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
    <div className="fixed inset-0 z-50 flex flex-col bg-[color-mix(in_oklch,var(--background)_90%,var(--foreground))] text-[color-mix(in_oklch,var(--foreground)_85%,var(--background))]">
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

        {/* Закладка на выделенном фрагменте текста */}
        <Button
          variant="outline"
          size="icon"
          onMouseDown={(e) => e.preventDefault()}
          onClick={handleAddBookmark}
          disabled={!sel || saving}
          title={
            sel
              ? "Поставить закладку на выделенном тексте"
              : "Выделите текст в книге, чтобы поставить закладку"
          }
          aria-label="Поставить закладку"
        >
          {saving ? <Loader2 className="animate-spin" /> : <BookmarkPlus />}
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

      {/* Сообщение над текстом */}
      {notice && (
        <div
          className={
            "shrink-0 border-b px-4 py-1.5 text-xs " +
            (notice.error
              ? "text-destructive"
              : "text-emerald-600 dark:text-emerald-400")
          }
          role={notice.error ? "alert" : undefined}
        >
          {notice.text}
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
              dangerouslySetInnerHTML={{ __html: data.html }}
            />
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
  const [finishingId, setFinishingId] = useState(null);
  const closeBook = () => {
    setOpenBook(null);
    setOpenJump(null);
  };

  const refresh = () => queryClient.invalidateQueries({ queryKey: ["books"] });

  // Отметить книгу прочитанной / вернуть в чтение (прочитанные уезжают
  // в конец списка и не предлагаются в «Дне»).
  const handleToggleFinished = async (book) => {
    setFinishingId(book.id);
    setError("");
    try {
      await setBookFinished(book.id, !book.finished_at);
      refresh();
      queryClient.invalidateQueries({ queryKey: ["last-bookmark"] });
    } catch (err) {
      setError(err.message || "Не удалось отметить книгу");
    } finally {
      setFinishingId(null);
    }
  };

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
                      {b.author}
                      {b.finished_at ? (
                        <span className="text-emerald-600 dark:text-emerald-400">
                          {b.author ? " · прочитана" : "прочитана"}
                        </span>
                      ) : (
                        percent > 0 && (
                          <span className="text-emerald-600 dark:text-emerald-400">
                            {b.author
                              ? ` · прочитано ${percent}%`
                              : `прочитано ${percent}%`}
                          </span>
                        )
                      )}
                    </CardDescription>
                  </div>
                  <div className="flex shrink-0 items-center gap-2">
                    <Button size="sm" onClick={() => setOpenBook(b)}>
                      <BookOpenText />
                      Читать
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => handleToggleFinished(b)}
                      disabled={finishingId === b.id}
                      title={
                        b.finished_at
                          ? "Вернуть книгу в чтение"
                          : "Отметить книгу прочитанной"
                      }
                      aria-label={
                        b.finished_at
                          ? "Вернуть книгу в чтение"
                          : "Отметить книгу прочитанной"
                      }
                    >
                      {finishingId === b.id ? (
                        <Loader2 className="animate-spin" />
                      ) : b.finished_at ? (
                        <RotateCcw />
                      ) : (
                        <BookCheck />
                      )}
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
