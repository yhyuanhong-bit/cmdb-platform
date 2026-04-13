import { memo, useState, useMemo } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";

/* ──────────────────────────────────────────────
   Mock data
   ────────────────────────────────────────────── */

type Category = "Training" | "Monitoring" | "Maintenance" | "Compliance" | "Infrastructure";

interface Video {
  id: number;
  titleKey: string;
  duration: string;
  date: string;
  category: Category;
  icon: string;
}

const VIDEOS: Video[] = [
  {
    id: 1,
    titleKey: "video_library.video_title_1",
    duration: "5:30",
    date: "2026-01-10",
    category: "Training",
    icon: "school",
  },
  {
    id: 2,
    titleKey: "video_library.video_title_2",
    duration: "8:15",
    date: "2026-01-24",
    category: "Training",
    icon: "upload_file",
  },
  {
    id: 3,
    titleKey: "video_library.video_title_3",
    duration: "6:45",
    date: "2026-02-05",
    category: "Monitoring",
    icon: "notifications_active",
  },
  {
    id: 4,
    titleKey: "video_library.video_title_4",
    duration: "7:20",
    date: "2026-02-18",
    category: "Maintenance",
    icon: "build",
  },
  {
    id: 5,
    titleKey: "video_library.video_title_5",
    duration: "9:10",
    date: "2026-03-03",
    category: "Compliance",
    icon: "fact_check",
  },
  {
    id: 6,
    titleKey: "video_library.video_title_6",
    duration: "4:55",
    date: "2026-03-15",
    category: "Infrastructure",
    icon: "view_in_ar",
  },
];

const CATEGORIES: Category[] = [
  "Training",
  "Monitoring",
  "Maintenance",
  "Compliance",
  "Infrastructure",
];

const CATEGORY_COLORS: Record<Category, string> = {
  Training: "bg-[#064e3b] text-[#34d399]",
  Monitoring: "bg-[#92400e] text-[#fbbf24]",
  Maintenance: "bg-[#1e3a5f] text-primary",
  Compliance: "bg-[#7f1d1d] text-[#ff6b6b]",
  Infrastructure: "bg-[#3b0764] text-[#c084fc]",
};

const THUMBNAIL_GRADIENTS = [
  "from-[#0c2d3f] to-[#162127]",
  "from-[#162127] to-[#0a2e1a]",
  "from-[#2d0c0c] to-[#162127]",
  "from-[#162127] to-[#1a1a2e]",
  "from-[#1a1a0c] to-[#162127]",
  "from-[#0c1a2d] to-[#1e0c2d]",
];

/* ──────────────────────────────────────────────
   Small reusable pieces
   ────────────────────────────────────────────── */

function Icon({ name, className = "" }: { name: string; className?: string }) {
  return (
    <span className={`material-symbols-outlined ${className}`}>{name}</span>
  );
}

/* ──────────────────────────────────────────────
   Video Card Components
   ────────────────────────────────────────────── */

const CATEGORY_I18N: Record<Category, string> = {
  Training: "video_library.category_training",
  Monitoring: "video_library.category_monitoring",
  Maintenance: "video_library.category_maintenance",
  Compliance: "video_library.category_compliance",
  Infrastructure: "video_library.category_infrastructure",
};

function VideoCardGrid({ video, index }: { video: Video; index: number }) {
  const { t } = useTranslation();
  return (
    <div className="group rounded-lg bg-surface-container transition-colors hover:bg-surface-container-high">
      {/* Thumbnail */}
      <div
        className={`relative flex h-40 items-center justify-center rounded-t-lg bg-gradient-to-br ${THUMBNAIL_GRADIENTS[index % THUMBNAIL_GRADIENTS.length]}`}
      >
        <Icon
          name="play_circle"
          className="text-5xl text-on-surface-variant/30 transition-all group-hover:text-primary/60 group-hover:scale-110"
        />
        {/* Duration badge */}
        <span className="absolute bottom-2 right-2 rounded bg-surface/80 px-2 py-0.5 text-[10px] font-bold text-on-surface">
          {video.duration}
        </span>
      </div>

      {/* Info */}
      <div className="p-4">
        <h3 className="mb-2 text-sm font-semibold leading-tight text-on-surface line-clamp-2">
          {t(video.titleKey)}
        </h3>
        <div className="flex items-center justify-between">
          <span className="text-[10px] text-on-surface-variant">
            {video.date}
          </span>
          <span
            className={`rounded px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider ${CATEGORY_COLORS[video.category]}`}
          >
            {t(CATEGORY_I18N[video.category])}
          </span>
        </div>
      </div>
    </div>
  );
}

