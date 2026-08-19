import { useEffect, useRef, useState } from "react";
import {
  useKnowledge,
  generateKnowledge,
  createKnowledge,
  updateKnowledge,
  repeatKnowledge,
  deleteKnowledge,
  synthesizeNoteAudio,
  getNoteAudio,
} from "./api.js";
import MarkdownView from "./MarkdownView.jsx";
import { useQueryClient } from "@tanstack/react-query";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardDescription,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import {
  Sparkles,
  Save,
  Loader2,
  AlertCircle,
  BookOpen,
  Trash2,
  Check,
  Edit,
  X,
  Repeat,
  ChevronDown,
  ChevronRight,
  Volume2,
  VolumeX,
} from "lucide-react";

// Строка-обёртка над обычным textarea (в стилистике shadcn/ui).
function Textarea({ className, ...props }) {
  return (
    <textarea
      data-slot="textarea"
      className={
        "w-full min-h-32 rounded-lg border border-input bg-transparent px-3 py-2 text-sm leading-relaxed text-foreground transition-colors outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-input/30 resize-y " +
        (className || "")
      }
      {...props}
    />
  );
}

// Отрисовка Markdown-контента вынесена в общий компонент MarkdownView.jsx.

// Форма генерации конспекта по теме через ИИ (DeepSeek).
// После генерации показывает предпросмотр с возможностью сохранить.
function GenerateForm({ onSaved }) {
  const [topic, setTopic] = useState("");
  const [generated, setGenerated] = useState(null);
  const [generating, setGenerating] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  const canGenerate = Boolean(topic.trim());

  const handleGenerate = async () => {
    if (!canGenerate) return;
    setGenerating(true);
    setError("");
    setNotice("");
    setGenerated(null);
    try {
      const data = await generateKnowledge(topic);
      setGenerated(data);
    } catch (err) {
      setError(err.message || "Не удалось сгенерировать конспект");
    } finally {
      setGenerating(false);
    }
  };

  const handleSave = async () => {
    if (!generated) return;
    setSaving(true);
    setError("");
    try {
      await createKnowledge({
        topic,
        title: generated.title || topic,
        content: generated.content,
      });
      setNotice("Конспект сохранён.");
      setGenerated(null);
      setTopic("");
      onSaved();
    } catch (err) {
      setError(err.message || "Не удалось сохранить конспект");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card className="my-3" size="sm">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Sparkles className="size-4 text-muted-foreground" />
          Генерация конспекта
        </CardTitle>
        <CardDescription>
          Введите тему — ИИ (DeepSeek) составит краткий конспект для повторения.
          Удачные варианты сохраняйте.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="flex flex-col gap-2 sm:flex-row">
          <Input
            value={topic}
            onChange={(e) => setTopic(e.target.value)}
            placeholder="Например: сортировка пузырьком, HTTP-методы, SQL JOIN…"
            onKeyDown={(e) => e.key === "Enter" && handleGenerate()}
          />
          <Button
            onClick={handleGenerate}
            disabled={generating || !canGenerate}
          >
            {generating ? <Loader2 className="animate-spin" /> : <Sparkles />}
            {generating ? "Генерирую…" : "Сгенерировать"}
          </Button>
        </div>

        {notice && (
          <p className="flex items-center gap-1.5 text-sm text-emerald-600">
            <Check className="size-4" />
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

        {generated && (
          <div className="space-y-3 rounded-lg border bg-muted/40 p-3">
            <p className="text-sm font-medium">Предпросмотр конспекта:</p>
            <MarkdownView className="max-h-80 overflow-y-auto whitespace-pre-wrap">
              {generated.content}
            </MarkdownView>
            <div className="flex flex-wrap items-center gap-3">
              <Button onClick={handleSave} disabled={saving}>
                {saving ? <Loader2 className="animate-spin" /> : <Save />}
                {saving ? "Сохраняю…" : "Сохранить конспект"}
              </Button>
              <Button
                variant="ghost"
                onClick={() => setGenerated(null)}
                disabled={saving}
              >
                <X />
                Отменить
              </Button>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

// Одна карточка сохранённого конспекта: заголовок, счётчик повторений,
// кнопка «Я повторил», редактирование контента, озвучка и удаление.
function NoteCard({ note, onSaved }) {
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(note.content);
  const [saving, setSaving] = useState(false);
  const [repeating, setRepeating] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState("");
  const [audioUrl, setAudioUrl] = useState(null); // blob-URL сгенерированного аудио
  const [ttsLoading, setTtsLoading] = useState(false);
  const [audioCheck, setAudioCheck] = useState(false); // уже проверяли наличие аудио
  const audioUrlRef = useRef(null); // отслеживаем текущий blob-URL для cleanup

  // Освобождаем все blob-URL аудио при размонтировании компонента.
  useEffect(() => {
    return () => {
      if (audioUrlRef.current) URL.revokeObjectURL(audioUrlRef.current);
    };
  }, []);

  const handleSave = async () => {
    setSaving(true);
    setError("");
    try {
      await updateKnowledge(note.id, {
        title: note.title,
        content: draft,
      });
      setEditing(false);
      onSaved();
    } catch (err) {
      setError(err.message || "Не удалось обновить конспект");
    } finally {
      setSaving(false);
    }
  };

  const handleRepeat = async () => {
    setRepeating(true);
    setError("");
    try {
      await repeatKnowledge(note.id);
      onSaved();
    } catch (err) {
      setError(err.message || "Не удалось отметить повторение");
    } finally {
      setRepeating(false);
    }
  };

  const handleDelete = async () => {
    if (!window.confirm("Удалить этот конспект?")) return;
    setDeleting(true);
    setError("");
    try {
      await deleteKnowledge(note.id);
      onSaved();
    } catch (err) {
      setError(err.message || "Не удалось удалить конспект");
    } finally {
      setDeleting(false);
    }
  };

  const toggle = () => {
    if (editing) return;
    const nextOpen = !open;
    setOpen(nextOpen);
    // При открытии карточки проверяем, есть ли уже сгенерированное аудио.
    if (nextOpen && !audioCheck) {
      ensureAudioChecked();
    }
  };

  // Проверяем, есть ли уже сгенерированное аудио для конспекта.
  // Если есть — подгружаем blob-URL, чтобы сразу показать плеер.
  const ensureAudioChecked = async () => {
    if (audioCheck) return;
    setAudioCheck(true);
    try {
      const blob = await getNoteAudio(note.id);
      if (blob) {
        const url = URL.createObjectURL(blob);
        // Если ранее был blob-URL — освобождаем его.
        if (audioUrlRef.current) URL.revokeObjectURL(audioUrlRef.current);
        audioUrlRef.current = url;
        setAudioUrl(url);
      }
    } catch (err) {
      // Не критично — просто не показываем плеер, кнопка «Озвучить» останется.
    }
  };

  // Генерируем аудио через Yandex SpeechKit.
  const handleSynthesize = async () => {
    setTtsLoading(true);
    setError("");
    try {
      const blob = await synthesizeNoteAudio(note.id);
      const url = URL.createObjectURL(blob);
      // Если ранее был blob-URL — освобождаем его.
      if (audioUrlRef.current) URL.revokeObjectURL(audioUrlRef.current);
      audioUrlRef.current = url;
      setAudioUrl(url);
    } catch (err) {
      setError(err.message || "Не удалось сгенерировать аудио");
    } finally {
      setTtsLoading(false);
    }
  };

  return (
    <Card className="my-3" size="sm">
      <CardHeader className="cursor-pointer select-none" onClick={toggle}>
        <div className="flex w-full flex-wrap items-center justify-between gap-2">
          <CardTitle className="flex items-center gap-2">
            {open ? (
              <ChevronDown className="size-4 text-muted-foreground" />
            ) : (
              <ChevronRight className="size-4 text-muted-foreground" />
            )}
            <BookOpen className="size-4 text-muted-foreground" />
            <span className="max-w-64 truncate">
              {note.content.split("\n")[0] || note.title || note.topic}
            </span>
          </CardTitle>
          <Badge variant="secondary" className="gap-1">
            <Repeat className="size-3.5" />
            Повторений: {note.repetitions}
          </Badge>
        </div>
      </CardHeader>

      {open && (
        <CardContent className="space-y-3">
          {error && (
            <p
              className="flex items-center gap-1.5 text-sm text-destructive"
              role="alert"
            >
              <AlertCircle className="size-4" />
              {error}
            </p>
          )}

          {editing ? (
            <div className="space-y-3">
              <Textarea
                className="min-h-56"
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
              />
              <div className="flex flex-wrap gap-2">
                <Button onClick={handleSave} disabled={saving}>
                  {saving ? <Loader2 className="animate-spin" /> : <Save />}
                  {saving ? "Сохраняю…" : "Сохранить"}
                </Button>
                <Button
                  variant="ghost"
                  onClick={() => {
                    setEditing(false);
                    setDraft(note.content);
                    setError("");
                  }}
                >
                  <X />
                  Отмена
                </Button>
              </div>
            </div>
          ) : (
            <>
              <div className="flex flex-wrap items-center gap-2">
                <Button
                  variant="outline"
                  onClick={handleRepeat}
                  disabled={repeating}
                >
                  {repeating ? <Loader2 className="animate-spin" /> : <Check />}
                  {repeating ? "Отмечаю…" : "Я повторил"}
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => {
                    setDraft(note.content);
                    setEditing(true);
                  }}
                >
                  <Edit />
                  Редактировать
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleSynthesize}
                  disabled={ttsLoading}
                  title={
                    audioUrl
                      ? "Аудио уже сгенерировано — воспроизвести"
                      : "Озвучить конспект (Yandex SpeechKit)"
                  }
                >
                  {ttsLoading ? (
                    <Loader2 className="animate-spin" />
                  ) : audioUrl ? (
                    <Volume2 />
                  ) : (
                    <VolumeX />
                  )}
                  {ttsLoading
                    ? "Озвучиваю…"
                    : audioUrl
                      ? "Слушать"
                      : "Озвучить"}
                </Button>
                <Button
                  variant="destructive"
                  size="sm"
                  onClick={handleDelete}
                  disabled={deleting}
                >
                  {deleting ? <Loader2 className="animate-spin" /> : <Trash2 />}
                  Удалить
                </Button>
              </div>

              {audioUrl && (
                <audio controls className="w-full rounded-md" src={audioUrl}>
                  Ваш браузер не поддерживает аудио.
                </audio>
              )}

              <MarkdownView>{note.content}</MarkdownView>
            </>
          )}
        </CardContent>
      )}
    </Card>
  );
}

// Главный компонент: генерация конспектов + список сохранённых.
function Knowledge() {
  const queryClient = useQueryClient();
  const knowledgeQuery = useKnowledge(true);

  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: ["knowledge"] });

  const savedNotes = knowledgeQuery.data;

  return (
    <section>
      <GenerateForm onSaved={refresh} />

      {knowledgeQuery.isLoading ? (
        <p className="text-sm text-muted-foreground">
          <Loader2 className="mr-1 inline size-4 animate-spin" />
          Загрузка конспектов…
        </p>
      ) : knowledgeQuery.isError ? (
        <p className="text-sm text-destructive">
          Ошибка: {knowledgeQuery.error?.message}
        </p>
      ) : savedNotes && savedNotes.length > 0 ? (
        savedNotes.map((n) => (
          <NoteCard key={n.id} note={n} onSaved={refresh} />
        ))
      ) : (
        <Card className="my-3" size="sm">
          <CardContent>
            <p className="flex items-center gap-2 text-sm text-muted-foreground">
              <BookOpen className="size-4" />
              Пока нет сохранённых конспектов. Сгенерируйте первый выше.
            </p>
          </CardContent>
        </Card>
      )}
    </section>
  );
}

export default Knowledge;
