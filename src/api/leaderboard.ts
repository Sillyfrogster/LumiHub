export type LeaderboardType = 'characters' | 'worldbooks' | 'creators';
export type LeaderboardMetric = 'downloads' | 'favorites';
export type LeaderboardPeriod = 'week' | 'month' | 'all';

export interface AssetLeaderboardEntry {
  id: string;
  name: string;
  creator: string;
  creatorUsername: string | null;
  creatorDiscordId: string | null;
  avatarUrl: string | null;
  downloads: number;
  favorites: number;
  views: number;
  createdAt: string;
}

export interface CreatorLeaderboardEntry {
  userId: string;
  discordId: string | null;
  username: string;
  displayName: string | null;
  avatar: string | null;
  totalDownloads: number;
  totalFavorites: number;
  assetCount: number;
}

export type LeaderboardEntry = AssetLeaderboardEntry | CreatorLeaderboardEntry;

export async function getLeaderboard(
  type: LeaderboardType,
  metric: LeaderboardMetric,
  period: LeaderboardPeriod,
  limit = 25,
): Promise<LeaderboardEntry[]> {
  const qs = new URLSearchParams({ type, metric, period, limit: String(limit) });
  const res = await fetch(`/api/v1/leaderboard?${qs}`);
  if (!res.ok) throw new Error(`Failed to fetch leaderboard: ${res.status}`);
  const json = await res.json();
  return json.data as LeaderboardEntry[];
}
