import { cn } from "@/lib/cn";

interface SpinnerProps {
  size?: number;
  className?: string;
  /** Accessible label — hidden by default since buttons show their own text. */
  label?: string;
}

export function Spinner({ size = 16, className, label = "Loading" }: SpinnerProps) {
  return (
    <svg
      className={cn("animate-spin", className)}
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      role="status"
      aria-label={label}
    >
      <circle className="opacity-20" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="3" />
      <path
        className="opacity-90"
        d="M22 12a10 10 0 0 0-10-10"
        stroke="currentColor"
        strokeWidth="3"
        strokeLinecap="round"
      />
    </svg>
  );
}