function VideoCardList({ video, index }: { video: Video; index: number }) {
  const { t } = useTranslation();
  return (
    <div className="group flex rounded-lg bg-surface-container transition-colors hover:bg-surface-container-high">
      {/* Thumbnail */}
      <div
        className={`relative flex h-24 w-40 shrink-0 items-center justify-center rounded-l-lg bg-gradient-to-br ${THUMBNAIL_GRADIENTS[index % THUMBNAIL_GRADIENTS.length]}`}
      >
        <Icon
          name="play_circle"
          className="text-3xl text-on-surface-variant/30 transition-all group-hover:text-primary/60"
        />
        <span className="absolute bottom-1.5 right-1.5 rounded bg-surface/80 px-1.5 py-0.5 text-[9px] font-bold text-on-surface">
          {video.duration}
        </span>
      </div>

      {/* Info */}
      <div className="flex flex-1 items-center justify-between gap-4 px-5 py-3">
        <div className="min-w-0 flex-1">
          <h3 className="mb-1 truncate text-sm font-semibold text-on-surface">
            {t(video.titleKey)}
          </h3>
          <span className="text-[10px] text-on-surface-variant">
            {video.date}
          </span>
        </div>
        <div className="flex items-center gap-3">
          <span
            className={`rounded px-2.5 py-1 text-[10px] font-semibold uppercase tracking-wider ${CATEGORY_COLORS[video.category]}`}
          >
            {t(CATEGORY_I18N[video.category])}
          </span>
          <Icon
            name={video.icon}
            className="text-lg text-on-surface-variant"
          />
        </div>
      </div>
    </div>
  );
}

/* ──────────────────────────────────────────────
   Main Page
   ────────────────────────────────────────────── */

