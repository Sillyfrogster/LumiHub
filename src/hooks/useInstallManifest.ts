import { useQuery } from '@tanstack/react-query';
import { useAuth } from './useAuth';
import type { UnifiedCharacterCard } from '../types/character';
import type { UnifiedWorldBook } from '../types/worldbook';

export interface ManifestEntry {
  slug: string;
  type: 'character' | 'worldbook';
  name: string;
  creator: string;
  source: string;
  installed_at: number | null;
}

interface ManifestResponse {
  entries: ManifestEntry[];
  instance_id: string | null;
}

/** Slugify a string: lowercase, replace non-alphanumeric with hyphens, collapse. */
function slugify(str: string): string {
  return str
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9-]/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '');
}

/** Build a `creator/name` slug from a character card. */
export function getCardSlug(card: UnifiedCharacterCard): string {
  if (card.source === 'chub') {
    // Chub cards already use creator/slug format as their ID
    return card.id.toLowerCase();
  }
  const creator = slugify(card.creator || 'unknown');
  const name = slugify(card.name || 'unnamed');
  return `${creator}/${name}`;
}

/** Build a `creator/name` slug from a world book. */
export function getWorldBookSlug(book: UnifiedWorldBook): string {
  if (book.source === 'chub') {
    return book.id.toLowerCase();
  }
  const creator = slugify(book.creator || 'unknown');
  const name = slugify(book.name || 'unnamed');
  return `${creator}/${name}`;
}

// ── Fuzzy slug matching ───────────────────────────────────────────────

/**
 * High-confidence guess: checks if a manifest entry likely represents the same card.
 * Both conditions must pass:
 *   1. Creator slugs match exactly
 *   2. One name slug includes the other (handles hex suffixes, truncated names, etc.)
 */
function isLikelySlugMatch(entrySlug: string, cardCreator: string, cardName: string): boolean {
  const slashIdx = entrySlug.indexOf('/');
  if (slashIdx === -1) return false;

  const entryCreator = entrySlug.slice(0, slashIdx);
  const entryName = entrySlug.slice(slashIdx + 1);

  const creator = slugify(cardCreator || 'unknown');
  const name = slugify(cardName || 'unnamed');

  if (entryCreator !== creator) return false;
  // Require the shorter side to be at least 3 chars to avoid false positives
  if (entryName.length < 3 || name.length < 3) return false;
  return entryName.includes(name) || name.includes(entryName);
}

// ── Dismissed guess tracking ──────────────────────────────────────────

const DISMISSED_KEY = 'lumihub:dismissed-install-guesses';

function getDismissedGuesses(): Set<string> {
  try {
    const raw = localStorage.getItem(DISMISSED_KEY);
    return raw ? new Set(JSON.parse(raw)) : new Set();
  } catch {
    return new Set();
  }
}

/** Persist a dismissed guess so the user won't see it again. */
export function dismissInstallGuess(slug: string): void {
  const set = getDismissedGuesses();
  set.add(slug);
  localStorage.setItem(DISMISSED_KEY, JSON.stringify([...set]));
}

// ── Fetching ──────────────────────────────────────────────────────────

async function fetchManifest(): Promise<ManifestResponse> {
  const res = await fetch('/api/v1/link/manifest', { credentials: 'include' });
  if (!res.ok) return { entries: [], instance_id: null };
  return res.json();
}

/** Fetch the full install manifest for the user's linked instance. */
export function useInstallManifest() {
  const { isAuthenticated } = useAuth();

  return useQuery({
    queryKey: ['link', 'manifest'],
    queryFn: fetchManifest,
    enabled: isAuthenticated,
    staleTime: 1000 * 60,
    refetchInterval: 1000 * 60,
  });
}

/** Check if a specific character card is installed on the user's Lumiverse instance. */
export function useIsInstalled(card?: UnifiedCharacterCard): { isInstalled: boolean; isGuess: boolean; entry: ManifestEntry | null } {
  const { data } = useInstallManifest();
  if (!card || !data?.entries.length) return { isInstalled: false, isGuess: false, entry: null };

  const slug = getCardSlug(card);
  const entry = data.entries.find((e) => e.slug === slug && e.type === 'character') ?? null;
  if (entry) return { isInstalled: true, isGuess: false, entry };

  // Fuzzy fallback: creator must match exactly, card name must be included in the entry
  // (or vice versa). Handles hex suffixes in Chub slugs, name truncation, etc.
  if (card.source === 'chub' && !getDismissedGuesses().has(slug)) {
    const guessEntry = data.entries.find(
      (e) => e.type === 'character' && isLikelySlugMatch(e.slug, card.creator, card.name),
    ) ?? null;
    if (guessEntry) {
      return { isInstalled: true, isGuess: true, entry: guessEntry };
    }
  }

  return { isInstalled: false, isGuess: false, entry: null };
}

/** Check if a specific world book is installed on the user's Lumiverse instance. */
export function useIsWorldBookInstalled(book?: UnifiedWorldBook): { isInstalled: boolean; isGuess: boolean; entry: ManifestEntry | null } {
  const { data } = useInstallManifest();
  if (!book || !data?.entries.length) return { isInstalled: false, isGuess: false, entry: null };

  const slug = getWorldBookSlug(book);
  const entry = data.entries.find((e) => e.slug === slug && e.type === 'worldbook') ?? null;
  if (entry) return { isInstalled: true, isGuess: false, entry };

  if (book.source === 'chub' && !getDismissedGuesses().has(slug)) {
    const guessEntry = data.entries.find(
      (e) => e.type === 'worldbook' && isLikelySlugMatch(e.slug, book.creator, book.name),
    ) ?? null;
    if (guessEntry) {
      return { isInstalled: true, isGuess: true, entry: guessEntry };
    }
  }

  return { isInstalled: false, isGuess: false, entry: null };
}
