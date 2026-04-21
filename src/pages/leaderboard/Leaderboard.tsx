import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { Trophy, Download, Heart, Users, Book, Crown, Eye } from 'lucide-react';
import {
  getLeaderboard,
  type LeaderboardType,
  type LeaderboardMetric,
  type LeaderboardPeriod,
  type AssetLeaderboardEntry,
  type CreatorLeaderboardEntry,
} from '../../api/leaderboard';
import { toUploadUrl } from '../../utils/media';
import styles from './Leaderboard.module.css';

const TYPE_OPTIONS: { key: LeaderboardType; label: string; icon: React.ReactNode }[] = [
  { key: 'characters', label: 'Characters', icon: <Users size={15} /> },
  { key: 'worldbooks', label: 'Worldbooks', icon: <Book size={15} /> },
  { key: 'creators', label: 'Creators', icon: <Crown size={15} /> },
];

const PERIOD_OPTIONS: { key: LeaderboardPeriod; label: string }[] = [
  { key: 'week', label: 'This Week' },
  { key: 'month', label: 'This Month' },
  { key: 'all', label: 'All Time' },
];

const METRIC_OPTIONS: { key: LeaderboardMetric; label: string; icon: React.ReactNode }[] = [
  { key: 'downloads', label: 'Downloads', icon: <Download size={13} /> },
  { key: 'favorites', label: 'Favorites', icon: <Heart size={13} /> },
];

const RANK_CLASS = ['rankGold', 'rankSilver', 'rankBronze'] as const;

const fmt = (n: number) => (n > 1000 ? `${(n / 1000).toFixed(1)}k` : String(n));

function SkeletonRow() {
  return <div className={styles.skeletonRow}><div className={styles.skeletonShimmer} /></div>;
}

const Leaderboard: React.FC = () => {
  const [type, setType] = useState<LeaderboardType>('characters');
  const [period, setPeriod] = useState<LeaderboardPeriod>('all');
  const [metric, setMetric] = useState<LeaderboardMetric>('downloads');

  const { data, isLoading } = useQuery({
    queryKey: ['leaderboard', type, metric, period],
    queryFn: () => getLeaderboard(type, metric, period, 25),
    staleTime: 1000 * 60 * 5,
  });

  const entries = data ?? [];

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <Trophy size={24} className={styles.trophyIcon} />
        <h1 className={styles.title}>Leaderboard</h1>
      </div>

      {/* Type tabs */}
      <div className={styles.tabs}>
        {TYPE_OPTIONS.map((opt) => (
          <button
            key={opt.key}
            className={`${styles.tab} ${type === opt.key ? styles.tabActive : ''}`}
            onClick={() => setType(opt.key)}
          >
            {opt.icon}
            {opt.label}
          </button>
        ))}
      </div>

      {/* Period + metric row */}
      <div className={styles.controls}>
        <div className={styles.periodGroup}>
          {PERIOD_OPTIONS.map((opt) => (
            <button
              key={opt.key}
              className={`${styles.chip} ${period === opt.key ? styles.chipActive : ''}`}
              onClick={() => setPeriod(opt.key)}
            >
              {opt.label}
            </button>
          ))}
        </div>

        {type !== 'creators' && (
          <div className={styles.metricGroup}>
            {METRIC_OPTIONS.map((opt) => (
              <button
                key={opt.key}
                className={`${styles.chip} ${metric === opt.key ? styles.chipActive : ''}`}
                onClick={() => setMetric(opt.key)}
              >
                {opt.icon}
                {opt.label}
              </button>
            ))}
          </div>
        )}
      </div>

      {/* List */}
      <div className={styles.list}>
        {isLoading
          ? Array.from({ length: 10 }).map((_, i) => <SkeletonRow key={i} />)
          : entries.length === 0
          ? <p className={styles.empty}>No entries for this period yet.</p>
          : entries.map((entry, i) => {
              const rankClass = i < 3 ? styles[RANK_CLASS[i]] : undefined;

              if (type === 'creators') {
                const c = entry as CreatorLeaderboardEntry;
                const avatarSrc = c.avatar && c.discordId
                  ? `https://cdn.discordapp.com/avatars/${c.discordId}/${c.avatar}.webp?size=64`
                  : null;
                return (
                  <Link
                    key={c.userId}
                    to={c.discordId ? `/user/${c.discordId}` : "/leaderboard"}
                    className={`${styles.row} ${rankClass ?? ''}`}
                  >
                    <span className={styles.rank}>{i + 1}</span>
                    <div className={styles.avatar}>
                      {avatarSrc
                        ? <img src={avatarSrc} alt={c.username} className={styles.avatarImg} />
                        : <div className={styles.avatarFallback}>{c.username.charAt(0).toUpperCase()}</div>
                      }
                    </div>
                    <div className={styles.info}>
                      <span className={styles.name}>{c.displayName || c.username}</span>
                      <span className={styles.sub}>@{c.username} · {c.assetCount} {c.assetCount === 1 ? 'asset' : 'assets'}</span>
                    </div>
                    <div className={styles.stats}>
                      <span className={styles.stat}><Download size={12} />{fmt(c.totalDownloads)}</span>
                      <span className={styles.stat}><Heart size={12} />{fmt(c.totalFavorites)}</span>
                    </div>
                  </Link>
                );
              }

              const a = entry as AssetLeaderboardEntry;
              const assetPath = type === 'characters' ? 'characters' : 'worldbooks';
              const avatarSrc = toUploadUrl(a.avatarUrl);

              return (
                <Link
                  key={a.id}
                  to={`/${assetPath}/${a.id}`}
                  className={`${styles.row} ${rankClass ?? ''}`}
                >
                  <span className={styles.rank}>{i + 1}</span>
                  <div className={styles.avatar}>
                    {avatarSrc
                      ? <img src={avatarSrc} alt={a.name} className={styles.avatarImg} />
                      : <div className={styles.avatarFallback}>{a.name.charAt(0).toUpperCase()}</div>
                    }
                  </div>
                  <div className={styles.info}>
                    <span className={styles.name}>{a.name}</span>
                    {a.creatorUsername && (
                      <span className={styles.sub}>by {a.creatorUsername}</span>
                    )}
                  </div>
                  <div className={styles.stats}>
                    <span className={styles.stat}><Download size={12} />{fmt(a.downloads)}</span>
                    <span className={styles.stat}><Heart size={12} />{fmt(a.favorites)}</span>
                    <span className={styles.stat}><Eye size={12} />{fmt(a.views)}</span>
                  </div>
                </Link>
              );
            })
        }
      </div>
    </div>
  );
};

export default Leaderboard;
