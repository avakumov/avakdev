import {
  Avatar,
  AvatarFallback,
  AvatarImage,
} from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { LogOut } from "lucide-react";

// Отображает аватар вошедшего пользователя, его имя и кнопку выхода.
// onLogout вызывается после успешного выхода.
function UserBadge({ username, isAdmin = false, onLogout }) {
  const initials = (username || "?").slice(0, 2).toUpperCase();

  return (
    <div className="flex items-center gap-2">
      <Tooltip>
        <TooltipTrigger asChild>
          <div className="flex items-center gap-2">
            <Avatar className="size-8">
              <AvatarImage alt={username} />
              <AvatarFallback className="text-xs font-semibold">
                {initials}
              </AvatarFallback>
            </Avatar>
            <span className="hidden text-sm font-medium sm:inline">
              {username}
            </span>
          </div>
        </TooltipTrigger>
        <TooltipContent>
          {username}
          {isAdmin ? " · администратор" : " · пользователь"}
        </TooltipContent>
      </Tooltip>

      <Button
        variant="outline"
        size="icon"
        onClick={onLogout}
        aria-label="Выйти"
      >
        <LogOut />
      </Button>
    </div>
  );
}

export default UserBadge;
