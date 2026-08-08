import { AnimatePresence, motion } from "framer-motion";
import { ArrowRight, Menu, ShieldCheck, X } from "lucide-react";
import { useState } from "react";
import { Link } from "react-router-dom";
import { useAppSelector } from "@/store/hooks";

const LINKS = [
  { href: "#features", label: "Capabilities" },
  { href: "#how", label: "How it works" },
  { href: "#security", label: "Security" },
];

export function LandingNav() {
  const token = useAppSelector((s) => s.auth.token);
  const [open, setOpen] = useState(false);

  const cta = token ? (
    <Link
      to="/app"
      className="group inline-flex h-9.5 items-center gap-2 rounded-lg bg-primary-600 px-4 text-sm font-semibold text-white shadow-sm transition-all duration-150 hover:bg-primary-500 active:scale-[0.98]"
    >
      Open console
      <ArrowRight className="h-4 w-4 transition-transform duration-150 group-hover:translate-x-0.5" aria-hidden="true" />
    </Link>
  ) : (
    <Link
      to="/login"
      className="inline-flex h-9.5 items-center rounded-lg border border-white/10 bg-white/5 px-4 text-sm font-semibold text-white transition-colors duration-150 hover:bg-white/10"
    >
      Sign in
    </Link>
  );

  return (
    <header className="sticky top-0 z-40 border-b border-white/5 bg-ink-950/70 backdrop-blur-xl">
      <nav className="mx-auto flex h-16 max-w-7xl items-center justify-between px-4 sm:px-6 lg:px-8" aria-label="Main">
        <Link to="/" className="flex items-center gap-2.5" aria-label="ComplianceHub home">
          <span className="brand-mark flex h-8 w-8 items-center justify-center rounded-lg" aria-hidden="true">
            <ShieldCheck className="h-4.5 w-4.5 text-white" />
          </span>
          <span className="font-display text-base font-semibold tracking-tight text-white">ComplianceHub</span>
        </Link>

        <div className="hidden items-center gap-1 md:flex">
          {LINKS.map((l) => (
            <a
              key={l.href}
              href={l.href}
              className="rounded-lg px-3 py-2 text-sm font-medium text-slate-300 transition-colors duration-150 hover:bg-white/5 hover:text-white"
            >
              {l.label}
            </a>
          ))}
        </div>

        <div className="hidden items-center gap-3 md:flex">{cta}</div>

        <button
          type="button"
          onClick={() => setOpen((o) => !o)}
          className="rounded-lg p-2 text-slate-300 transition-colors hover:bg-white/5 hover:text-white md:hidden"
          aria-expanded={open}
          aria-label={open ? "Close menu" : "Open menu"}
        >
          {open ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
        </button>
      </nav>

      <AnimatePresence>
        {open && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: "auto" }}
            exit={{ opacity: 0, height: 0 }}
            transition={{ duration: 0.18, ease: "easeOut" }}
            className="overflow-hidden border-t border-white/5 md:hidden"
          >
            <div className="space-y-1 px-4 py-4">
              {LINKS.map((l) => (
                <a
                  key={l.href}
                  href={l.href}
                  onClick={() => setOpen(false)}
                  className="block rounded-lg px-3 py-2.5 text-sm font-medium text-slate-200 transition-colors hover:bg-white/5"
                >
                  {l.label}
                </a>
              ))}
              <div className="pt-2">{cta}</div>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </header>
  );
}
