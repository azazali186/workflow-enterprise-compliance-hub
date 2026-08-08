import { motion } from "framer-motion";
import type { ButtonHTMLAttributes, ReactNode } from "react";
import { cn } from "@/lib/cn";
import { Spinner } from "./Spinner";

type Variant = "primary" | "secondary" | "ghost" | "danger" | "outline-danger";
type Size = "sm" | "md" | "lg";

// Omit the gesture handlers framer-motion re-types so the native props spread
// cleanly onto motion.button.
type NativeButtonProps = Omit<
  ButtonHTMLAttributes<HTMLButtonElement>,
  "onAnimationStart" | "onAnimationEnd" | "onAnimationIteration" | "onDrag" | "onDragStart" | "onDragEnd" | "onTransitionEnd"
>;

export interface ButtonProps extends NativeButtonProps {
  variant?: Variant;
  size?: Size;
  loading?: boolean;
  icon?: ReactNode;
  children?: ReactNode;
}

const variantClasses: Record<Variant, string> = {
  primary:
    "bg-primary-600 text-white shadow-sm hover:bg-primary-700 active:bg-primary-800 disabled:hover:bg-primary-600",
  secondary:
    "border border-slate-200 bg-white text-slate-700 shadow-sm hover:bg-slate-50 hover:text-slate-900 active:bg-slate-100",
  ghost: "text-slate-600 hover:bg-slate-100 hover:text-slate-900 active:bg-slate-200",
  danger: "bg-danger-600 text-white shadow-sm hover:bg-danger-700 active:bg-danger-800 disabled:hover:bg-danger-600",
  "outline-danger":
    "border border-danger-200 bg-white text-danger-600 shadow-sm hover:bg-danger-50 active:bg-danger-100",
};

const sizeClasses: Record<Size, string> = {
  sm: "h-8 gap-1.5 px-3 text-xs",
  md: "h-9.5 gap-2 px-4 text-sm",
  lg: "h-11 gap-2 px-5 text-sm",
};

export function Button({
  variant = "primary",
  size = "md",
  loading = false,
  icon,
  className,
  children,
  disabled,
  type = "button",
  ...rest
}: ButtonProps) {
  const isDisabled = disabled || loading;
  return (
    <motion.button
      type={type}
      whileTap={isDisabled ? undefined : { scale: 0.98 }}
      className={cn(
        "inline-flex select-none items-center justify-center rounded-lg font-medium transition-colors duration-150",
        "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-500",
        "disabled:pointer-events-none disabled:opacity-50",
        variantClasses[variant],
        sizeClasses[size],
        className,
      )}
      disabled={isDisabled}
      aria-busy={loading || undefined}
      {...rest}
    >
      {loading ? <Spinner size={size === "sm" ? 14 : 16} className="shrink-0" /> : icon}
      {children}
    </motion.button>
  );
}
