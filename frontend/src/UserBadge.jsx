import { ChevronRight, ShieldCheck } from "lucide-react";
import UserAvatar from "@/components/UserAvatar.jsx";

// Аватар с именем пользователя. Клик ведёт на страницу профиля,
// где можно выйти, переключить тему и сменить аватар.
function UserBadge({ user, onClick }) {
  const username = user?.username || "";
  const isAdmin = Boolean(user?.is_admin);

  return (
    <button
      type="button"
      onClick={onClick}
      aria-label="Открыть страницу пользователя"
      className="flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
    >
      <UserAvatar
        username={username}
        className="size-8 shrink-0"
        presetId={user?.avatar_preset}
        photoData={user?.avatar_data}
        photoMime={user?.avatar_mime}
      />
      <span className="hidden min-w-0 flex-1 truncate text-sm font-medium sm:inline">
        {username}
      </span>
      {isAdmin && (
        <ShieldCheck
          className="hidden size-4 shrink-0 text-muted-foreground sm:inline"
          aria-label="Администратор"
        />
      )}
      <ChevronRight className="hidden size-4 shrink-0 text-muted-foreground sm:inline" />
    </button>
  );
}

export default UserBadge;
