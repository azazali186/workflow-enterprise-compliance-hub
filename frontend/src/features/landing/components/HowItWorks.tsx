import { motion, useReducedMotion } from "framer-motion";
import { Reveal } from "./Reveal";

const STEPS = [
  {
    index: "01",
    title: "Detect",
    copy: "The compliance_check saga evaluates every obligation against its controls and due dates. Drift is caught the moment it happens.",
    saga: "compliance_check",
  },
  {
    index: "02",
    title: "Evaluate",
    copy: "violation_processing triages severity, opens alerts and routes each finding to the right owner.",
    saga: "violation_processing",
  },
  {
    index: "03",
    title: "Remediate",
    copy: "corrective_action_flow tracks corrective actions with owners and deadlines until they are closed — or escalates.",
    saga: "corrective_action_flow",
  },
  {
    index: "04",
    title: "Report",
    copy: "audit_execution and the reporting layer assemble evidence with a complete before/after trail, ready for auditors.",
    saga: "audit_execution",
  },
];

export function HowItWorks() {
  const reduce = useReducedMotion();

  return (
    <section id="how" className="landing-grid relative scroll-mt-20 overflow-hidden bg-ink-950 py-20 lg:py-28">
      <div className="pointer-events-none absolute -top-40 right-0 h-96 w-96 rounded-full bg-primary-700/15 blur-3xl" aria-hidden="true" />

      <div className="relative mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <Reveal className="max-w-2xl">
          <p className="font-mono text-xs uppercase tracking-widest text-primary-400">How it works</p>
          <h2 className="mt-3 font-display text-3xl font-bold tracking-tight text-white sm:text-4xl">
            One pipeline, from drift to auditor-ready.
          </h2>
          <p className="mt-4 text-base leading-relaxed text-slate-400">
            Four orchestrator workflows run the whole loop automatically — the same sagas the console reports on.
          </p>
        </Reveal>

        <motion.ol
          className="mt-14 grid grid-cols-1 gap-10 sm:grid-cols-2 lg:grid-cols-4 lg:gap-6"
          initial={reduce ? false : "hidden"}
          whileInView={reduce ? undefined : "visible"}
          viewport={{ once: true, amount: 0.2 }}
          variants={{ hidden: {}, visible: { transition: { staggerChildren: 0.12 } } }}
        >
          {STEPS.map((step, i) => (
            <motion.li
              key={step.index}
              variants={{
                hidden: { opacity: 0, y: 24 },
                visible: { opacity: 1, y: 0, transition: { duration: 0.5, ease: [0.22, 1, 0.36, 1] } },
              }}
              className="relative"
            >
              {/* Connector line between steps (desktop) */}
              {i < STEPS.length - 1 && (
                <span
                  className="absolute -right-3 top-7 hidden h-px w-6 bg-gradient-to-r from-primary-500/60 to-transparent lg:block"
                  aria-hidden="true"
                />
              )}

              <div className="flex items-baseline gap-3">
                <span className="font-mono text-2xl font-semibold text-primary-400">{step.index}</span>
                <h3 className="font-display text-lg font-semibold tracking-tight text-white">{step.title}</h3>
              </div>
              <p className="mt-3 text-sm leading-relaxed text-slate-400">{step.copy}</p>
              <p className="mt-4 inline-block rounded-md bg-white/5 px-2 py-1 font-mono text-[11px] text-slate-400 ring-1 ring-inset ring-white/10">
                {step.saga}
              </p>
            </motion.li>
          ))}
        </motion.ol>
      </div>
    </section>
  );
}