function VideoLibrary() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [search, setSearch] = useState("");
  const [activeCategory, setActiveCategory] = useState<Category | "All">("All");
  const [viewMode, setViewMode] = useState<"grid" | "list">("grid");

  const filtered = useMemo(() => {
    return VIDEOS.filter((v) => {
      const matchesSearch =
        search === "" ||
        t(v.titleKey).toLowerCase().includes(search.toLowerCase());
      const matchesCategory =
        activeCategory === "All" || v.category === activeCategory;
      return matchesSearch && matchesCategory;
    });
  }, [search, activeCategory, t]);

  return (
    <div className="min-h-screen space-y-6 bg-surface px-6 py-5 font-body">
      {/* Breadcrumb */}
      <nav
        aria-label="Breadcrumb"
        className="flex items-center gap-1.5 text-xs uppercase tracking-widest text-on-surface-variant"
      >
        {[t('video_library.breadcrumb_resources'), t('video_library.breadcrumb_video_library')].map((crumb, i, arr) => (
          <span key={crumb} className="flex items-center gap-1.5">
            <span className="cursor-pointer transition-colors hover:text-primary">
              {crumb}
            </span>
            {i < arr.length - 1 && (
              <Icon name="chevron_right" className="text-[14px] opacity-40" />
            )}
          </span>
        ))}
      </nav>

      {/* Title */}
      <div>
        <h1 className="font-headline text-2xl font-bold text-on-surface">
          {t('video_library.title')}
        </h1>
        <p className="mt-1 text-sm text-on-surface-variant">
          {t('video_library.subtitle')}
        </p>
      </div>

      {/* Search + Filter bar */}
      <div className="flex flex-col gap-4 rounded-lg bg-surface-container p-4 sm:flex-row sm:items-center sm:justify-between">
        {/* Search */}
        <div className="relative flex-1 sm:max-w-sm">
          <Icon
            name="search"
            className="absolute left-3 top-1/2 -translate-y-1/2 text-lg text-on-surface-variant"
          />
          <input
            type="text"
            placeholder={t('video_library.search_placeholder')}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full rounded-md bg-surface-container-low py-2 pl-10 pr-4 text-sm text-on-surface placeholder-on-surface-variant/50 outline-none transition-colors focus:bg-surface-container-high"
          />
        </div>

        {/* View toggle */}
        <div className="flex gap-1">
          <button
            type="button"
            onClick={() => setViewMode("grid")}
            className={`flex items-center gap-1.5 rounded-md px-3 py-2 text-xs font-semibold uppercase tracking-wider transition-colors ${
              viewMode === "grid"
                ? "bg-primary text-on-primary-container"
                : "bg-surface-container-low text-on-surface-variant hover:bg-surface-container-high"
            }`}
            aria-label="Grid view"
          >
            <Icon name="grid_view" className="text-base" />
            {t('common.grid')}
          </button>
          <button
            type="button"
            onClick={() => setViewMode("list")}
            className={`flex items-center gap-1.5 rounded-md px-3 py-2 text-xs font-semibold uppercase tracking-wider transition-colors ${
              viewMode === "list"
                ? "bg-primary text-on-primary-container"
                : "bg-surface-container-low text-on-surface-variant hover:bg-surface-container-high"
            }`}
            aria-label="List view"
          >
            <Icon name="view_list" className="text-base" />
            {t('common.list')}
          </button>
        </div>
      </div>

      {/* Category filter chips */}
      <div className="flex flex-wrap gap-2">
        <button
          type="button"
          onClick={() => setActiveCategory("All")}
          className={`rounded-full px-4 py-1.5 text-xs font-semibold uppercase tracking-wider transition-colors ${
            activeCategory === "All"
              ? "bg-primary text-on-primary-container"
              : "bg-surface-container text-on-surface-variant hover:bg-surface-container-high"
          }`}
        >
          {t('common.all')}
        </button>
        {CATEGORIES.map((cat) => (
          <button
            key={cat}
            type="button"
            onClick={() => setActiveCategory(cat)}
            className={`rounded-full px-4 py-1.5 text-xs font-semibold uppercase tracking-wider transition-colors ${
              activeCategory === cat
                ? "bg-primary text-on-primary-container"
                : "bg-surface-container text-on-surface-variant hover:bg-surface-container-high"
            }`}
          >
            {t(CATEGORY_I18N[cat])}
          </button>
        ))}
      </div>

      {/* Results count */}
      <p className="text-xs text-on-surface-variant">
        {t('video_library.showing_of_videos', { shown: filtered.length, total: VIDEOS.length })}
      </p>

      {/* Video grid / list */}
      {filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-lg bg-surface-container py-16">
          <Icon
            name="video_library"
            className="mb-3 text-4xl text-on-surface-variant/30"
          />
          <p className="text-sm text-on-surface-variant">
            {t('video_library.no_videos_match')}
          </p>
        </div>
      ) : viewMode === "grid" ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {filtered.map((video, i) => (
            <div key={video.id} onClick={() => navigate(`/help/videos/player?v=${video.id}`)} className="cursor-pointer">
              <VideoCardGrid video={video} index={i} />
            </div>
          ))}
        </div>
      ) : (
        <div className="space-y-3">
          {filtered.map((video, i) => (
            <div key={video.id} onClick={() => navigate(`/help/videos/player?v=${video.id}`)} className="cursor-pointer">
              <VideoCardList video={video} index={i} />
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

export default memo(VideoLibrary);
