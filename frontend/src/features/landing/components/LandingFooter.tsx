import { ShieldCheck } from "lucide-react";
import { Link } from "react-router-dom";

const COLUMNS = [
  {
    title: "Product",
    links: [
      { label: "Capabilities", href: "#features" },
      { label: "How it works", href: "#how" },
      { label: "Security", href: "#security" },
    ],
  },
  {
    title: "Console",
    links: [
      { label: "Dashboard", to: "/app" },
      { label: "Analytics", to: "/app/analytics" },
      { label: "Audit log", to: "/app/audit-logs" },
      { label: "Sign in", to: "/login" },
    ],
  },
];

export function LandingFooter() {
  return (
    <footer className="border-t border-white/5 bg-ink-950">
      <div className="mx-auto max-w-7xl px-4 py-14 sm:px-6 lg:px-8">
        <div className="grid grid-cols-1 gap-10 sm:grid-cols-3">
          <div>
            <Link to="/" className="flex items-center gap-2.5" aria-label="ComplianceHub home">
              <span className="brand-mark flex h-8 w-8 items-center justify-center rounded-lg" aria-hidden="true">
                <ShieldCheck className="h-4.5 w-4.5 text-white" />
              </span>
              <span className="font-display text-base font-semibold tracking-tight text-white">ComplianceHub</span>
            </Link>
            <p className="mt-4 max-w-xs text-sm leading-relaxed text-slate-400">
              Continuous compliance governance — monitoring, workflows, evidence and audit, in one console.
            </p>
          </div>

          {COLUMNS.map((col) => (
            <nav key={col.title} aria-label={col.title}>
              <p className="font-mono text-xs uppercase tracking-widest text-slate-500">{col.title}</p>
              <ul className="mt-4 space-y-2.5">
                {col.links.map((l) =>
                  "to" in l ? (
                    <li key={l.label}>
                      <Link to={l.to} className="text-sm text-slate-400 transition-colors hover:text-white">
                        {l.label}
                      </Link>
                    </li>
                  ) : (
                    <li key={l.label}>
                      <a href={l.href} className="text-sm text-slate-400 transition-colors hover:text-white">
                        {l.label}
                      </a>
                    </li>
                  ),
                )}
              </ul>
            </nav>
          ))}
        </div>

        <div className="mt-12 flex flex-col items-start justify-between gap-3 border-t border-white/5 pt-6 sm:flex-row sm:items-center">
          <p className="font-mono text-xs text-slate-500">© 2026 ComplianceHub · Governance console</p>
          <p className="font-mono text-xs text-slate-500">Go · Hertz · GORM · Postgres · Redis · NATS</p>
        </div>
      </div>
    </footer>
  );
}
