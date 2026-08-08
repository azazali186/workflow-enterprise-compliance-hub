import { KeyRound, Lock, RefreshCcw, ScrollText, UserCheck, type LucideIcon } from "lucide-react";
import { cn } from "@/lib/cn";
import { Reveal } from "./Reveal";

const POINTS: Array<{ icon: LucideIcon; title: string; copy: string }> = [
  {
    icon: Lock,
    title: "Encryption at rest",
    copy: "Sensitive values are AES-256-GCM encrypted before they reach the database — secrets never appear in payloads, logs or audit snapshots.",
  },
  {
    icon: RefreshCcw,
    title: "Versioned key rotation",
    copy: "Encryption keys rotate without downtime: dual-read while a background worker re-encrypts every row to the new key.",
  },
  {
    icon: UserCheck,
    title: "Least-privilege access",
    copy: "Every route is an explicit permission. Tokens are single-session, passwords are bcrypt-hashed, and failed logins are rate limited.",
  },
  {
    icon: ScrollText,
    title: "Immutable event history",
    copy: "Reliable event delivery through an outbox with retries and backoff — nothing silently dropped, everything replayable.",
  },
];

export function SecuritySection() {
  return (
    <section id="security" className="scroll-mt-20 bg-white py-20 lg:py-28">
      <div className="mx-auto grid max-w-7xl items-center gap-12 px-4 sm:px-6 lg:grid-cols-2 lg:gap-16 lg:px-8">
        <Reveal>
          <p className="font-mono text-xs uppercase tracking-widest text-primary-600">Security</p>
          <h2 className="mt-3 font-display text-3xl font-bold tracking-tight text-slate-900 sm:text-4xl">
            Built for the people who audit the auditors.
          </h2>
          <p className="mt-4 text-base leading-relaxed text-slate-600">
            ComplianceHub is designed from the ground up to hold up to scrutiny: encryption on the way to disk, access
            control on every single route, and a trail you can point at.
          </p>

          <div className="mt-8 rounded-2xl border border-slate-200 bg-canvas p-5">
            <div className="flex items-center gap-2.5">
              <KeyRound className="h-4 w-4 text-primary-600" aria-hidden="true" />
              <p className="font-mono text-xs uppercase tracking-widest text-slate-500">Secrets policy</p>
            </div>
            <ul className="mt-3 space-y-2 font-mono text-xs text-slate-500">
              <li>· no tokens in URLs or access logs</li>
              <li>· no raw passwords — bcrypt hashes only</li>
              <li>· redacted audit fields, no sensitive payloads</li>
            </ul>
          </div>
        </Reveal>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          {POINTS.map((p, i) => {
            const Icon = p.icon;
            return (
              <Reveal key={p.title} delay={i * 0.08}>
                <div className="h-full rounded-2xl border border-slate-200 bg-white p-5 shadow-card transition-shadow duration-200 hover:shadow-lift">
                  <span
                    className={cn(
                      "inline-flex h-9 w-9 items-center justify-center rounded-lg ring-1 ring-inset",
                      i % 2 === 0 ? "bg-primary-50 text-primary-600 ring-primary-100" : "bg-success-50 text-success-600 ring-success-100",
                    )}
                  >
                    <Icon className="h-4.5 w-4.5" aria-hidden="true" />
                  </span>
                  <h3 className="mt-3 font-display text-base font-semibold tracking-tight text-slate-900">{p.title}</h3>
                  <p className="mt-1.5 text-sm leading-relaxed text-slate-600">{p.copy}</p>
                </div>
              </Reveal>
            );
          })}
        </div>
      </div>
    </section>
  );
}
