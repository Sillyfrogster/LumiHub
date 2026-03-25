export const FILE_SIZE_LIMITS = {
  IMAGE: 5 * 1024 * 1024,
  CHARACTER_CARD: 10 * 1024 * 1024,
  CHARX: 50 * 1024 * 1024,
  WORLDBOOK: 5 * 1024 * 1024,
  THEME: 2 * 1024 * 1024,
  PRESET: 1 * 1024 * 1024,
  PACK: 10 * 1024 * 1024,
} as const;

export const ALLOWED_MIME_TYPES = {
  IMAGE: ['image/png', 'image/jpeg', 'image/gif', 'image/webp', 'image/avif'],
  JSON: ['application/json'],
  CSS: ['text/css'],
} as const;

export const UPLOAD_PATHS = {
  ROOT: 'uploads',
  CHARACTERS: 'uploads/characters',
  CHARACTER_IMAGES: 'uploads/characters/images',
  WORLDBOOKS: 'uploads/worldbooks',
  THEMES: 'uploads/themes',
  PRESETS: 'uploads/presets',
} as const;

export const NSFW_CLASSES = {
  DRAWING: 'Drawing',
  HENTAI: 'Hentai',
  NEUTRAL: 'Neutral',
  PORN: 'Porn',
  SEXY: 'Sexy',
} as const;

export const SORT_OPTIONS = {
  CREATED_AT: 'created_at',
  UPDATED_AT: 'updated_at',
  DOWNLOADS: 'downloads',
  VIEWS: 'views',
  RATING: 'rating',
  NAME: 'name',
} as const;
