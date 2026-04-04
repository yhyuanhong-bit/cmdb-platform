import { memo, useState, useMemo } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";

/* ──────────────────────────────────────────────
   Mock data
   ────────────────────────────────────────────── */

type Category = "Surveillance" | "Training" | "Security" | "Audit" | "Testing";

interface Video {
  id: number;
  title: string;
  duration: string;
  date: string;
  category: Category;
  icon: string;
}

const VIDEOS: Video[] = [
  {
    id: 1,
    title: "Rack A01 - Thermal Event 10/24",
    duration: "02:34",
    date: "2025-10-24",
    category: "Surveillance",
    icon: "videocam",
  },
  {
    id: 2,
    title: "Quarterly Maintenance SOP",
    duration: "15:42",
    date: "2025-11-02",
    category: "Training",
    icon: "school",
  },
  {
    id: 3,
    title: "Server Room B - Access Log",
    duration: "01:15",
    date: "2025-12-08",
    category: "Security",
    icon: "shield",
  },
  {
    id: 4,
    title: "Emergency Procedure Walkthrough",
    duration: "08:20",
    date: "2026-01-15",
    category: "Training",
    icon: "school",
  },
  {
    id: 5,
    title: "Network Closet Inspection",
    duration: "03:45",
    date: "2026-02-20",
    category: "Audit",
    icon: "fact_check",
  },
  {
    id: 6,
    title: "UPS Failover Test Recording",
    duration: "05:12",
    date: "2026-03-10",
    category: "Testing",
    icon: "science",
  },
];

const CATEGORIES: Category[] = [
  "Surveillance",
  "Training",
  "Security",
  "Audit",
  "Testing",
];

const CATEGORY_COLORS: Record<Category, string> = {
  Surveillance: "bg-[#1e3a5f] text-primary",
  Training: "bg-[#064e3b] text-[#34d399]",
  Security: "bg-[#7f1d1d] text-[#ff6b6b]",
  Audit: "bg-[#92400e] text-[#fbbf24]",
  Testing: "bg-[#3b0764] text-[#c084fc]",
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

function VideoCardGrid({ video, index }: { video: Video; index: number }) {
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
          {video.title}
        </h3>
        <div className="flex items-center justify-between">
          <span className="text-[10px] text-on-surface-variant">
            {video.date}
          </span>
          <span
            className={`rounded px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider ${CATEGORY_COLORS[video.category]}`}
          >
            {video.category}
          </span>
        </div>
      </div>
    </div>
  );
}

function VideoCardList({ video, index }: { video: Video; index: number }) {
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
            {video.title}
          </h3>
          <span className="text-[10px] text-on-surface-variant">
            {video.date}
          </span>
        </div>
        <div className="flex items-center gap-3">
          <span
            className={`rounded px-2.5 py-1 text-[10px] font-semibold uppercase tracking-wider ${CATEGORY_COLORS[video.category]}`}
          >
            {video.category}
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
        v.title.toLowerCase().includes(search.toLowerCase());
      const matchesCategory =
        activeCategory === "All" || v.category === activeCategory;
      return matchesSearch && matchesCategory;
    });
  }, [search, activeCategory]);

  return (
    <div className="min-h-screen space-y-6 bg-surface px-6 py-5 font-body">
      {/* Breadcrumb */}
      <nav
        aria-label="Breadcrumb"
        className="flex items-center gap-1.5 text-xs uppercase tracking-widest text-on-surface-variant"
      >
        {["RESOURCES", "VIDEO_LIBRARY"].map((crumb, i, arr) => (
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
            {cat}
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
            <div key={video.id} onClick={() => navigate('/help/videos/player')} className="cursor-pointer">
              <VideoCardGrid video={video} index={i} />
            </div>
          ))}
        </div>
      ) : (
        <div className="space-y-3">
          {filtered.map((video, i) => (
            <div key={video.id} onClick={() => navigate('/help/videos/player')} className="cursor-pointer">
              <VideoCardList video={video} index={i} />
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

export default memo(VideoLibrary);
