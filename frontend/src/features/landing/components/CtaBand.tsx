import { ArrowRight } from "lucide-react";
import { Link } from "react-router-dom";
import { useAppSelector } from "@/store/hooks";
import { Reveal } from "./Reveal";

export function CtaBand() {
  const token = useAppSelector((s) => s.auth.token);

  return (
    <section className="bg-white pb-24 lg:pb-32">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <Reveal>
          <div className="landing-grid relative overflow-hidden rounded-3xl bg-ink-950 px-6 py-16 text-center shadow-modal sm:px-12 lg:py-20">
            <div className="pointer-events-none absolute -top-32 left-1/2 h-72 w-[560px] -translate-x-1/2 rounded-full bg-primary-600/25 blur-3xl" aria-hidden="true" />
            <div className="pointer-events-none absolute -bottom-40 right-0 h-64 w-64 rounded-full bg-primary-800/20 blur-3xl" aria-hidden="true" />

            <div className="relative">
              <p className="font-mono text-xs uppercase tracking-widest text-primary-300">Get started</p>
              <h2 className="mx-auto mt-4 max-w-2xl font-display text-3xl font-bold tracking-tight text-white sm:text-4xl">
                Ready to see your compliance posture in real time?
              </h2>
              <p className="mx-auto mt-4 max-w-xl text-base leading-relaxed text-slate-400">
                The console is live — every module, every workflow, every audit log. Sign in and watch the board move.
              </p>
              <div className="mt-9 flex flex-wrap items-center justify-center gap-3">
                <Link
                  to={token ? "/app" : "/login"}
                  className="group inline-flex h-11 items-center gap-2 rounded-lg bg-primary-600 px-6 text-sm font-semibold text-white shadow-lg shadow-primary-600/30 transition-all duration-150 hover:bg-primary-500 active:scale-[0.98]"
                >
                  {token ? "Open the console" : "Sign in"}
                  <ArrowRight className="h-4 w-4 transition-transform duration-150 group-hover:translate-x-0.5" aria-hidden="true" />
                </Link>
                <a
                  href="#features"
                  className="inline-flex h-11 items-center rounded-lg border border-white/10 bg-white/5 px-6 text-sm font-semibold text-white transition-colors duration-150 hover:bg-white/10"
                >
                  Review capabilities
                </a>
              </div>
            </div>
          </div>
        </Reveal>
      </div>
    </section>
  );
}
