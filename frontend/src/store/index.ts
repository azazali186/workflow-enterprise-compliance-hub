import { configureStore } from "@reduxjs/toolkit";
import authReducer from "./auth.slice";
import toastReducer, { setDispatch } from "./toast.slice";

export const store = configureStore({
  reducer: {
    auth: authReducer,
    toast: toastReducer,
  },
});

// Bridge for the imperative toast helper (services, non-component code).
setDispatch(store.dispatch);

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
