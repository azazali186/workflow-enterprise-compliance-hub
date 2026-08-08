import { createSlice, type PayloadAction } from "@reduxjs/toolkit";

export type ToastKind = "success" | "error" | "info" | "warning";

export interface Toast {
  id: number;
  kind: ToastKind;
  title: string;
  description?: string;
}

interface ToastState {
  toasts: Toast[];
}

const initialState: ToastState = { toasts: [] };

let nextId = 1;

const toastSlice = createSlice({
  name: "toast",
  initialState,
  reducers: {
    pushed(state, action: PayloadAction<Omit<Toast, "id">>) {
      state.toasts.push({ ...action.payload, id: nextId++ });
      if (state.toasts.length > 5) state.toasts.shift();
    },
    dismissed(state, action: PayloadAction<number>) {
      state.toasts = state.toasts.filter((t) => t.id !== action.payload);
    },
  },
});

export const toastActions = toastSlice.actions;
export default toastSlice.reducer;

/* --- dispatch bridge (avoids importing the store into reducers) --- */
let dispatchRef: ((action: unknown) => void) | null = null;

export function setDispatch(dispatch: (action: unknown) => void): void {
  dispatchRef = dispatch;
}

/** Imperative toast helper usable from services, hooks, and components. */
export function notify(kind: ToastKind, title: string, description?: string): void {
  dispatchRef?.(toastActions.pushed({ kind, title, description }));
}

export const toast = {
  success: (title: string, description?: string) => notify("success", title, description),
  error: (title: string, description?: string) => notify("error", title, description),
  info: (title: string, description?: string) => notify("info", title, description),
  warning: (title: string, description?: string) => notify("warning", title, description),
};
