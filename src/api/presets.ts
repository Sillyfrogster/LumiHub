import { apiFetch } from './client';

const BASE = '/api/v1/presets';

export interface PresetListParams {
  page?: number;
  limit?: number;
  sort?: string;
  order?: string;
  search?: string;
  tags?: string;
  ownerId?: string;
}

export async function listPresets(params: PresetListParams = {}) {
  const query = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined) query.set(k, String(v));
  }
  const res = await fetch(`${BASE}?${query}`);
  if (!res.ok) throw new Error(`Failed to list presets: ${res.status}`);
  return res.json();
}

export async function getPreset(id: string) {
  const res = await fetch(`${BASE}/${id}`);
  if (!res.ok) throw new Error(`Failed to fetch preset: ${res.status}`);
  return res.json();
}

export async function createPreset(
  file: File,
  meta: { name?: string; description?: string; tags?: string },
  image?: File,
): Promise<{ id: string; message: string }> {
  const form = new FormData();
  form.append('preset_file', file);
  if (meta.name) form.append('name', meta.name);
  if (meta.description) form.append('description', meta.description);
  if (meta.tags) form.append('tags', meta.tags);
  if (image) form.append('image', image);

  const res = await apiFetch(BASE, { method: 'POST', body: form });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.message || `Failed to create preset: ${res.status}`);
  }
  return res.json();
}

export async function deletePreset(id: string): Promise<void> {
  const res = await apiFetch(`${BASE}/${id}`, { method: 'DELETE' });
  if (!res.ok) throw new Error(`Failed to delete preset: ${res.status}`);
}

/** Increments the view counter (best-effort, no auth required). */
export async function viewPreset(id: string): Promise<void> {
  fetch(`${BASE}/${id}/view`, { method: 'POST' }).catch(() => {});
}

/** Fetches distinct tags used across all presets. */
export async function listPresetTags(search?: string): Promise<{ name: string; count: number }[]> {
  const qs = new URLSearchParams();
  if (search) qs.set('search', search);
  const res = await fetch(`${BASE}/tags?${qs}`);
  if (!res.ok) return [];
  const json = await res.json();
  return json.tags ?? [];
}
