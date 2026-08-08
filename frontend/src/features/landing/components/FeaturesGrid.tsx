import { motion, useReducedMotion } from "framer-motion";
import {
  Activity,
  BellRing,
  FileCheck2,
  History,
  ShieldCheck,
  Workflow,
  type LucideIcon,
} from "lucide-react";
import { cn } from "@/lib/cn";
import { Reveal } from "./Reveal";

interface Feature {
  icon: LucideIcon;
  title: string;
  copy: string;
  accent: string;
  span?: boolean;
  meta?: string[];
}

const FEATURES: Feature[] = [
  {
    icon: Activity,
    title: "Continuous monitoring",
    copy: "Every compliance, deadline and control is evaluated automatically. The moment something drifts, a violation is raised and an alert fires.",
    accent: "text-primary-600 bg-primary-50 ring-primary-100",
    span: true,
    meta: ["compliances", "violations", "deadlines"],
  },
  {
    icon: BellRing,
    title: "Real-time alerts",
    copy: "WebSocket events land the instant anything changes — no polling, no refresh.",
    accent: "text-warning-600 bg-warning-50 ring-warning-100",
    meta: ["violation_detected", "deadline_approaching"],
  },
  {
    icon: Workflow,
    title: "Workflows that close the loop",
    copy: "Violations become corrective actions with owners and due dates, tracked to completion.",
    accent: "text-info-600 bg-info-50 ring-info-100",
  },
  {
    icon: FileCheck2,
    title: "Reports & analytics",
    copy: "Summaries and breakdowns generated on demand — ready to hand to auditors.",
    accent: "text-success-600 bg-success-50 ring-success-100",
  },
  {
    icon: History,
    title: "Audit trail on everything",
    copy: "Before and after snapshots, actor, IP and a field-level diff for every mutation.",
    accent: "text-primary-600 bg-primary-50 ring-primary-100",
  },
  {
    icon: ShieldCheck,
    title: "Role-based access",
    copy: "Route-level permissions enforced server-side. Admin, compliance officer and read-only viewer.",
    accent: "text-success-600 bg-success-50 ring-success-100",
  },
];

export function FeaturesGrid() {
  const reduce = useReducedMotion();

  return (
    <section id="features" className="scroll-mt-20 bg-canvas py-20 lg:py-28">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <Reveal className="max-w-2xl">
          <p className="font-mono text-xs uppercase tracking-widest text-primary-600">Capabilities</p>
          <h2 className="mt-3 font-display text-3xl font-bold tracking-tight text-slate-900 sm:text-4xl">
            Everything a compliance team needs, in one console.
          </h2>
          <p className="mt-4 text-base leading-relaxed text-slate-600">
            No spreadsheets, no chasing owners, no reconstructing what happened. The platform does the watching and
            keeps the receipts.
          </p>
        </Reveal>

        <motion.div
          className="mt-12 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3"
          initial={reduce ? false : "hidden"}
          whileInView={reduce ? undefined : "visible"}
          viewport={{ once: true, amount: 0.15 }}
          variants={{
            hidden: {},
            visible: { transition: { staggerChildren: 0.08 } },
          }}
        >
          {FEATURES.map((f) => {
            const Icon = f.icon;
            return (
              <motion.article
                key={f.title}
                variants={{
                  hidden: { opacity: 0, y: 24 },
                  visible: { opacity: 1, y: 0, transition: { duration: 0.5, ease: [0.22, 1, 0.36, 1] } },
                }}
                whileHover={reduce ? undefined : { y: -4 }}
                className={cn(
                  "group rounded-2xl border border-slate-200 bg-white p-6 shadow-card transition-shadow duration-200 hover:shadow-lift",
                  // True bento: the featured card fills a 2×2 block on desktop so
                  // the 6-card grid lands on an exact 3×3 cell area (no orphans).
                  f.span && "lg:col-span-2 lg:row-span-2",
                )}
              >
                <span className={cn("inline-flex h-10 w-10 items-center justify-center rounded-xl ring-1 ring-inset", f.accent)}>
                  <Icon className="h-5 w-5" aria-hidden="true" />
                </span>
                <h3 className="mt-4 font-display text-lg font-semibold tracking-tight text-slate-900">{f.title}</h3>
                <p className="mt-2 text-sm leading-relaxed text-slate-600">{f.copy}</p>
                {f.meta && (
                  <div className="mt-4 flex flex-wrap gap-1.5">
                    {f.meta.map((m) => (
                      <span key={m} className="rounded-md bg-slate-50 px-2 py-0.5 font-mono text-[11px] text-slate-500 ring-1 ring-inset ring-slate-200">
                        {m}
                      </span>
                    ))}
                  </div>
                )}
              </motion.article>
            );
          })}
        </motion.div>
      </div>
    </section>
  );
}
