import { Reveal } from "./Reveal";

const STATS = [
  { value: "4", label: "automated workflows", sub: "detect → evaluate → remediate → report" },
  { value: "AES-256-GCM", label: "encryption at rest", sub: "with versioned key rotation" },
  { value: "Route-level", label: "RBAC on every endpoint", sub: "admin · officer · viewer" },
  { value: "Before/after", label: "audit snapshots", sub: "for every create, update, delete" },
];

export function StatsStrip() {
  return (
    <section className="border-y border-slate-200 bg-white">
      <div className="mx-auto grid max-w-7xl grid-cols-1 divide-y divide-slate-200 sm:grid-cols-2 sm:divide-y-0 lg:grid-cols-4 lg:divide-x">
        {STATS.map((s, i) => (
          <Reveal key={s.value} delay={i * 0.08} className="px-6 py-8 sm:px-8">
            <p className="font-mono text-2xl font-semibold tracking-tight text-slate-900">{s.value}</p>
            <p className="mt-1.5 text-sm font-medium text-slate-700">{s.label}</p>
            <p className="mt-0.5 font-mono text-[11px] text-slate-400">{s.sub}</p>
          </Reveal>
        ))}
      </div>
    </section>
  );
}
