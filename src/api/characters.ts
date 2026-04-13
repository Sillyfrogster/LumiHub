import type { LumiHubCharacter, CharacterImage } from '../types/character';
import { apiFetch } from './client';

const BASE = '/api/v1/characters';

interface PaginatedResponse {
  data: LumiHubCharacter[];
  pagination: {
    page: number;
    limit: number;
    total: number;
    totalPages: number;
  };
}

interface SingleResponse {
  data: LumiHubCharacter;
}

interface CreateResponse {
  id: string;
  message: string;
}

export interface ListParams {
  page?: number;
  limit?: number;
  sort?: string;
  order?: 'asc' | 'desc';
  search?: string;
  tags?: string;
  ownerId?: string;
}

/** Builds a query string from the given list parameters. */
function toQuery(params: ListParams): string {
  const qs = new URLSearchParams();
  if (params.page) qs.set('page', String(params.page));
  if (params.limit) qs.set('limit', String(params.limit));
  if (params.sort) qs.set('sort', params.sort);
  if (params.order) qs.set('order', params.order);
  if (params.search) qs.set('search', params.search);
  if (params.tags) qs.set('tags', params.tags);
  if (params.ownerId) qs.set('ownerId', params.ownerId);
  return qs.toString();
}

/** Fetches a paginated list of characters from the backend. */
export async function listCharacters(params: ListParams = {}): Promise<PaginatedResponse> {
  const res = await fetch(`${BASE}?${toQuery(params)}`);
  if (!res.ok) throw new Error(`Failed to list characters: ${res.status}`);
  return res.json();
}

/** Fetches a single character by its UUID. */
export async function getCharacter(id: string): Promise<SingleResponse> {
  const res = await fetch(`${BASE}/${id}`);
  if (!res.ok) throw new Error(`Failed to get character: ${res.status}`);
  return res.json();
}

/** Creates a new character via multipart form-data. */
export async function createCharacter(
  data: Record<string, unknown>,
  image?: File,
): Promise<CreateResponse> {
  const form = new FormData();
  form.append('character_data', JSON.stringify(data));
  if (image) form.append('image', image);

  const res = await apiFetch(BASE, { method: 'POST', body: form });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.message || `Failed to create character: ${res.status}`);
  }
  return res.json();
}

/** Updates an existing character by UUID. */
export async function updateCharacter(
  id: string,
  data: Record<string, unknown>,
  image?: File,
): Promise<{ data: LumiHubCharacter; message: string }> {
  const form = new FormData();
  form.append('character_data', JSON.stringify(data));
  if (image) form.append('image', image);

  const res = await apiFetch(`${BASE}/${id}`, { method: 'PUT', body: form });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.message || `Failed to update character: ${res.status}`);
  }
  return res.json();
}

/** Deletes a character by UUID. */
export async function deleteCharacter(id: string): Promise<void> {
  const res = await apiFetch(`${BASE}/${id}`, { method: 'DELETE' });
  if (!res.ok) throw new Error(`Failed to delete character: ${res.status}`);
}

/** Creates a character from a .charx file upload. */
export async function createCharacterFromCharx(
  data: Record<string, unknown>,
  charxFile: File,
): Promise<CreateResponse> {
  const form = new FormData();
  form.append('character_data', JSON.stringify(data));
  form.append('charx_file', charxFile);

  const res = await apiFetch(`${BASE}/charx`, { method: 'POST', body: form });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.message || `Failed to import .charx: ${res.status}`);
  }
  return res.json();
}

/** Fetches all images for a character. */
export async function getCharacterImages(id: string): Promise<CharacterImage[]> {
  const res = await fetch(`${BASE}/${id}/images`);
  if (!res.ok) return [];
  const json = await res.json();
  return json.data;
}

/** Increments the download counter and returns the new count. */
export async function downloadCharacter(id: string): Promise<{ downloads: number }> {
  const res = await fetch(`${BASE}/${id}/download`, { method: 'POST' });
  if (!res.ok) throw new Error(`Failed to record download: ${res.status}`);
  return res.json();
}

/** Increments the view counter (best-effort, no auth required). */
export async function viewCharacter(id: string): Promise<void> {
  fetch(`${BASE}/${id}/view`, { method: 'POST' }).catch(() => {});
}

/** Fetches distinct tags used across all LumiHub characters. */
export async function listCharacterTags(search?: string): Promise<{ name: string; count: number }[]> {
  const qs = new URLSearchParams();
  if (search) qs.set('search', search);
  const res = await fetch(`${BASE}/tags?${qs}`);
  if (!res.ok) return [];
  const json = await res.json();
  return json.tags ?? [];
}
