import { Activity } from "lucide-react";
import { cn } from "@/lib/utils";

// Логотип «avakumov»: слева чёрная плашка с жёлтым значком, справа название
// на жёлтой заливке, между ними — плавный градиент чёрный → жёлтый.
// Высота фиксированная (h-7), дети растягиваются через items-stretch —
// никаких процентных высот, которые схлопываются в некоторых браузерах.
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
        "flex h-7 w-fit cursor-pointer items-stretch overflow-hidden rounded-lg border-2 border-black text-black transition-opacity hover:opacity-80 dark:border-white",
        className,
      )}
    >
      {/* Чёрная плашка с жёлтым значком */}
      <span className="flex items-center bg-black pl-2 pr-0.5">
        <Activity className="size-4 text-yellow-500" />
      </span>

      {/* Резкая граница обратным слешем под 30° от вертикали: направление
          градиента 60deg даёт границу, перпендикулярную ему (30° от вертикали),
          а жёсткий стоп 49.5%/50.5% — чёткую линию без плавного перехода.
          bg-yellow-500 — фолбэк. */}
      <span className="w-3 bg-yellow-500 bg-[linear-gradient(70deg,#000_46%,#eab308_54%)]" />

      {/* Название на жёлтой заливке */}
      <span className="flex items-center bg-yellow-500 pl-0.5 pr-2">
        <span className="font-heading font-semibold tracking-tight">
          avakumov
        </span>
      </span>
    </a>
  );
}

export default Brand;
