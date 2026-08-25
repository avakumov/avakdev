import {
  Avatar,
  AvatarFallback,
  AvatarImage,
} from "@/components/ui/avatar";
import { ChevronRight, ShieldCheck } from "lucide-react";

// Аватар с именем пользователя. Клик ведёт на страницу «Пользователь»,
// где можно выйти и переключить тему оформления.
function UserBadge({ username, isAdmin = false, onClick }) {
  const initials = (username || "?").slice(0, 2).toUpperCase();

  return (
    <button
      type="button"
      onClick={onClick}
      aria-label="Открыть страницу пользователя"
      className="flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
    >
      <Avatar className="size-8 shrink-0">
        <AvatarImage alt={username} />
        <AvatarFallback className="text-xs font-semibold">
          {initials}
        </AvatarFallback>
      </Avatar>
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
