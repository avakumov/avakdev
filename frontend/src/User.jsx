import { useEffect, useRef, useState } from "react";
import { updateMe, updateAvatar } from "./api.js";
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
} from "lucide-react";

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
        photo_data: photo,
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
          src={`data:${photoMime || "image/jpeg"};base64,${photo}`}
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

  // Необязательные контакты (черновик формы).
  const [phone, setPhone] = useState(user?.phone || "");
  const [telegram, setTelegram] = useState(user?.telegram || "");
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState("");

  // При обновлении данных пользователя (после сохранения/входа) — синхронизируем.
  useEffect(() => {
    setPhone(user?.phone || "");
    setTelegram(user?.telegram || "");
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
      });
      await refreshMe();
      setSaved(true);
    } catch (err) {
      setError(err.message || "Не удалось сохранить профиль");
    } finally {
      setSaving(false);
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
              {saving ? "Сохраняю…" : "Сохранить контакты"}
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
