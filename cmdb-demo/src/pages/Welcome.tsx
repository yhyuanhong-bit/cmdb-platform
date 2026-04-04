import { memo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";

/* ──────────────────────────────────────────────
   Types & mock data
   ────────────────────────────────────────────── */

interface FeatureCard {
  icon: string;
  title: string;
  description: string;
}

const ONBOARDING_TABS = ["Welcome", "Connect", "Analyze", "Secure", "Finish"];

const FEATURE_CARDS: FeatureCard[] = [
  {
    icon: "view_in_ar",
    title: "3D Visuals",
    description: "Deep dive into spatial data clusters.",
  },
  {
    icon: "psychology",
    title: "Predictive AI",
    description: "Solve incidents before they impact users.",
  },
  {
    icon: "sync",
    title: "CMDB Sync",
    description: "Real-time inventory orchestration.",
  },
];

/* ──────────────────────────────────────────────
   Small reusable pieces (light‑theme scoped)
   ────────────────────────────────────────────── */

function LightIcon({
  name,
  className = "",
}: {
  name: string;
  className?: string;
}) {
  return (
    <span className={`material-symbols-outlined ${className}`}>{name}</span>
  );
}

/* ──────────────────────────────────────────────
   Welcome Page
   ────────────────────────────────────────────── */

const Welcome = memo(function Welcome() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [activeTab] = useState(0);

  return (
    <div className="min-h-screen bg-[#f3f7fb] font-[Inter]">
      {/* ─── Top Navigation ─── */}
      <header className="flex items-center justify-between px-8 h-16 bg-white/70 backdrop-blur-md border-b border-[#e2e8f0]">
        {/* Left: logo + tabs */}
        <div className="flex items-center gap-8">
          <span className="font-[Manrope] font-extrabold text-[#005f98] text-lg tracking-tight">
            {t('welcome.brand')}
          </span>

          <nav className="flex items-center gap-1">
            {ONBOARDING_TABS.map((tab, idx) => (
              <button
                key={tab}
                className={`px-4 py-4 text-sm font-medium transition-colors relative ${
                  idx === activeTab
                    ? "text-[#005f98]"
                    : "text-[#6b7b8d] hover:text-[#2a2f32]"
                }`}
              >
                {tab}
                {idx === activeTab && (
                  <span className="absolute bottom-0 left-4 right-4 h-[2px] bg-[#005f98] rounded-full" />
                )}
              </button>
            ))}
          </nav>
        </div>

        {/* Right: actions */}
        <div className="flex items-center gap-3">
          <button className="p-2 rounded-lg hover:bg-[#e8f0f8] transition-colors">
            <LightIcon
              name="help_outline"
              className="text-[20px] text-[#6b7b8d]"
            />
          </button>
          <button className="p-2 rounded-lg hover:bg-[#e8f0f8] transition-colors">
            <LightIcon
              name="settings"
              className="text-[20px] text-[#6b7b8d]"
            />
          </button>
          <div className="w-9 h-9 rounded-full bg-gradient-to-br from-[#005f98] to-[#2aa7ff] flex items-center justify-center shadow-md">
            <LightIcon name="person" className="text-[16px] text-white" />
          </div>
        </div>
      </header>

      {/* ─── Main Content ─── */}
      <main className="max-w-7xl mx-auto px-8 py-12 flex gap-10">
        {/* Left Column (60%) */}
        <section className="flex-[0_0_60%] flex flex-col gap-8">
          {/* Headline */}
          <div>
            <h1 className="text-4xl font-bold text-[#2a2f32] font-[Manrope] leading-tight">
              {t('welcome.headline_1')}
            </h1>
            <h1 className="text-4xl font-bold text-[#005f98] font-[Manrope] leading-tight">
              {t('welcome.headline_2')}
            </h1>
            <p className="mt-4 text-[#5a6570] text-base leading-relaxed max-w-lg">
              {t('welcome.description')}
            </p>
          </div>

          {/* Step indicator */}
          <div className="flex items-center gap-3">
            <span className="inline-flex items-center gap-2 bg-[#005f98]/10 text-[#005f98] text-xs font-semibold px-3 py-1.5 rounded-full">
              <span className="w-2 h-2 rounded-full bg-[#005f98]" />
              {t('welcome.step_indicator')}
            </span>
          </div>

          {/* Feature cards */}
          <div className="grid grid-cols-3 gap-4">
            {FEATURE_CARDS.map((card) => (
              <div
                key={card.title}
                className="bg-white rounded-xl shadow-sm border border-[#e8edf2] p-5 flex flex-col gap-3 hover:shadow-md transition-shadow"
              >
                <div className="w-10 h-10 rounded-lg bg-[#005f98]/10 flex items-center justify-center">
                  <LightIcon
                    name={card.icon}
                    className="text-[22px] text-[#005f98]"
                  />
                </div>
                <h3 className="text-sm font-semibold text-[#2a2f32] font-[Manrope]">
                  {card.title}
                </h3>
                <p className="text-xs text-[#6b7b8d] leading-relaxed">
                  {card.description}
                </p>
              </div>
            ))}
          </div>

          {/* Actions */}
          <div className="flex items-center gap-6 mt-2">
            <button
              onClick={() => navigate('/dashboard')}
              className="text-sm text-[#6b7b8d] hover:text-[#2a2f32] transition-colors font-medium"
            >
              {t('welcome.skip')}
            </button>
            <button
              onClick={() => navigate('/locations')}
              className="inline-flex items-center gap-2 bg-gradient-to-r from-[#005f98] to-[#2aa7ff] text-white text-sm font-semibold px-6 py-3 rounded-xl shadow-lg shadow-[#005f98]/25 hover:shadow-xl hover:shadow-[#005f98]/30 transition-all active:scale-[0.98]"
            >
              {t('welcome.next_step')}
              <LightIcon name="arrow_forward" className="text-[18px]" />
            </button>
          </div>
        </section>

        {/* Right Column (40%) */}
        <section className="flex-[0_0_40%] flex flex-col gap-4 relative">
          {/* Dark illustration placeholder */}
          <div className="bg-[#1a2332] rounded-2xl flex-1 min-h-[420px] flex items-center justify-center relative overflow-hidden">
            {/* Decorative elements */}
            <div className="absolute inset-0 opacity-20">
              <div className="absolute top-8 left-8 w-32 h-32 border border-[#2aa7ff]/30 rounded-xl rotate-12" />
              <div className="absolute bottom-16 right-8 w-24 h-24 border border-[#005f98]/30 rounded-full" />
              <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-40 h-40 bg-gradient-to-br from-[#005f98]/20 to-[#2aa7ff]/20 rounded-2xl rotate-45" />
            </div>
            <div className="flex flex-col items-center gap-3 z-10">
              <LightIcon
                name="deployed_code"
                className="text-[64px] text-[#2aa7ff]/40"
              />
              <span className="text-[#5a7a94] text-sm font-medium">
                {t('welcome.visualization_preview')}
              </span>
            </div>
          </div>

          {/* Floating status card */}
          <div className="absolute bottom-6 left-4 right-4 bg-white/95 backdrop-blur-md rounded-xl shadow-lg border border-[#e8edf2] px-5 py-4 flex items-center gap-3">
            <div className="w-9 h-9 rounded-lg bg-[#2aa7ff]/10 flex items-center justify-center shrink-0">
              <span className="text-lg">&#x26A1;</span>
            </div>
            <div className="min-w-0">
              <p className="text-sm font-semibold text-[#2a2f32]">
                {t('welcome.predictive_analysis_active')}
              </p>
              <p className="text-xs text-[#6b7b8d]">
                {t('welcome.realtime_monitoring_enabled')}
              </p>
            </div>
            <div className="ml-auto shrink-0">
              <span className="flex h-2.5 w-2.5">
                <span className="animate-ping absolute inline-flex h-2.5 w-2.5 rounded-full bg-[#22c55e] opacity-75" />
                <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-[#22c55e]" />
              </span>
            </div>
          </div>
        </section>
      </main>
    </div>
  );
});

export default Welcome;
