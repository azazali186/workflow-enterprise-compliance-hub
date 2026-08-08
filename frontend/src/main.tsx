import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import { onUnauthorized, setToken } from "@/services/api/client";
import { authActions } from "@/store/auth.slice";
import { store } from "@/store";
import "@/styles/index.css";

// Restore the persisted token into the API client before the first request.
const stored = (() => {
  try {
    return localStorage.getItem("ch.token");
  } catch {
    return null;
  }
})();
setToken(stored);

// Any authenticated request returning 401 clears the session once; the route
// guard then redirects to /login.
onUnauthorized(() => {
  setToken(null);
  store.dispatch(authActions.sessionFailed());
});

const rootEl = document.getElementById("root");
if (!rootEl) throw new Error("Missing #root element");

createRoot(rootEl).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
