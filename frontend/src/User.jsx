import { useEffect, useRef, useState } from "react";
import { updateMe, updateAvatar, linkTelegram, unlinkTelegram } from "./api.js";
import { useQueryClient } from "@tanstack/react-query";
import { useAppStore } from "./store.js";
import {
  Avatar,
  AvatarFallback,
  AvatarImage,
} from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { Slider } from "@/components/ui/slider";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardDescription,
} from "@/components/ui/card";
import UserAvatar from "@/components/UserAvatar.jsx";
import { AVATAR_PRESETS } from "@/lib/avatars.js";
import {
  NotificationsCard,
  CreateNotificationModal,
} from "./Notifications.jsx";
import { cn } from "@/lib/utils";
import {
  LogOut,
  Moon,
  Save,
  Sun,
  User as UserIcon,
  Phone,
  Send,
  Loader2,
  AlertCircle,
  Check,
  ImagePlus,
  Trash2,
  X,
  Gauge,
} from "lucide-react";

// Скорость чтения заметок (раздел «День»): диапазон ползунка в профиле.
const READING_SPEED_MIN = 500;
const READING_SPEED_MAX = 3000;
const READING_SPEED_DEFAULT = 1500;

// Приводит скорость чтения к диапазону ползунка.
// 0 / пусто / нечисловое значение = среднее по умолчанию.
function normalizeReadingSpeed(v) {
  const n = Number(v);
  if (!Number.isFinite(n) || n <= 0) return READING_SPEED_DEFAULT;
  return Math.min(
    READING_SPEED_MAX,
    Math.max(READING_SPEED_MIN, Math.round(n))
  );
}

// «Голый» base64 из photo (data URL или уже без префикса) — сервер хранит
// аватар без data URI префикса.
const avatarBase64 = (photo) =>
  photo ? photo.replace(/^data:[^,]+,/, "") : "";

// src для <img>: готовый data URL оставляем как есть, иначе собираем из
// mime + base64 (защита от уже сохранённых значений с префиксом).
const avatarDataSrc = (photo, mime) =>
  photo
    ? photo.startsWith("data:")
      ? photo
      : `data:${mime || "image/jpeg"};base64,${photo}`
    : "";

// Уменьшает картинку до 256px и возвращает data URL (jpeg), чтобы хранить
// на сервере компактно (base64).
function fileToAvatarDataUrl(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(new Error("Не удалось прочитать файл"));
    reader.onload = () => {
      const img = new Image();
      img.onerror = () => reject(new Error("Файл не является изображением"));
      img.onload = () => {
        const MAX = 256;
        let { width, height } = img;
        if (width > MAX || height > MAX) {
          const k = MAX / Math.max(width, height);
          width = Math.round(width * k);
          height = Math.round(height * k);
        }
        const canvas = document.createElement("canvas");
        canvas.width = width;
        canvas.height = height;
        const ctx = canvas.getContext("2d");
        ctx.fillStyle = "#ffffff"; // подложка для прозрачных PNG
        ctx.fillRect(0, 0, width, height);
        ctx.drawImage(img, 0, 0, width, height);
        resolve(canvas.toDataURL("image/jpeg", 0.9));
      };
      img.src = reader.result;
    };
    reader.readAsDataURL(file);
  });
}

