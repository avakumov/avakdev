import { useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { createDraft, updateDraft, deleteDraft } from "./api.js";
import { useAppStore } from "./store.js";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { AlertCircle, Loader2, Save, Trash2, X } from "lucide-react";

// Новые записи пишутся жёлтым (черновик); правка существующей — обычным цветом.
const NEW_DRAFT_TEXT_CLASS = "text-amber-500 dark:text-amber-400";

// Общее поведение полей: Ctrl+Enter сохраняет, Enter переводит строку,
// Esc закрывает панель (и гасит событие, чтобы не закрыть книгу под ней).
function draftKeyHandler(save, close) {
  return (e) => {
    if ((e.ctrlKey || e.metaKey) && e.key === "Enter") {
      e.preventDefault();
      save();
    } else if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      close();
    }
  };
}

// Меню (сайдбар) бывает не видно: на узких экранах его нет, а на широких его
// перекрывает полноэкранное окно (книга, модалки). Тогда кнопку «з» надо
// прижать к левому краю — проверяем это по самой вёрстке: «достаём» точку
// внизу сайдбара и смотрим, он ли там оказался сверху.
function useMenuCovered(active) {
  const [covered, setCovered] = useState(false);

  useEffect(() => {
    if (!active) return;
    const check = () => {
      const aside = document.querySelector("aside");
      if (!aside) {
        setCovered(true);
        return;
      }
      const r = aside.getBoundingClientRect();
      if (r.width === 0 || r.right <= 0) {
        setCovered(true); // меню убрано за край (мобильный)
        return;
      }
      const el = document.elementFromPoint(r.right - 20, r.bottom - 40);
      setCovered(!el || !aside.contains(el));
    };
    check();
    // Окна поверх страницы появляются/исчезают без внешних событий,
    // поэтому периодически перепроверяем.
    const timer = setInterval(check, 500);
    window.addEventListener("resize", check);
    return () => {
      clearInterval(timer);
      window.removeEventListener("resize", check);
    };
  }, [active]);

  return covered;
}

// Быстрая заметка: круглая жёлтая кнопка «з» слева внизу, а справа от неё —
// панель ввода. Рендерится в App, поэтому доступна на всех страницах,
// в том числе поверх открытой книги.
function NotesDock() {
  const queryClient = useQueryClient();
  const editor = useAppStore((s) => s.draftEditor);
  const openEditor = useAppStore((s) => s.openDraftEditor);
  const closeEditor = useAppStore((s) => s.closeDraftEditor);
  const [text, setText] = useState("");
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState("");
  const areaRef = useRef(null);

  const isNew = Boolean(editor) && !editor.id;
  const busy = saving || deleting;
  // Пока панель открыта, точку внизу меню занимает сама панель — не проверяем,
  // иначе кнопка прыгала бы вместе с вводом.
  const menuCovered = useMenuCovered(!editor);
  const anchorClass = menuCovered ? "left-4" : "left-4 lg:left-68";

  // Открыли панель: подставляем текст (правка) и ставим фокус в поле.
  useEffect(() => {
    if (!editor) return;
    setText(editor.content || "");
    setError("");
    const frame = requestAnimationFrame(() => areaRef.current?.focus());
    return () => cancelAnimationFrame(frame);
  }, [editor]);

  const save = async () => {
    if (busy) return;
    if (!text.trim()) {
      setError("Пустая заметка — сохранять нечего");
      return;
    }
    setSaving(true);
    setError("");
    try {
      if (editor.id) await updateDraft(editor.id, text);
      else await createDraft(text);
      await queryClient.invalidateQueries({ queryKey: ["drafts"] });
      closeEditor();
    } catch (err) {
      setError(err.message || "Не удалось сохранить заметку");
    } finally {
      setSaving(false);
    }
  };

  // Удаление живёт только здесь — на карточке заметки кнопки удаления нет.
  const handleDelete = async () => {
    if (!editor?.id || busy) return;
    if (!window.confirm("Удалить заметку?")) return;
    setDeleting(true);
    setError("");
    try {
      await deleteDraft(editor.id);
      await queryClient.invalidateQueries({ queryKey: ["drafts"] });
      closeEditor();
    } catch (err) {
      setError(err.message || "Не удалось удалить заметку");
    } finally {
      setDeleting(false);
    }
  };

  // Круглая жёлтая кнопка «з». Пока видно меню, она стоит справа от него,
  // иначе — у левого края экрана. Когда панель открыта, кнопка её закрывает.
  const zButton = (
    <button
      type="button"
      onClick={editor ? closeEditor : () => openEditor({})}
      disabled={busy}
      title={editor ? "Закрыть заметку" : "Быстрая заметка"}
      aria-label={editor ? "Закрыть заметку" : "Быстрая заметка"}
      className="flex size-10 shrink-0 cursor-pointer items-center justify-center rounded-full bg-amber-400 shadow-md transition-colors hover:bg-amber-300 disabled:cursor-not-allowed disabled:opacity-60"
    >
      {/* Буква «з» курсивом — как рукописная пометка. Сдвиг translate-y —
          оптическая поправка: строчная буква без выносных элементов в любом
          serif-шрифте сидит на пару пикселей ниже центра строки. Margin’ом
          такую поправку делать нельзя — во flex он сдвигает бокс вниз. */}
      <span className="-translate-y-0.5 font-serif text-2xl leading-none font-medium text-neutral-900 italic">
        з
      </span>
    </button>
  );

  if (!editor) {
    return (
      <div className={cn("fixed bottom-4 z-70", anchorClass)}>{zButton}</div>
    );
  }

  return (
    <div className={cn("fixed bottom-4 z-70 flex items-end gap-2", anchorClass)}>
      {zButton}
      <div className="w-[min(30rem,calc(100vw-5.5rem))] border bg-background p-2 shadow-md">
        <div className="flex items-start gap-1">
          <textarea
            ref={areaRef}
            value={text}
            rows={5}
            onChange={(e) => setText(e.target.value)}
            onKeyDown={draftKeyHandler(save, closeEditor)}
            placeholder="Текст заметки…"
            aria-label="Текст заметки"
            className={cn(
              "min-w-0 flex-1 resize-y bg-transparent text-sm leading-relaxed outline-none placeholder:text-muted-foreground",
              isNew && NEW_DRAFT_TEXT_CLASS,
            )}
          />
          {/* Закрытие — только крестиком. */}
          <Button
            variant="ghost"
            size="icon"
            onClick={closeEditor}
            disabled={busy}
            title="Закрыть"
            aria-label="Закрыть"
            className="size-6 shrink-0"
          >
            <X className="size-3.5" />
          </Button>
        </div>

        {error && (
          <p
            className="mt-1 flex items-center gap-1.5 text-xs text-destructive"
            role="alert"
          >
            <AlertCircle className="size-3.5" />
            {error}
          </p>
        )}

        <div className="mt-1 flex flex-wrap items-center gap-2">
          <span className="mr-auto text-[11px] text-muted-foreground">
            Ctrl+Enter — сохранить
          </span>
          {!isNew && (
            <Button
              variant="destructive"
              size="sm"
              onClick={handleDelete}
              disabled={busy}
            >
              {deleting ? <Loader2 className="animate-spin" /> : <Trash2 />}
              Удалить
            </Button>
          )}
          <Button size="sm" onClick={save} disabled={busy}>
            {saving ? <Loader2 className="animate-spin" /> : <Save />}
            Сохранить
          </Button>
        </div>
      </div>
    </div>
  );
}

export default NotesDock;
