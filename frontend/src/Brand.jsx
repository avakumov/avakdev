import { Activity } from "lucide-react";
import { cn } from "@/lib/utils";

// Логотип «avakumov»: значок Activity и название в одной рамке —
// жёлтая заливка, чёрная обводка, чёрные иконка и текст.
// Клик ведёт на главную («Отчеты»): по умолчанию переход по href="/",
// при переданном onClick — SPA-переход без перезагрузки страницы.
function Brand({ onClick, className }) {
  return (
    <a
      href="/"
      onClick={(e) => {
        if (onClick) {
          e.preventDefault();
          onClick();
        }
      }}
      className={cn(
        "flex w-fit cursor-pointer items-center gap-2 rounded-lg border-2 border-black bg-yellow-500 px-2 py-0.5 transition-opacity hover:opacity-80",
        className,
      )}
    >
      <Activity className="size-4" />
      <span className="font-heading font-semibold tracking-tight text-black">
        avakumov
      </span>
    </a>
  );
}

export default Brand;