// Модалка смены аватара: превью, готовые варианты, своё фото, сброс.
function AvatarModal({ user, onClose, onSaved }) {
  const [preset, setPreset] = useState(user?.avatar_preset || "");
  const [photo, setPhoto] = useState(user?.avatar_data || "");
  const [photoMime, setPhotoMime] = useState(user?.avatar_mime || "image/jpeg");

  const fileRef = useRef(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const username = user?.username || "";
  const initials = username.slice(0, 2).toUpperCase() || "?";

  const handleFile = async (e) => {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    if (!file.type.startsWith("image/")) {
      setError("Выберите файл изображения");
      return;
    }
    if (file.size > 5 * 1024 * 1024) {
      setError("Фото слишком большое (макс. 5 МБ)");
      return;
    }
    setBusy(true);
    setError("");
    try {
      const dataUrl = await fileToAvatarDataUrl(file);
      setPhoto(dataUrl);
      setPhotoMime("image/jpeg");
      setPreset(""); // своё фото отменяет готовый вариант
    } catch (err) {
      setError(err.message || "Не удалось загрузить фото");
    } finally {
      setBusy(false);
    }
  };

  const handleSave = async () => {
    setBusy(true);
    setError("");
    try {
      await updateAvatar({
        preset,
        photo_data: avatarBase64(photo),
        photo_mime: photo ? photoMime : "",
      });
      onSaved();
      onClose();
    } catch (err) {
      setError(err.message || "Не удалось сохранить аватар");
      setBusy(false);
    }
  };

  // Превью текущего выбора.
  let preview;
  if (photo) {
    preview = (
      <Avatar className="size-16">
        <AvatarImage
          src={avatarDataSrc(photo, photoMime)}
          alt={username}
        />
        <AvatarFallback>{initials}</AvatarFallback>
      </Avatar>
    );
  } else {
    const p = AVATAR_PRESETS.find((x) => x.id === preset);
    preview = (
      <Avatar className="size-16">
        <AvatarFallback className={cn("text-3xl", p?.bg)}>
          {p?.emoji ?? initials}
        </AvatarFallback>
      </Avatar>
    );
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      onClick={onClose}
    >
      <Card
        className="flex max-h-[85vh] w-full max-w-md flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        <CardHeader>
          <CardTitle className="flex items-center gap-3">
            {preview}
            <span className="min-w-0">
              <span className="block truncate">{username}</span>
              <span className="block text-xs font-normal text-muted-foreground">
                Смена аватара
              </span>
            </span>
          </CardTitle>
        </CardHeader>

        <CardContent className="flex-1 space-y-4 overflow-y-auto">
          <div>
            <p className="mb-2 text-xs font-medium text-muted-foreground">
              Готовые варианты
            </p>
            <div className="flex flex-wrap gap-2">
              {AVATAR_PRESETS.map((p) => {
                const active = preset === p.id;
                return (
                  <button
                    key={p.id}
                    type="button"
                    onClick={() => {
                      setPreset(p.id);
                      setPhoto("");
                    }}
                    title={`Аватар «${p.id}»`}
                    className={cn(
                      "flex size-10 cursor-pointer items-center justify-center rounded-full text-xl transition-transform hover:scale-110",
                      p.bg,
                      active &&
                        "ring-2 ring-primary ring-offset-2 ring-offset-background",
                    )}
                  >
                    <span aria-hidden="true">{p.emoji}</span>
                  </button>
                );
              })}
            </div>
          </div>

          <Separator />

          <div className="flex flex-wrap items-center gap-2">
            <input
              ref={fileRef}
              type="file"
              accept="image/*"
              className="hidden"
              onChange={handleFile}
            />
            <Button
              variant="outline"
              onClick={() => fileRef.current?.click()}
              disabled={busy}
            >
              {busy ? <Loader2 className="animate-spin" /> : <ImagePlus />}
              {busy ? "Обрабатываю…" : "Загрузить своё фото"}
            </Button>
            {(preset || photo) && (
              <Button
                variant="ghost"
                onClick={() => {
                  setPreset("");
                  setPhoto("");
                }}
              >
                <Trash2 />
                По умолчанию
              </Button>
            )}
          </div>

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

        <div className="flex justify-end gap-2 border-t p-4">
          <Button variant="ghost" onClick={onClose} disabled={busy}>
            <X />
            Отмена
          </Button>
          <Button onClick={handleSave} disabled={busy}>
            {busy ? <Loader2 className="animate-spin" /> : <Save />}
            {busy ? "Сохраняю…" : "Сохранить"}
          </Button>
        </div>
      </Card>
    </div>
  );
}

// Страница «Профиль»: аватар (смена в модалке), контактные данные,
// переключатель темы и выход из аккаунта.
function User({ user, onLogout }) {
  const queryClient = useQueryClient();
  const theme = useAppStore((s) => s.theme);
  const toggleTheme = useAppStore((s) => s.toggleTheme);

  const username = user?.username || "";
  const isAdmin = Boolean(user?.is_admin);

  const [avatarOpen, setAvatarOpen] = useState(false);
  const [notifCreateOpen, setNotifCreateOpen] = useState(false);

  // Привязка Telegram.
  const [tgBusy, setTgBusy] = useState(false);
  const [tgError, setTgError] = useState("");
  const [tgStep, setTgStep] = useState(false);

  // Необязательные контакты (черновик формы).
  const [phone, setPhone] = useState(user?.phone || "");
  const [telegram, setTelegram] = useState(user?.telegram || "");
  // Скорость чтения: значение ползунка (500–5000).
  const [readingSpeed, setReadingSpeed] = useState(
    normalizeReadingSpeed(user?.reading_speed)
  );
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState("");

  // При обновлении данных пользователя (после сохранения/входа) — синхронизируем.
  useEffect(() => {
    setPhone(user?.phone || "");
    setTelegram(user?.telegram || "");
    setReadingSpeed(normalizeReadingSpeed(user?.reading_speed));
  }, [user]);

  const refreshMe = () =>
    queryClient.invalidateQueries({ queryKey: ["me"] });

  const handleSaveContacts = async () => {
    setSaving(true);
    setError("");
    setSaved(false);
    try {
      await updateMe({
        phone: phone.trim(),
        telegram: telegram.trim().replace(/^@/, ""),
        reading_speed: normalizeReadingSpeed(readingSpeed),
      });
      await refreshMe();
      setSaved(true);
    } catch (err) {
      setError(err.message || "Не удалось сохранить профиль");
    } finally {
      setSaving(false);
    }
  };

  const handleLinkTg = async () => {
    setTgBusy(true);
    setTgError("");
    try {
      const { url } = await linkTelegram();
      setTgStep(true);
      window.open(url, "_blank", "noopener");
    } catch (err) {
      setTgError(err.message || "Не удалось создать ссылку привязки");
    } finally {
      setTgBusy(false);
    }
  };

  const handleUnlinkTg = async () => {
    setTgBusy(true);
    setTgError("");
    try {
      await unlinkTelegram();
      setTgStep(false);
      await refreshMe();
    } catch (err) {
      setTgError(err.message || "Не удалось отключить Telegram");
    } finally {
      setTgBusy(false);
    }
  };

  return (
    <section>
      <h2 className="flex items-center gap-2 text-xl font-semibold">
        <UserIcon className="size-5 text-muted-foreground" />
        Профиль
      </h2>

      <Card className="my-3" size="sm">
        <CardHeader>
          <CardTitle className="flex w-full items-center gap-3">

            <button
              type="button"
              onClick={() => setAvatarOpen(true)}
              aria-label="Сменить аватар"
              title="Сменить аватар"
              className="cursor-pointer rounded-full transition-transform hover:scale-105 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
            <UserAvatar
              username={username}
              className="size-12 shrink-0"
              presetId={user?.avatar_preset}
              photoData={user?.avatar_data}
              photoMime={user?.avatar_mime}
            />
            </button>
            <span className="min-w-0 truncate">{username}</span>
            <Badge variant={isAdmin ? "default" : "outline"}>
              {isAdmin ? "Администратор" : "Пользователь"}
            </Badge>
            <Button
              variant="outline"
              size="icon"
              onClick={toggleTheme}
              className="ml-auto shrink-0"
              aria-label={
                theme === "dark" ? "Включить светлую тему" : "Включить тёмную тему"
              }
            >
              {theme === "dark" ? <Sun /> : <Moon />}
            </Button>
          </CardTitle>
        </CardHeader>

        <CardContent className="space-y-4">

          <Separator />

          {/* Контакты */}
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label htmlFor="profile-phone">Телефон</Label>
              <div className="relative">
                <Phone className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  id="profile-phone"
                  value={phone}
                  onChange={(e) => setPhone(e.target.value)}
                  placeholder="+7 900 000-00-00"
                  inputMode="tel"
                  autoComplete="tel"
                  className="pl-9"
                />
              </div>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="profile-telegram">Telegram</Label>
              <div className="relative">
                <Send className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
                <span className="pointer-events-none absolute top-1/2 left-9 -translate-y-1/2 text-muted-foreground">
                  @
                </span>
                <Input
                  id="profile-telegram"
                  value={telegram}
                  onChange={(e) => setTelegram(e.target.value)}
                  placeholder="username"
                  autoComplete="off"
                  className="pl-12"
                />
              </div>
            </div>
          </div>

          {/* Скорость чтения (для раздела «День») */}
          <div className="space-y-1.5">
            <div className="flex items-center justify-between gap-3">
              <Label>Чтение символов в минуту</Label>
              <span className="text-sm font-semibold tabular-nums">
                {readingSpeed} симв/мин
              </span>
            </div>
            <div className="flex items-center gap-3">
              <Gauge className="size-4 shrink-0 text-muted-foreground" />
              <Slider
                min={READING_SPEED_MIN}
                max={READING_SPEED_MAX}
                step={50}
                value={[readingSpeed]}
                onValueChange={(vals) => setReadingSpeed(vals[0])}
                aria-label="Скорость чтения символов в минуту"
                className="flex-1"
              />
            </div>
            <p className="text-xs text-muted-foreground">
              Среднее значение — {READING_SPEED_DEFAULT} символов в минуту.
              Влияет на расчёт времени повторения заметок в разделе «День».
            </p>
          </div>

          {/* Привязка Telegram для уведомлений */}
          <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border p-3">
            <div className="min-w-0 space-y-0.5">
              <p className="flex items-center gap-1.5 text-sm font-medium">
                <Send className="size-4 text-muted-foreground" />
                Telegram для уведомлений
              </p>
              <p className="text-xs text-muted-foreground">
                {user?.telegram_linked
                  ? `Подключено${user?.telegram ? ` (@${user.telegram})` : ""} — сюда придут уведомления`
                  : "Не подключено — нажмите кнопку и запустите бота"}
              </p>
              {tgError && (
                <p
                  className="flex items-center gap-1 text-xs text-destructive"
                  role="alert"
                >
                  <AlertCircle className="size-3.5" />
                  {tgError}
                </p>
              )}
            </div>
            <div className="flex flex-wrap items-center gap-2">
              {user?.telegram_linked ? (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleUnlinkTg}
                  disabled={tgBusy}
                >
                  <X />
                  Отключить
                </Button>
              ) : (
                <>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={handleLinkTg}
                    disabled={tgBusy}
                  >
                    <Send />
                    {tgBusy ? "Создаю…" : "Подключить"}
                  </Button>
                  {tgStep && (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={refreshMe}
                      disabled={tgBusy}
                    >
                      <Check />
                      Проверить подключение
                    </Button>
                  )}
                </>
              )}
            </div>
          </div>

          {error && (
            <p
              className="flex items-center gap-1.5 text-sm text-destructive"
              role="alert"
            >
              <AlertCircle className="size-4" />
              {error}
            </p>
          )}

          <div className="flex items-center gap-3">
            <Button onClick={handleSaveContacts} disabled={saving}>
              {saving ? <Loader2 className="animate-spin" /> : <Save />}
              {saving ? "Сохраняю…" : "Сохранить"}
            </Button>
            {saved && (
              <span className="flex items-center gap-1 text-sm text-emerald-600 dark:text-emerald-400">
                <Check className="size-4" />
                Сохранено
              </span>
            )}
          </div>

          <Separator />

          {/* Выход */}
          <Button variant="destructive" className="w-full" onClick={onLogout}>
            <LogOut />
            Выйти
          </Button>
        </CardContent>
      </Card>

      {avatarOpen && (
        <AvatarModal
          user={user}
          onClose={() => setAvatarOpen(false)}
          onSaved={refreshMe}
        />
      )}

      <NotificationsCard onAdd={() => setNotifCreateOpen(true)} />

      {notifCreateOpen && (
        <CreateNotificationModal
          onClose={() => setNotifCreateOpen(false)}
          onCreated={() =>
            queryClient.invalidateQueries({ queryKey: ["notifications"] })
          }
        />
      )}
    </section>
  );
}

export default User;
