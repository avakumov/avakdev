import {
  Avatar,
  AvatarFallback,
  AvatarImage,
} from "@/components/ui/avatar";
import { presetById } from "@/lib/avatars.js";
import { cn } from "@/lib/utils";

// Аватар пользователя: своё фото (photoData+photoMime), готовый вариант
// (presetId) или инициалы. Размер задаётся через className ("size-8"/"size-12").
function UserAvatar({
  username,
  className,
  presetId = "",
  photoData = "",
  photoMime = "",
}) {
  const initials = (username || "?").slice(0, 2).toUpperCase();

  // Своё фото.
  if (photoData) {
    return (
      <Avatar className={className}>
        <AvatarImage
          src={`data:${photoMime || "image/jpeg"};base64,${photoData}`}
          alt={username}
        />
        <AvatarFallback className="text-sm">{initials}</AvatarFallback>
      </Avatar>
    );
  }

  // Готовый вариант (эмодзи на цветном фоне).
  if (presetId) {
    const preset = presetById(presetId);
    return (
      <Avatar className={className}>
        <AvatarFallback
          className={cn("text-xl leading-none", preset?.bg)}
        >
          {preset?.emoji ?? initials}
        </AvatarFallback>
      </Avatar>
    );
  }

  // По умолчанию — инициалы.
  return (
    <Avatar className={className}>
      <AvatarImage alt={username} />
      <AvatarFallback className="text-sm">{initials}</AvatarFallback>
    </Avatar>
  );
}

export default UserAvatar;
