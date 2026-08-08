import { CtaBand } from "../components/CtaBand";
import { FeaturesGrid } from "../components/FeaturesGrid";
import { Hero } from "../components/Hero";
import { HowItWorks } from "../components/HowItWorks";
import { LandingFooter } from "../components/LandingFooter";
import { LandingNav } from "../components/LandingNav";
import { SecuritySection } from "../components/SecuritySection";
import { StatsStrip } from "../components/StatsStrip";

/**
 * Public marketing landing for ComplianceHub. Owns the "/" route; the
 * authenticated console lives under "/app". Lightweight on purpose — no data
 * fetching, no auth — so it renders instantly for visitors and search engines.
 */
export function LandingPage() {
  return (
    <div className="min-h-dvh bg-ink-950 font-sans text-slate-300 antialiased">
      <LandingNav />
      <main>
        <Hero />
        <StatsStrip />
        <FeaturesGrid />
        <HowItWorks />
        <SecuritySection />
        <CtaBand />
      </main>
      <LandingFooter />
    </div>
  );
}
