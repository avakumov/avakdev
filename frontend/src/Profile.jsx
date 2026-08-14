import { useState, useRef } from "react";
import {
  useProfile,
  saveProfile,
  generateResume,
  saveResumeText,
  profileResumeUrl,
  uploadProfilePhoto,
  deleteProfilePhoto,
} from "./api.js";
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
import {
  FileText,
  Sparkles,
  ExternalLink,
  Edit,
  Save,
  X,
  Loader2,
  AlertCircle,
  User,
  ImagePlus,
  Trash2,
} from "lucide-react";

// Получить data URI фото из полей профиля.
function photoDataUri(photoData, photoMime) {
  if (!photoData) return "";
  return `data:${photoMime || "image/jpeg"};base64,${photoData}`;
}

// Подставить актуальное фото в HTML резюме на место плейсхолдера
// <img id="resume-photo">. Если фото нет — удаляет тег плейсхолдера.
function applyPhotoToHtml(html, photoData, photoMime) {
  if (!html) return html;
  const src = photoDataUri(photoData, photoMime);
  // Приоритет — наш плейсхолдер <img id="resume-photo">;
  // иначе план Б — любой тег <img class="photo">.
  // В обоих regex есть флаг g, чтобы заменять все вхождения.
  const byId = /<img[^>]*id="resume-photo"[^>]*>/gi;
  const byClass = /<img[^>]*class="[^"]*\bphoto\b[^"]*"[^>]*>/gi;

  if (!src) {
    if (byId.test(html)) {
      byId.lastIndex = 0;
      return html.replace(byId, "");
    }
    byClass.lastIndex = 0;
    return html.replace(byClass, "");
  }

  const replacement = `<img id="resume-photo" class="photo" alt="Фото" src="${src}">`;
  if (byId.test(html)) {
    byId.lastIndex = 0;
    return html.replace(byId, replacement);
  }
  byClass.lastIndex = 0;
  return html.replace(byClass, replacement);
}

// Строка-обёртка над обычным textarea (в стилистике shadcn/ui).
function Textarea({ className, ...props }) {
  return (
    <textarea
      data-slot="textarea"
      className={
        "w-full min-h-28 rounded-lg border border-input bg-transparent px-3 py-2 text-sm leading-relaxed text-foreground transition-colors outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-input/30 resize-y " +
        (className || "")
      }
      {...props}
    />
  );
}

