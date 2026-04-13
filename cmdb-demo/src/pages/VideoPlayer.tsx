import { toast } from 'sonner'
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate, useSearchParams } from 'react-router-dom';

const tabs = [
  { key: 'Inventory SOPs', i18n: 'video_player.tab_inventory_sops' },
  { key: 'Maintenance', i18n: 'video_player.tab_maintenance' },
  { key: 'Navigation', i18n: 'video_player.tab_navigation' },
];

const VIDEO_DATA: Record<string, { titleKey: string; chapters: { titleKey: string; duration: string }[] }> = {
  '1': { titleKey: 'video_player.video_title_1', chapters: [{ titleKey: 'video_player.chapter_introduction', duration: '3:20' }, { titleKey: 'video_player.chapter_creating_assets', duration: '5:10' }, { titleKey: 'video_player.chapter_asset_lifecycle', duration: '8:45' }] },
  '2': { titleKey: 'video_player.video_title_2', chapters: [{ titleKey: 'video_player.chapter_scanning_basics', duration: '4:00' }, { titleKey: 'video_player.chapter_qr_code_scanning', duration: '6:30' }, { titleKey: 'video_player.chapter_discrepancy_handling', duration: '7:15' }] },
  '3': { titleKey: 'video_player.video_title_3', chapters: [{ titleKey: 'video_player.chapter_overview', duration: '3:45' }, { titleKey: 'video_player.chapter_connection_mapping', duration: '5:20' }] },
  '4': { titleKey: 'video_player.video_title_4', chapters: [{ titleKey: 'video_player.chapter_ai_models', duration: '4:30' }, { titleKey: 'video_player.chapter_rca_analysis', duration: '6:00' }] },
  '5': { titleKey: 'video_player.video_title_5', chapters: [{ titleKey: 'video_player.chapter_scoring_rules', duration: '5:15' }, { titleKey: 'video_player.chapter_dependencies', duration: '7:00' }] },
  '6': { titleKey: 'video_player.video_title_6', chapters: [{ titleKey: 'video_player.chapter_user_management', duration: '4:45' }, { titleKey: 'video_player.chapter_role_permissions', duration: '6:20' }] },
};

const TAG_KEYS = ['video_player.tag_hardware_maintenance', 'video_player.tag_sop_compliance', 'video_player.tag_tier3_training'];

