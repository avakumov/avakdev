import { useDrafts } from "./api.js";
import { useAppStore } from "./store.js";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardDescription,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { StickyNote, Plus, Pencil, Loader2 } from "lucide-react";

// Раздел «Заметки»: список быстрых записей-черновиков. Пишутся и правятся в
// панели у кнопки «з» (см. NotesDock). У записи одно действие — редактирование:
// удаление доступно внутри формы правки.
function Notes() {
  const draftsQuery = useDrafts(true);
  const openDraftEditor = useAppStore((s) => s.openDraftEditor);

  const drafts = draftsQuery.data || [];

  return (
    <section>
      <h2 className="flex items-center gap-2 text-xl font-semibold">
        <StickyNote className="size-5 text-muted-foreground" />
        Заметки
      </h2>

      {/* Новая заметка */}
      <Card className="my-3" size="sm">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Plus className="size-4 text-muted-foreground" />
            Новая заметка
          </CardTitle>
          <CardDescription>
            Текст вводится в панели у кнопки «з» слева внизу: Ctrl+Enter —
            сохранить, Enter — новая строка. Кнопка работает на любой странице.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Button onClick={() => openDraftEditor({})}>
            <Plus />
            Написать заметку
          </Button>
        </CardContent>
      </Card>

      {/* Список заметок */}
      {draftsQuery.isLoading ? (
        <p className="text-sm text-muted-foreground">
          <Loader2 className="mr-1 inline size-4 animate-spin" />
          Загрузка заметок…
        </p>
      ) : drafts.length === 0 ? (
        <Card className="my-3" size="sm">
          <CardContent>
            <p className="flex items-center gap-2 text-sm text-muted-foreground">
              <StickyNote className="size-4" />
              Пока нет заметок. Добавьте первую выше.
            </p>
          </CardContent>
        </Card>
      ) : (
        drafts.map((note) => (
          <Card key={note.id} className="my-3" size="sm">
            <CardContent className="flex items-start gap-2">
              <p className="min-w-0 flex-1 text-sm wrap-break-word whitespace-pre-wrap">
                {note.content}
              </p>
              <Button
                variant="ghost"
                size="icon"
                onClick={() =>
                  openDraftEditor({ id: note.id, content: note.content })
                }
                title="Редактировать заметку"
                aria-label="Редактировать заметку"
                className="shrink-0"
              >
                <Pencil />
              </Button>
            </CardContent>
          </Card>
        ))
      )}
    </section>
  );
}

export default Notes;
