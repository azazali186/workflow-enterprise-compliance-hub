import { motion, useReducedMotion } from "framer-motion";
import { ArrowRight, ChevronDown, Lock } from "lucide-react";
import { Link } from "react-router-dom";
import { useAppSelector } from "@/store/hooks";
import { ComplianceBoard } from "./ComplianceBoard";

const FRAMEWORKS = ["SOC 2", "GDPR", "ISO 27001", "PCI DSS"];

const container = {
  hidden: {},
  visible: { transition: { staggerChildren: 0.1, delayChildren: 0.15 } },
};

const item = {
  hidden: { opacity: 0, y: 24 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.6, ease: [0.22, 1, 0.36, 1] as const } },
};

export function Hero() {
  const token = useAppSelector((s) => s.auth.token);
  const reduce = useReducedMotion();
  const animate = reduce ? false : "visible";

  return (
    <section className="landing-grid relative overflow-hidden">
      {/* Radial violet glows */}
      <div className="pointer-events-none absolute -top-40 left-1/2 h-[560px] w-[900px] -translate-x-1/2 rounded-full bg-primary-600/20 blur-3xl" aria-hidden="true" />
      <div className="pointer-events-none absolute -bottom-56 -left-40 h-96 w-96 rounded-full bg-primary-800/20 blur-3xl" aria-hidden="true" />

      <div className="relative mx-auto grid max-w-7xl items-center gap-14 px-4 pb-24 pt-16 sm:px-6 lg:grid-cols-2 lg:gap-10 lg:px-8 lg:pb-32 lg:pt-24">
        <motion.div variants={container} initial={reduce ? false : "hidden"} animate={animate}>
          <motion.div variants={item}>
            <div className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/5 px-3 py-1">
              <span className="font-mono text-[11px] uppercase tracking-widest text-slate-400">Built for</span>
              {FRAMEWORKS.map((f) => (
                <span key={f} className="font-mono text-[11px] font-semibold text-primary-300">
                  {f}
                </span>
              ))}
            </div>
          </motion.div>

          <motion.h1
            variants={item}
            className="mt-6 font-display text-4xl font-bold leading-[1.08] tracking-tight text-white sm:text-5xl lg:text-6xl"
          >
            Know your compliance posture{" "}
            <span className="bg-gradient-to-r from-primary-300 via-primary-400 to-primary-500 bg-clip-text text-transparent">
              in real time
            </span>
            .
          </motion.h1>

          <motion.p variants={item} className="mt-6 max-w-xl text-lg leading-relaxed text-slate-400">
            ComplianceHub continuously monitors every obligation, evaluates each control, and turns violations into
            tracked corrective actions — with a complete before/after audit trail behind every change.
          </motion.p>

          <motion.div variants={item} className="mt-9 flex flex-wrap items-center gap-3">
            <Link
              to={token ? "/app" : "/login"}
              className="group inline-flex h-11 items-center gap-2 rounded-lg bg-primary-600 px-6 text-sm font-semibold text-white shadow-lg shadow-primary-600/25 transition-all duration-150 hover:bg-primary-500 hover:shadow-primary-500/30 active:scale-[0.98]"
            >
              {token ? "Open the console" : "Sign in to the console"}
              <ArrowRight className="h-4 w-4 transition-transform duration-150 group-hover:translate-x-0.5" aria-hidden="true" />
            </Link>
            <a
              href="#how"
              className="inline-flex h-11 items-center gap-2 rounded-lg border border-white/10 bg-white/5 px-6 text-sm font-semibold text-white transition-colors duration-150 hover:bg-white/10"
            >
              See how it works
              <ChevronDown className="h-4 w-4" aria-hidden="true" />
            </a>
          </motion.div>

          <motion.p variants={item} className="mt-8 flex items-center gap-2 text-xs text-slate-400">
            <Lock className="h-3.5 w-3.5" aria-hidden="true" />
            AES-256-GCM encryption at rest · route-level role-based access · full audit trail
          </motion.p>
        </motion.div>

        <motion.div
          initial={reduce ? false : { opacity: 0, scale: 0.96, y: 20 }}
          animate={reduce ? undefined : { opacity: 1, scale: 1, y: 0 }}
          transition={{ duration: 0.7, delay: 0.35, ease: [0.22, 1, 0.36, 1] }}
        >
          <ComplianceBoard />
        </motion.div>
      </div>
    </section>
  );
}
