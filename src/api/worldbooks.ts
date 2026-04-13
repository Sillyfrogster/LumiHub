import { apiFetch } from './client';

const BASE = '/api/v1/worldbooks';

export interface WorldbookListParams {
  page?: number;
  limit?: number;
  sort?: string;
  order?: string;
  search?: string;
  tags?: string;
  ownerId?: string;
}

export async function listWorldbooks(params: WorldbookListParams = {}) {
  const query = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined) query.set(k, String(v));
  }
  const res = await fetch(`${BASE}?${query}`);
  if (!res.ok) throw new Error(`Failed to list worldbooks: ${res.status}`);
  return res.json();
}

export async function getWorldbook(id: string) {
  const res = await fetch(`${BASE}/${id}`);
  if (!res.ok) throw new Error(`Failed to fetch worldbook: ${res.status}`);
  return res.json();
}

export async function createWorldbook(
  file: File,
  meta: { name?: string; description?: string; tags?: string },
  image?: File,
): Promise<{ id: string; message: string; entryCount: number }> {
  const form = new FormData();
  form.append('worldbook_file', file);
  if (meta.name) form.append('name', meta.name);
  if (meta.description) form.append('description', meta.description);
  if (meta.tags) form.append('tags', meta.tags);
  if (image) form.append('image', image);

  const res = await apiFetch(BASE, { method: 'POST', body: form });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.message || `Failed to create worldbook: ${res.status}`);
  }
  return res.json();
}

export async function deleteWorldbook(id: string): Promise<void> {
  const res = await apiFetch(`${BASE}/${id}`, { method: 'DELETE' });
  if (!res.ok) throw new Error(`Failed to delete worldbook: ${res.status}`);
}

/** Increments the view counter (best-effort, no auth required). */
export async function viewWorldbook(id: string): Promise<void> {
  fetch(`${BASE}/${id}/view`, { method: 'POST' }).catch(() => {});
}

export async function exportWorldbook(id: string) {
  const res = await fetch(`${BASE}/${id}/export`);
  if (!res.ok) throw new Error(`Failed to export worldbook: ${res.status}`);
  return res.json();
}

/** Fetches distinct tags used across all LumiHub worldbooks. */
export async function listWorldbookTags(search?: string): Promise<{ name: string; count: number }[]> {
  const qs = new URLSearchParams();
  if (search) qs.set('search', search);
  const res = await fetch(`${BASE}/tags?${qs}`);
  if (!res.ok) return [];
  const json = await res.json();
  return json.tags ?? [];
}
