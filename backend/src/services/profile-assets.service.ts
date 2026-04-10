import path from 'node:path';
import { mkdir, unlink } from 'node:fs/promises';
import { logger } from '../utils/logger.ts';
import { UPLOAD_PATHS } from '../utils/constants.ts';
import sharp from 'sharp';

export async function processProfileImage(buffer: Buffer): Promise<{ filePath: string, size: number }> {
    const filename = `${crypto.randomUUID()}.webp`;
    const dir = path.resolve(UPLOAD_PATHS.PROFILE_ASSETS);
    await mkdir(dir, { recursive: true });

    const fullPath = path.join(dir, filename);

    // Convert to webp, resize if too large
    const info = await sharp(buffer)
        .resize({ width: 1920, height: 1080, fit: 'inside', withoutEnlargement: true })
        .webp({ quality: 80 })
        .toFile(fullPath);

    return {
        filePath: `${UPLOAD_PATHS.PROFILE_ASSETS}/${filename}`,
        size: info.size
    };
}

export async function processProfileVideo(tempFilePath: string): Promise<{ filePath: string, size: number }> {
    const filename = `${crypto.randomUUID()}.webm`;
    const dir = path.resolve(UPLOAD_PATHS.PROFILE_ASSETS);
    await mkdir(dir, { recursive: true });
    
    const fullPath = path.join(dir, filename);

    return new Promise((resolve, reject) => {
        // this was just a test, ffmpeg-static needs to be added.
        const proc = Bun.spawn([
            'ffmpeg',
            '-i', tempFilePath,
            '-c:v', 'libvpx-vp9',
            '-crf', '30',
            '-b:v', '1M',
            '-an',
            '-vf', 'scale=\'min(1920,iw)\':\'min(1080,ih)\':force_original_aspect_ratio=decrease',
            '-y',
            fullPath
        ], {
            stdout: 'ignore',
            stderr: 'pipe',
        });

        proc.exited.then(async (code) => {
            try {
                await unlink(tempFilePath);
            } catch (e) {
                logger.error(`Failed to delete temp video file: ${tempFilePath}`, e);
            }

            if (code === 0) {
                const file = Bun.file(fullPath);
                resolve({
                    filePath: `${UPLOAD_PATHS.PROFILE_ASSETS}/${filename}`,
                    size: file.size
                });
            } else {
                let errText = '';
                try {
                     errText = await new Response(proc.stderr).text();
                } catch(e) {}
                logger.error(`FFmpeg failed with code ${code}. ${errText}`);
                reject(new Error(`Video conversion failed with code ${code}`));
            }
        });
    });
}

export async function deleteAssetFile(relativePath: string) {
    if (!relativePath.startsWith(UPLOAD_PATHS.PROFILE_ASSETS)) return;
    const fullPath = resolveUploadPath(relativePath);
    if (!fullPath) {
        logger.error(`Refusing to delete path outside uploads root: ${relativePath}`);
        return;
    }
    try {
        await unlink(fullPath);
    } catch (e) {
        logger.error(`Failed to delete asset file: ${fullPath}`, e);
    }
}

function resolveUploadPath(relativePath: string): string | null {
    const uploadsRoot = path.resolve(UPLOAD_PATHS.ROOT);
    const normalized = relativePath.replace(/^\/+/, '');
    const absolute = path.resolve(uploadsRoot, normalized.startsWith(`${UPLOAD_PATHS.ROOT}/`)
        ? normalized.slice(UPLOAD_PATHS.ROOT.length + 1)
        : normalized);

    if (absolute === uploadsRoot || absolute.startsWith(`${uploadsRoot}${path.sep}`)) {
        return absolute;
    }

    return null;
}
