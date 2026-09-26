import { X } from "lucide-react";
import { cn } from "@/lib/utils";

// Крестик закрытия в правом верхнем углу модалки.
// Панель модалки должна быть `relative`, чтобы крестик встал по её углу.
function ModalClose({ onClose, className, ...props }) {
  return (
    <button
      type="button"
      onClick={onClose}
      aria-label="Закрыть"
      title="Закрыть"
      className={cn(
        "absolute top-2 right-2 z-10 flex size-8 cursor-pointer items-center justify-center text-muted-foreground transition-colors hover:text-foreground",
        className,
      )}
      {...props}
    >
      <X className="size-4" />
    </button>
  );
}

export default ModalClose;
