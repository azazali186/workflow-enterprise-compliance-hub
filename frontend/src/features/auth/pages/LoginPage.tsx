import { motion } from "framer-motion";
import { CheckCircle2, Eye, EyeOff, KeyRound, ShieldCheck, UserRound } from "lucide-react";
import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { Button } from "@/components/ui/Button";
import { Field } from "@/components/ui/Field";
import { Input } from "@/components/ui/Input";
import { authApi } from "@/services/auth.service";
import { setToken } from "@/services/api/client";
import { useAppDispatch } from "@/store/hooks";
import { authActions } from "@/store/auth.slice";
import { ApiError } from "@/types/api";

const FEATURES = [
  "Route-level permissions enforced on every endpoint",
  "Server-side cursor pagination on every list",
  "Full audit trail with before/after snapshots",
];

export function LoginPage() {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const dispatch = useAppDispatch();
  const navigate = useNavigate();

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!username.trim() || !password) {
      setError("Enter both your username and password.");
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const res = await authApi.login(username.trim(), password);
      setToken(res.access_token);
      dispatch(authActions.signedIn({ token: res.access_token, user: res.user }));
      try {
        const me = await authApi.me();
        dispatch(authActions.sessionLoaded(me));
      } catch {
        /* the route guard retries /me on the next render */
      }
      navigate("/app", { replace: true });
    } catch (err) {
      setError(ApiError.is(err) ? err.userMessage : "Sign-in failed — please try again.");
      setLoading(false);
    }
  };

  return (
    <div className="grid min-h-dvh lg:grid-cols-2">
      {/* Brand panel */}
      <div className="relative hidden flex-col justify-between overflow-hidden bg-ink-950 p-10 text-slate-300 lg:flex">
        <div
          className="pointer-events-none absolute -left-40 -top-40 h-96 w-96 rounded-full bg-primary-600/25 blur-3xl"
          aria-hidden="true"
        />
        <div
          className="pointer-events-none absolute -bottom-48 -right-24 h-96 w-96 rounded-full bg-primary-500/15 blur-3xl"
          aria-hidden="true"
        />
        <div className="relative flex items-center gap-3">
          <span className="brand-mark flex h-9 w-9 items-center justify-center rounded-xl" aria-hidden="true">
            <ShieldCheck className="h-5 w-5 text-white" />
          </span>
          <div>
            <p className="text-base font-bold tracking-tight text-white">ComplianceHub</p>
            <p className="text-xs text-slate-400">Governance console</p>
          </div>
        </div>

        <div className="relative max-w-md">
          <h1 className="text-3xl font-bold leading-tight tracking-tight text-white">
            Compliance, reviewed with total clarity.
          </h1>
          <ul className="mt-8 space-y-3">
            {FEATURES.map((f) => (
              <li key={f} className="flex items-start gap-2.5 text-sm text-slate-300">
                <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-primary-400" aria-hidden="true" />
                {f}
              </li>
            ))}
          </ul>
        </div>

        <p className="relative text-xs text-slate-500">© 2026 ComplianceHub. Internal tool.</p>
      </div>

      {/* Form panel */}
      <div className="flex items-center justify-center bg-canvas px-4 py-12 sm:px-8">
        <motion.div
          initial={{ opacity: 0, y: 12 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.3, ease: [0.22, 1, 0.36, 1] }}
          className="w-full max-w-sm"
        >
          <div className="mb-8 flex items-center gap-3 lg:hidden">
            <span className="brand-mark flex h-9 w-9 items-center justify-center rounded-xl" aria-hidden="true">
              <ShieldCheck className="h-5 w-5 text-white" />
            </span>
            <p className="text-base font-bold tracking-tight text-slate-900">ComplianceHub</p>
          </div>

          <h2 className="text-2xl font-bold tracking-tight text-slate-900">Welcome back</h2>
          <p className="mt-1.5 text-sm text-slate-500">Sign in to the governance console.</p>

          <form onSubmit={submit} className="mt-8 space-y-4" noValidate>
            {error && (
              <div
                className="rounded-lg border border-danger-200 bg-danger-50 px-3.5 py-2.5 text-sm text-danger-700"
                role="alert"
              >
                {error}
              </div>
            )}

            <Field label="Username" htmlFor="login-username" required>
              <div className="relative">
                <UserRound
                  className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400"
                  aria-hidden="true"
                />
                <Input
                  id="login-username"
                  autoComplete="username"
                  autoFocus
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  placeholder="admin"
                  className="pl-9"
                />
              </div>
            </Field>

            <Field label="Password" htmlFor="login-password" required>
              <div className="relative">
                <KeyRound
                  className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400"
                  aria-hidden="true"
                />
                <Input
                  id="login-password"
                  type={showPassword ? "text" : "password"}
                  autoComplete="current-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="••••••••"
                  className="pl-9 pr-10"
                />
                <button
                  type="button"
                  onClick={() => setShowPassword((s) => !s)}
                  className="absolute right-2 top-1/2 -translate-y-1/2 rounded-md p-1.5 text-slate-400 transition-colors hover:text-slate-600"
                  aria-label={showPassword ? "Hide password" : "Show password"}
                >
                  {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
            </Field>

            <Button type="submit" size="lg" loading={loading} className="w-full">
              Sign in
            </Button>
          </form>
        </motion.div>
      </div>
    </div>
  );
}
