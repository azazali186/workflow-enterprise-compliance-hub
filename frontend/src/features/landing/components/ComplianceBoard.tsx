import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { useEffect, useState } from "react";
import { cn } from "@/lib/cn";

type Status = "compliant" | "in_review" | "non_compliant";

const STATUS_META: Record<Status, { label: string; dot: string; text: string; chip: string }> = {
  compliant: { label: "Compliant", dot: "bg-success-500", text: "text-success-400", chip: "bg-success-500/10 text-success-400 ring-success-500/20" },
  in_review: { label: "In review", dot: "bg-warning-500", text: "text-warning-400", chip: "bg-warning-500/10 text-warning-400 ring-warning-500/20" },
  non_compliant: { label: "At risk", dot: "bg-danger-500", text: "text-danger-400", chip: "bg-danger-500/10 text-danger-400 ring-danger-500/20" },
};

interface Obligation {
  code: string;
  name: string;
  timeline: Status[];
}

const OBLIGATIONS: Obligation[] = [
  { code: "GDPR", name: "EU General Data Protection", timeline: ["compliant", "in_review", "compliant", "compliant"] },
  { code: "SOC 2", name: "Type II attestation", timeline: ["in_review", "in_review", "compliant", "compliant"] },
  { code: "ISO 27001", name: "ISMS certification", timeline: ["non_compliant", "non_compliant", "in_review", "in_review"] },
  { code: "PCI DSS", name: "Cardholder data environment", timeline: ["compliant", "compliant", "in_review", "compliant"] },
];

const EVENTS = [
  "violation_detected · ISO 27001",
  "deadline_approaching · PCI DSS",
  "audit_scheduled · SOC 2",
  "compliance.updated · GDPR",
  "corrective_action_flow · #CA-0412",
];

const SEVERITY: Record<Status, string> = { compliant: "low", in_review: "medium", non_compliant: "critical" };

export function ComplianceBoard() {
  const reduce = useReducedMotion();
  const [tick, setTick] = useState(0);

  useEffect(() => {
    if (reduce) return;
    const id = window.setInterval(() => setTick((t) => t + 1), 2400);
    return () => window.clearInterval(id);
  }, [reduce]);

  const event = EVENTS[tick % EVENTS.length];

  return (
    <div className="relative">
      {/* Ambient glow behind the monitor */}
      <div
        className="pointer-events-none absolute -inset-8 rounded-[2rem] bg-primary-600/20 blur-3xl"
        aria-hidden="true"
      />

      <div className="relative overflow-hidden rounded-2xl border border-white/10 bg-ink-900/90 shadow-modal backdrop-blur">
        <div className="landing-scan" aria-hidden="true" />

        {/* Console header */}
        <div className="flex items-center justify-between border-b border-white/5 px-4 py-3">
          <div className="flex items-center gap-1.5" aria-hidden="true">
            <span className="h-2.5 w-2.5 rounded-full bg-danger-500/80" />
            <span className="h-2.5 w-2.5 rounded-full bg-warning-500/80" />
            <span className="h-2.5 w-2.5 rounded-full bg-success-500/80" />
          </div>
          <p className="font-mono text-[11px] uppercase tracking-widest text-slate-400">compliance monitor</p>
          <span className="flex items-center gap-1.5 font-mono text-[11px] text-success-400">
            <span className="relative flex h-1.5 w-1.5">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-success-500 opacity-60" aria-hidden="true" />
              <span className="relative inline-flex h-1.5 w-1.5 rounded-full bg-success-500" aria-hidden="true" />
            </span>
            LIVE
          </span>
        </div>

        {/* Obligation rows */}
        <ul className="divide-y divide-white/5">
          {OBLIGATIONS.map((o) => {
            const status = o.timeline[tick % o.timeline.length];
            const meta = STATUS_META[status];
            return (
              <li key={o.code} className="flex items-center justify-between gap-3 px-4 py-3">
                <div className="flex min-w-0 items-center gap-3">
                  <span className="w-20 shrink-0 font-mono text-xs font-semibold tracking-wide text-slate-200">{o.code}</span>
                  <span className="truncate text-sm text-slate-400">{o.name}</span>
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  <span className={cn("hidden rounded-md px-1.5 py-0.5 font-mono text-[10px] uppercase ring-1 ring-inset sm:inline", meta.chip)}>
                    {SEVERITY[status]}
                  </span>
                  <span className={cn("flex w-24 items-center gap-1.5 rounded-md px-2 py-1 text-[11px] font-medium", meta.chip)}>
                    <span className={cn("h-1.5 w-1.5 rounded-full", meta.dot)} aria-hidden="true" />
                    <AnimatePresence mode="wait" initial={false}>
                      <motion.span
                        key={status}
                        initial={{ opacity: 0, y: 4 }}
                        animate={{ opacity: 1, y: 0 }}
                        exit={{ opacity: 0, y: -4 }}
                        transition={{ duration: 0.18 }}
                        className={cn("truncate", meta.text)}
                      >
                        {meta.label}
                      </motion.span>
                    </AnimatePresence>
                  </span>
                </div>
              </li>
            );
          })}
        </ul>

        {/* Event ticker */}
        <div className="flex items-center gap-2 border-t border-white/5 bg-ink-950/60 px-4 py-2.5">
          <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-primary-400" aria-hidden="true" />
          <AnimatePresence mode="wait" initial={false}>
            <motion.p
              key={event}
              initial={{ opacity: 0, y: 6 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -6 }}
              transition={{ duration: 0.2 }}
              className="truncate font-mono text-[11px] text-slate-400"
            >
              <span className="text-slate-500">bus › </span>
              {event}
            </motion.p>
          </AnimatePresence>
        </div>
      </div>

      {/* Floating KPI chips */}
      <motion.div
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.9, duration: 0.5, ease: [0.22, 1, 0.36, 1] }}
        className="absolute -left-6 -top-5 hidden rounded-xl border border-white/10 bg-ink-900/90 px-3 py-2 shadow-pop backdrop-blur md:block"
      >
        <p className="font-mono text-[10px] uppercase tracking-widest text-slate-500">open violations</p>
        <p className="font-mono text-lg font-semibold text-danger-400">3</p>
      </motion.div>
      <motion.div
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 1.1, duration: 0.5, ease: [0.22, 1, 0.36, 1] }}
        className="absolute -bottom-5 -right-6 hidden rounded-xl border border-white/10 bg-ink-900/90 px-3 py-2 shadow-pop backdrop-blur md:block"
      >
        <p className="font-mono text-[10px] uppercase tracking-widest text-slate-500">audit coverage</p>
        <p className="font-mono text-lg font-semibold text-success-400">100%</p>
      </motion.div>
    </div>
  );
}
