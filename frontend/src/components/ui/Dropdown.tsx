import { AnimatePresence, motion } from "framer-motion";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { cn } from "@/lib/cn";

export interface DropdownItem {
  key: string;
  label: string;
  icon?: ReactNode;
  danger?: boolean;
  disabled?: boolean;
  onSelect?: () => void;
}

export interface DropdownProps {
  /** Renders the trigger (button semantics expected). */
  trigger: (props: { open: boolean; toggle: () => void }) => ReactNode;
  items: DropdownItem[];
  align?: "left" | "right";
  className?: string;
}

export function Dropdown({ trigger, items, align = "right", className }: DropdownProps) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onPointerDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onPointerDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return (
    <div ref={rootRef} className={cn("relative", className)}>
      {trigger({ open, toggle: () => setOpen((o) => !o) })}
      <AnimatePresence>
        {open && (
          <motion.ul
            className={cn(
              "absolute z-30 mt-1.5 min-w-44 overflow-hidden rounded-lg border border-slate-200 bg-white py-1 shadow-pop",
              align === "right" ? "right-0" : "left-0",
            )}
            initial={{ opacity: 0, y: -4, scale: 0.98 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: -4, scale: 0.98 }}
            transition={{ duration: 0.12 }}
            role="menu"
          >
            {items.map((item) => (
              <li key={item.key}>
                <button
                  type="button"
                  role="menuitem"
                  disabled={item.disabled}
                  className={cn(
                    "flex w-full items-center gap-2.5 px-3 py-2 text-left text-sm transition-colors",
                    "disabled:pointer-events-none disabled:opacity-50",
                    item.danger
                      ? "text-danger-600 hover:bg-danger-50"
                      : "text-slate-700 hover:bg-slate-50 hover:text-slate-900",
                  )}
                  onClick={() => {
                    setOpen(false);
                    item.onSelect?.();
                  }}
                >
                  {item.icon}
                  {item.label}
                </button>
              </li>
            ))}
          </motion.ul>
        )}
      </AnimatePresence>
    </div>
  );
}