export default function VideoPlayer() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const videoId = searchParams.get('v') || '';
  const currentVideo = VIDEO_DATA[videoId] || VIDEO_DATA['1'];
  const [activeTab, setActiveTab] = useState('Inventory SOPs');

  return (
    <div className="min-h-screen bg-surface text-on-surface font-body p-8">
      {/* Back Link */}
      <button
        onClick={() => navigate('/help/videos')}
        className="flex items-center gap-1.5 text-on-surface-variant hover:text-primary text-sm mb-6 cursor-pointer"
      >
        <span className="material-symbols-outlined text-[18px]">arrow_back</span>
        <span className="text-sm">{t('video_player.back_to_library')}</span>
      </button>

      {/* Title & Tabs */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-6">
        <h1 className="font-headline font-bold text-3xl tracking-tight">{t('video_player.title')}</h1>
        <div className="flex gap-1 bg-surface-container rounded-xl p-1">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              onClick={() => setActiveTab(tab.key)}
              className={`px-4 py-2 rounded-lg text-xs font-semibold tracking-wider uppercase cursor-pointer transition-colors ${
                activeTab === tab.key
                  ? 'bg-surface-container-high text-on-surface'
                  : 'text-on-surface-variant hover:text-on-surface'
              }`}
            >
              {t(tab.i18n)}
            </button>
          ))}
        </div>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6">
        {/* Left: Video + Details */}
        <div className="xl:col-span-2 flex flex-col gap-6">
          {/* Video Player Placeholder */}
          <div className="bg-surface-container-low rounded-2xl aspect-video flex items-center justify-center relative overflow-hidden">
            <div className="absolute inset-0 bg-gradient-to-br from-surface-container/80 to-surface-container-low/40" />
            <button className="relative z-10 w-20 h-20 rounded-full bg-primary/20 flex items-center justify-center cursor-pointer hover:bg-primary/30 transition-colors group">
              <span className="material-symbols-outlined text-primary text-4xl group-hover:scale-110 transition-transform">
                play_arrow
              </span>
            </button>
            <div className="absolute bottom-4 left-4 right-4 z-10">
              <div className="h-1 bg-surface-container-highest rounded-full overflow-hidden">
                <div className="h-full bg-primary rounded-full" style={{ width: '0%' }} />
              </div>
            </div>
          </div>

          {/* Module Label */}
          <div>
            <p className="text-xs text-on-surface-variant tracking-widest uppercase font-semibold mb-2">
              {t('video_player.section_rack_operations')} <span className="text-primary">&bull;</span> {t('video_player.module_label', { num: '04' })}
            </p>
            <h2 className="font-headline font-bold text-3xl text-on-surface leading-tight mb-4">
              {t(currentVideo.titleKey)}
            </h2>
            <p className="text-on-surface-variant text-sm leading-relaxed max-w-2xl mb-6">
              {t('video_player.video_description')}
            </p>

            {/* Instructor */}
            <div className="flex items-center gap-3 mb-6">
              <div className="w-10 h-10 rounded-full bg-surface-container-high flex items-center justify-center">
                <span className="material-symbols-outlined text-on-surface-variant text-xl">person</span>
              </div>
              <div>
                <p className="text-on-surface text-sm font-semibold">{t('video_player.label_instructor')}</p>
                <p className="text-on-surface-variant text-xs">{t('video_player.label_instructor_role')}</p>
              </div>
            </div>

            {/* Tags */}
            <div className="flex flex-wrap gap-2 mb-6">
              {TAG_KEYS.map((tagKey) => (
                <span
                  key={tagKey}
                  className="bg-surface-container-high px-3 py-1.5 rounded-lg text-[0.625rem] font-semibold tracking-widest text-on-surface-variant uppercase"
                >
                  {t(tagKey)}
                </span>
              ))}
            </div>

            {/* Download Button */}
            <button onClick={() => toast.info('Coming Soon')} className="machined-gradient text-[#001b34] font-semibold text-sm tracking-wider uppercase px-6 py-3 rounded-lg cursor-pointer hover:opacity-90 transition-opacity">
              <span className="flex items-center gap-2">
                <span className="material-symbols-outlined text-[18px]">download</span>
                {t('video_player.btn_download_sop_pdf')}
              </span>
            </button>
          </div>
        </div>

        {/* Right: Chapter List */}
        <div className="flex flex-col gap-4">
          <div className="bg-surface-container rounded-2xl p-6">
            <div className="flex items-center justify-between mb-6">
              <h3 className="font-headline font-bold text-base tracking-tight">{t('video_player.section_rack_operations')}</h3>
              <span className="text-xs text-on-surface-variant tracking-wider">{t('video_player.label_modules_progress')}</span>
            </div>

            <div className="flex flex-col gap-2">
              {currentVideo.chapters.map((ch, idx) => (
                <button
                  key={idx}
                  className="w-full text-left rounded-xl p-4 cursor-pointer transition-colors bg-surface-container-low hover:bg-surface-container-high"
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="flex items-start gap-3 min-w-0">
                      <div className="w-8 h-8 rounded-lg flex-shrink-0 flex items-center justify-center mt-0.5 bg-surface-container-high">
                        <span className="text-on-surface-variant text-xs font-semibold">{String(idx + 1).padStart(2, '0')}</span>
                      </div>
                      <div className="min-w-0">
                        <p className="text-[0.625rem] text-on-surface-variant tracking-widest uppercase mb-0.5">
                          {t('video_player.chapter_label', { num: String(idx + 1).padStart(2, '0') })}
                        </p>
                        <p className="text-sm font-medium truncate text-on-surface">
                          {t(ch.titleKey)}
                        </p>
                      </div>
                    </div>
                    <span className="text-on-surface-variant text-xs font-mono flex-shrink-0 mt-1">{ch.duration}</span>
                  </div>
                </button>
              ))}
            </div>
          </div>

          {/* Course Completion */}
          <div className="bg-surface-container rounded-2xl p-6">
            <div className="flex items-center justify-between mb-3">
              <p className="text-xs text-on-surface-variant tracking-wider uppercase font-semibold">{t('video_player.section_course_completion')}</p>
              <span className="text-on-surface-variant text-sm font-bold">{t('video_player.status_not_started')}</span>
            </div>
            <div className="h-2 bg-surface-container-low rounded-full overflow-hidden">
              <div className="h-full bg-primary rounded-full transition-all" style={{ width: '0%' }} />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