// Карточка описания профиля: поле для ввода и кнопка сохранения.
function DescriptionForm({ initial, onSaved }) {
  const [description, setDescription] = useState(initial || "");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const handleSave = async () => {
    setSaving(true);
    setError("");
    try {
      await saveProfile(description);
      onSaved();
    } catch (err) {
      setError(err.message || "Не удалось сохранить описание");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card className="my-3" size="sm">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <User className="size-4 text-muted-foreground" />О себе
        </CardTitle>
        <CardDescription>
          Опишите себя, свои навыки и опыт — на основе этого будет сгенерировано
          резюме.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <Textarea
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          placeholder="Например: backend-разработчик, 5 лет опыта с Go и PostgreSQL, люблю автоматизацию…"
        />
        {error && (
          <p
            className="flex items-center gap-1.5 text-sm text-destructive"
            role="alert"
          >
            <AlertCircle className="size-4" />
            {error}
          </p>
        )}
        <div className="flex gap-2">
          <Button onClick={handleSave} disabled={saving}>
            {saving ? <Loader2 className="animate-spin" /> : <Save />}
            {saving ? "Сохраняю…" : "Сохранить описание"}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

// Карточка фото: показ, загрузка и удаление фотографии для резюме.
function PhotoForm({ photoData, photoMime, onChanged }) {
  const fileInputRef = useRef(null);
  const [uploading, setUploading] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState("");

  const hasPhoto = Boolean(photoData);

  const handleFile = async (e) => {
    const file = e.target.files?.[0];
    e.target.value = ""; // разрешить повторный выбор того же файла
    if (!file) return;
    setUploading(true);
    setError("");
    try {
      await uploadProfilePhoto(file);
      onChanged();
    } catch (err) {
      setError(err.message || "Не удалось загрузить фото");
    } finally {
      setUploading(false);
    }
  };

  const handleDelete = async () => {
    setDeleting(true);
    setError("");
    try {
      await deleteProfilePhoto();
      onChanged();
    } catch (err) {
      setError(err.message || "Не удалось удалить фото");
    } finally {
      setDeleting(false);
    }
  };

  return (
    <Card className="my-3" size="sm">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <ImagePlus className="size-4 text-muted-foreground" />
          Фото
        </CardTitle>
        <CardDescription>
          Добавьте фотографию — она появится в начале резюме.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {hasPhoto && (
          <img
            src={photoDataUri(photoData, photoMime)}
            alt="Фото профиля"
            className="size-28 rounded-full border-3 border-primary object-cover"
          />
        )}
        <input
          ref={fileInputRef}
          type="file"
          accept="image/jpeg,image/png,image/webp,image/gif"
          className="hidden"
          onChange={handleFile}
        />
        {error && (
          <p
            className="flex items-center gap-1.5 text-sm text-destructive"
            role="alert"
          >
            <AlertCircle className="size-4" />
            {error}
          </p>
        )}
        <div className="flex flex-wrap gap-2">
          <Button
            variant="outline"
            onClick={() => fileInputRef.current?.click()}
            disabled={uploading}
          >
            {uploading ? <Loader2 className="animate-spin" /> : <ImagePlus />}
            {uploading
              ? "Загружаю…"
              : hasPhoto
                ? "Заменить фото"
                : "Загрузить фото"}
          </Button>
          {hasPhoto && (
            <Button
              variant="destructive"
              onClick={handleDelete}
              disabled={deleting}
            >
              {deleting ? <Loader2 className="animate-spin" /> : <Trash2 />}
              Удалить
            </Button>
          )}
        </div>
      </CardContent>
    </Card>
  );
}

// Карточка генерации: запускает DeepSeek из сохранённого описания.
function GenerateForm({ description, onGenerated }) {
  const [generating, setGenerating] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  const handleGenerate = async () => {
    setGenerating(true);
    setError("");
    setNotice("");
    try {
      await generateResume();
      setNotice("Резюме успешно сгенерировано.");
      onGenerated();
    } catch (err) {
      setError(err.message || "Не удалось сгенерировать резюме");
    } finally {
      setGenerating(false);
    }
  };

  const canGenerate = Boolean(description && description.trim());

  return (
    <Card className="my-3" size="sm">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Sparkles className="size-4 text-muted-foreground" />
          Генерация резюме
        </CardTitle>
        <CardDescription>
          Нажмите, чтобы сгенерировать резюме по описанию с помощью ИИ
          (DeepSeek).
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {notice && (
          <p className="flex items-center gap-1.5 text-sm text-emerald-600">
            <Sparkles className="size-4" />
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
        <Button onClick={handleGenerate} disabled={generating || !canGenerate}>
          {generating ? <Loader2 className="animate-spin" /> : <Sparkles />}
          {generating ? "Генерирую…" : "Сгенерировать резюме"}
        </Button>
        {!canGenerate && (
          <p className="text-xs text-muted-foreground">
            Сначала сохраните описание в блоке «О себе».
          </p>
        )}
      </CardContent>
    </Card>
  );
}

// Карточка готового резюме: просмотр, открытие отдельной страницы и редактирование.
function ResumeView({ resume, photoData, photoMime, onChanged }) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(resume);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const handleSave = async () => {
    setSaving(true);
    setError("");
    try {
      await saveResumeText(draft);
      setEditing(false);
      onChanged();
    } catch (err) {
      setError(err.message || "Не удалось сохранить резюме");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card className="my-3" size="sm">
      <CardHeader>
        <div className="flex w-full flex-wrap items-center justify-between gap-2">
          <CardTitle className="flex items-center gap-2">
            <FileText className="size-4 text-muted-foreground" />
            Резюме
          </CardTitle>
          {!editing && (
            <div className="flex flex-wrap gap-2">
              <Button variant="outline" size="sm" asChild>
                <a
                  href={profileResumeUrl()}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  <ExternalLink />
                  Открыть резюме
                </a>
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  setDraft(resume);
                  setEditing(true);
                }}
              >
                <Edit />
                Редактировать
              </Button>
            </div>
          )}
        </div>
      </CardHeader>
      <CardContent>
        {editing ? (
          <div className="space-y-3">
            <Textarea
              className="min-h-72 font-mono"
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
            />
            {error && (
              <p
                className="flex items-center gap-1.5 text-sm text-destructive"
                role="alert"
              >
                <AlertCircle className="size-4" />
                {error}
              </p>
            )}
            <div className="flex flex-wrap gap-2">
              <Button onClick={handleSave} disabled={saving}>
                {saving ? <Loader2 className="animate-spin" /> : <Save />}
                {saving ? "Сохраняю…" : "Сохранить"}
              </Button>
              <Button
                variant="ghost"
                onClick={() => {
                  setEditing(false);
                  setError("");
                }}
              >
                <X />
                Отмена
              </Button>
            </div>
          </div>
        ) : (
          <iframe
            title="Превью резюме"
            srcDoc={applyPhotoToHtml(resume, photoData, photoMime)}
            className="h-150 w-full rounded-lg border bg-white"
          />
        )}
      </CardContent>
    </Card>
  );
}

// Главный компонент: профиль и генерация резюме.
function Profile() {
  const queryClient = useQueryClient();
  const profileQuery = useProfile(true);

  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: ["profile"] });

  const profile = profileQuery.data;
  const hasResume = Boolean(profile && profile.resume && profile.resume.trim());

  return (
    <section>
      {profileQuery.isLoading ? (
        <p className="text-sm text-muted-foreground">
          <Loader2 className="mr-1 inline size-4 animate-spin" />
          Загрузка профиля…
        </p>
      ) : profileQuery.isError ? (
        <p className="text-sm text-destructive">
          Ошибка: {profileQuery.error?.message}
        </p>
      ) : (
        <>
          <DescriptionForm
            key="desc"
            initial={profile?.description}
            onSaved={refresh}
          />

          <PhotoForm
            photoData={profile?.photo_data}
            photoMime={profile?.photo_mime}
            onChanged={refresh}
          />

          <GenerateForm
            description={profile?.description}
            onGenerated={refresh}
          />

          {hasResume ? (
            <ResumeView
              resume={profile.resume}
              photoData={profile?.photo_data}
              photoMime={profile?.photo_mime}
              onChanged={refresh}
            />
          ) : (
            <Card className="my-3" size="sm">
              <CardContent>
                <p className="flex items-center gap-2 text-sm text-muted-foreground">
                  <FileText className="size-4" />
                  Резюме ещё не создано. Заполните описание и сгенерируйте его.
                </p>
              </CardContent>
            </Card>
          )}
        </>
      )}
    </section>
  );
}

export default Profile;
