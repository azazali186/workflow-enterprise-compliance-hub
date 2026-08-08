import { AlertTriangle } from "lucide-react";
import { ApiError } from "@/types/api";
import { Button } from "./Button";
import { cn } from "@/lib/cn";

export interface ErrorStateProps {
  title?: string;
  error?: unknown;
  onRetry?: () => void;
  className?: string;
}

export function ErrorState({ title = "Something went wrong", error, onRetry, className }: ErrorStateProps) {
  const message =
    ApiError.is(error) && error.status !== 0
      ? error.userMessage
      : "We couldn't load this data. Please try again.";

  return (
    <div className={cn("flex flex-col items-center justify-center px-6 py-12 text-center", className)} role="alert">
      <div className="flex h-11 w-11 items-center justify-center rounded-full bg-danger-50 text-danger-500">
        <AlertTriangle className="h-5 w-5" aria-hidden="true" />
      </div>
      <h3 className="mt-3 text-sm font-semibold text-slate-900">{title}</h3>
      <p className="mt-1 max-w-sm text-sm text-slate-500">{message}</p>
      {onRetry && (
        <Button variant="secondary" size="sm" className="mt-4" onClick={onRetry}>
          Try again
        </Button>
      )}
    </div>
  );
}
