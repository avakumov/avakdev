import { useAppStore } from "./store.js";
import {
  Avatar,
  AvatarFallback,
  AvatarImage,
} from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardDescription,
} from "@/components/ui/card";
import { LogOut, Moon, Sun, User as UserIcon } from "lucide-react";

// Страница «Пользователь»: профиль, переключатель темы (светлая/тёмная)
// и выход из аккаунта.
function User({ username, isAdmin = false, onLogout }) {
  const theme = useAppStore((s) => s.theme);
  const toggleTheme = useAppStore((s) => s.toggleTheme);
  const initials = (username || "?").slice(0, 2).toUpperCase();

  return (
    <section>
      <h2 className="flex items-center gap-2 text-xl font-semibold">
        <UserIcon className="size-5 text-muted-foreground" />
        Пользователь
      </h2>

      <Card className="my-3" size="sm">
        <CardHeader>
          <CardTitle className="flex items-center gap-3">
            <Avatar className="size-12">
              <AvatarImage alt={username} />
              <AvatarFallback className="text-base font-semibold">
                {initials}
              </AvatarFallback>
            </Avatar>
            <span className="min-w-0 truncate">{username}</span>
            <Badge variant={isAdmin ? "default" : "outline"}>
              {isAdmin ? "Администратор" : "Пользователь"}
            </Badge>
          </CardTitle>
          <CardDescription>
            Настройки аккаунта и оформления
          </CardDescription>
        </CardHeader>

        <CardContent className="space-y-4">
          {/* Переключатель темы */}
          <div className="flex items-center justify-between gap-3 rounded-lg border p-3">
            <div className="min-w-0">
              <p className="text-sm font-medium">Тема оформления</p>
              <p className="text-xs text-muted-foreground">
                {theme === "dark" ? "Тёмная" : "Светлая"}
              </p>
            </div>
            <Button
              variant="outline"
              size="icon"
              onClick={toggleTheme}
              aria-label={
                theme === "dark" ? "Включить светлую тему" : "Включить тёмную тему"
              }
            >
              {theme === "dark" ? <Sun /> : <Moon />}
            </Button>
          </div>

          {/* Выход */}
          <Button variant="destructive" className="w-full" onClick={onLogout}>
            <LogOut />
            Выйти
          </Button>
        </CardContent>
      </Card>
    </section>
  );
}

export default User;
