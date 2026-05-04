import crypto from 'node:crypto';
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { ALLOWED_MIME_TYPES, FILE_SIZE_LIMITS } from '../utils/constants.ts';

export interface ApiErrorPayload {
  error: string;
  message: string;
  statusCode: number;
}

export type IntakeResult<T> = T | { error: ApiErrorPayload };

export function isIntakeError<T>(result: IntakeResult<T>): result is { error: ApiErrorPayload } {
  return typeof result === 'object' && result !== null && 'error' in result;
}

export async function parseJsonImport<T>(
  formData: FormData,
  options: {
    fileField: string;
    dataField: string;
    fileLimit: number;
    fileLimitMessage: string;
    missingMessage: string;
    invalidMessage: string;
    normalize: (input: unknown) => T;
  },
): Promise<IntakeResult<T>> {
  const jsonFile = formData.get(options.fileField);
  const inlineData = formData.get(options.dataField);

  let raw: string | null = null;
  if (jsonFile instanceof File && jsonFile.size > 0) {
    if (jsonFile.size > options.fileLimit) {
      return badRequest(options.fileLimitMessage);
    }
    raw = await jsonFile.text();
  } else if (typeof inlineData === 'string') {
    raw = inlineData;
  }

  if (!raw) {
    return badRequest(options.missingMessage);
  }

  try {
    return options.normalize(JSON.parse(raw));
  } catch (err: any) {
    return badRequest(err.message || options.invalidMessage);
  }
}

export function readString(formData: FormData, key: string): string | null {
  const value = formData.get(key);
  return typeof value === 'string' ? value : null;
}

export function readTags(formData: FormData): string[] {
  const raw = readString(formData, 'tags');
  return raw ? raw.split(',').map((tag) => tag.trim()).filter(Boolean) : [];
}

export async function savePreviewImage(
  file: File | string | null,
  dir: string,
): Promise<IntakeResult<{ path: string | null }>> {
  if (!(file instanceof File) || file.size === 0) return { path: null };
  if (file.size > FILE_SIZE_LIMITS.IMAGE) {
    return badRequest('Image exceeds 5MB limit');
  }
  if (!ALLOWED_MIME_TYPES.IMAGE.includes(file.type as any)) {
    return badRequest('Unsupported image type');
  }

  const ext = file.name?.split('.').pop()?.toLowerCase() || 'png';
  const filename = `${crypto.randomUUID()}.${ext}`;
  await mkdir(dir, { recursive: true });
  const fullPath = path.join(dir, filename);
  await writeFile(fullPath, Buffer.from(await file.arrayBuffer()));
  return { path: fullPath };
}

function badRequest(message: string): { error: ApiErrorPayload } {
  return {
    error: {
      error: 'Bad Request',
      message,
      statusCode: 400,
    },
  };
}
