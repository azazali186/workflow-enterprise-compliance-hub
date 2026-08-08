import { AnimatePresence, motion } from "framer-motion";
import { CheckCircle2, Info, TriangleAlert, X, XCircle } from "lucide-react";
import { useEffect } from "react";
import { useAppDispatch, useAppSelector } from "@/store/hooks";
import { toastActions, type ToastKind } from "@/store/toast.slice";
import { cn } from "@/lib/cn";

const kindMeta: Record<ToastKind, { icon: typeof Info; iconClass: string; barClass: string }> = {
  success: { icon: CheckCircle2, iconClass: "text-success-500", barClass: "bg-success-500" },
  error: { icon: XCircle, iconClass: "text-danger-500", barClass: "bg-danger-500" },
  warning: { icon: TriangleAlert, iconClass: "text-warning-500", barClass: "bg-warning-500" },
  info: { icon: Info, iconClass: "text-info-500", barClass: "bg-info-500" },
};

const AUTO_DISMISS_MS = 4000;

export function Toaster() {
  const toasts = useAppSelector((s) => s.toast.toasts);
  const dispatch = useAppDispatch();

  return (
    <div
      className="pointer-events-none fixed bottom-4 right-4 z-[70] flex w-full max-w-sm flex-col gap-2"
      aria-live="polite"
    >
      <AnimatePresence initial={false}>
        {toasts.map((t) => (
          <ToastItem key={t.id} id={t.id} kind={t.kind} title={t.title} description={t.description} onDismiss={() => dispatch(toastActions.dismissed(t.id))} />
        ))}
      </AnimatePresence>
    </div>
  );
}

interface ToastItemProps {
  id: number;
  kind: ToastKind;
  title: string;
  description?: string;
  onDismiss: () => void;
}

function ToastItem({ id, kind, title, description, onDismiss }: ToastItemProps) {
  useEffect(() => {
    const t = window.setTimeout(onDismiss, AUTO_DISMISS_MS);
    return () => window.clearTimeout(t);
  }, [id, onDismiss]);

  const meta = kindMeta[kind];
  const Icon = meta.icon;

  return (
    <motion.div
      layout
      initial={{ opacity: 0, x: 24, scale: 0.97 }}
      animate={{ opacity: 1, x: 0, scale: 1 }}
      exit={{ opacity: 0, x: 24, scale: 0.97 }}
      transition={{ type: "spring", stiffness: 420, damping: 32 }}
      className="pointer-events-auto relative flex items-start gap-3 overflow-hidden rounded-xl border border-slate-200 bg-white p-3.5 pr-9 shadow-pop"
      role="status"
    >
      <span className={cn("absolute inset-y-0 left-0 w-0.5", meta.barClass)} aria-hidden="true" />
      <Icon className={cn("mt-0.5 h-4.5 w-4.5 shrink-0", meta.iconClass)} aria-hidden="true" />
      <div className="min-w-0">
        <p className="text-sm font-semibold text-slate-900">{title}</p>
        {description && <p className="mt-0.5 text-xs leading-relaxed text-slate-500">{description}</p>}
      </div>
      <button
        type="button"
        onClick={onDismiss}
        className="absolute right-2.5 top-2.5 rounded-md p-1 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-600"
        aria-label="Dismiss notification"
      >
        <X className="h-3.5 w-3.5" />
      </button>
    </motion.div>
  );
}
