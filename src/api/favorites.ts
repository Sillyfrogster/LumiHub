import { apiFetch } from './client';

export type FavoriteAssetType = 'character' | 'worldbook';

export interface ToggleFavoriteResult {
  favorited: boolean;
  favorites: number;
}

/** Toggles the current user's favorite on an asset. Returns updated state. */
export async function toggleFavorite(
  assetType: FavoriteAssetType,
  assetId: string,
): Promise<ToggleFavoriteResult> {
  const res = await apiFetch('/api/v1/favorites/toggle', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ assetType, assetId }),
  });
  if (!res.ok) throw new Error(`Failed to toggle favorite: ${res.status}`);
  return res.json();
}

/** Checks whether the current user has favorited a given asset. */
export async function checkFavorite(
  assetType: FavoriteAssetType,
  assetId: string,
): Promise<boolean> {
  const qs = new URLSearchParams({ assetType, assetId });
  const res = await apiFetch(`/api/v1/favorites/check?${qs}`);
  if (!res.ok) return false;
  const json = await res.json();
  return json.favorited as boolean;
}
