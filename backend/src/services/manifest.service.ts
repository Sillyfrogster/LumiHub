/**
 * Install Manifest Service — manages the per-instance manifest of installed
 * characters and world books. Synced from Lumiverse instances via WebSocket.
 */
import { AppDataSource } from '../db/connection.ts';
import { InstallManifest } from '../entities/InstallManifest.entity.ts';
import { IsNull } from 'typeorm';

const repo = () => AppDataSource.getRepository(InstallManifest);

export interface ManifestEntry {
    slug: string;
    type: 'character' | 'worldbook';
    name: string;
    creator: string;
    source: string;
    installed_at: number | null;
}

/**
 * Full sync: replace all manifest entries for an instance with the provided list.
 * Uses upsert on (instance_id, slug) and deletes entries no longer present.
 */
export async function syncManifest(instanceId: string, entries: ManifestEntry[]): Promise<void> {
    const repository = repo();

    // Build the set of incoming slugs (keyed on slug alone to match the DB unique constraint)
    const incomingSlugs = new Set(entries.map((e) => e.slug));

    // Delete entries that are no longer in the manifest
    const existing = await repository.find({ where: { instance_id: instanceId } });
    const toDelete = existing.filter((e) => !incomingSlugs.has(e.slug));
    if (toDelete.length > 0) {
        await repository.remove(toDelete);
    }

    // Deduplicate by slug — the DB constraint is (instance_id, slug) without entry_type,
    // so two entries with the same slug (e.g. duplicate characters, or a character and
    // worldbook sharing a creator/name) would violate the constraint. Last entry wins.
    if (entries.length > 0) {
        const deduped = new Map<string, ManifestEntry>();
        for (const e of entries) {
            deduped.set(e.slug, e);
        }

        const upsertRows = [...deduped.values()].map((e) => ({
            instance_id: instanceId,
            slug: e.slug,
            entry_type: e.type,
            character_name: e.name,
            creator_name: e.creator,
            source: e.source,
            installed_at: e.installed_at,
            synced_at: new Date(),
        }));

        await repository.upsert(upsertRows, ['instance_id', 'slug']);
    }
}

/** Get the full manifest for an instance, optionally filtered by type. */
export async function getManifest(instanceId: string, type?: 'character' | 'worldbook'): Promise<ManifestEntry[]> {
    const where: any = { instance_id: instanceId };
    if (type) where.entry_type = type;
    const rows = await repo().find({ where });
    return rows.map((r) => ({
        slug: r.slug,
        type: r.entry_type as 'character' | 'worldbook',
        name: r.character_name,
        creator: r.creator_name,
        source: r.source,
        installed_at: r.installed_at,
    }));
}

/** Check if a specific slug is installed on an instance. */
export async function checkSlug(instanceId: string, slug: string): Promise<ManifestEntry | null> {
    const row = await repo().findOne({ where: { instance_id: instanceId, slug } });
    if (!row) return null;
    return {
        slug: row.slug,
        type: row.entry_type as 'character' | 'worldbook',
        name: row.character_name,
        creator: row.creator_name,
        source: row.source,
        installed_at: row.installed_at,
    };
}

/** Get the manifest for the first linked instance belonging to a user. */
export async function getManifestForUser(userId: string, type?: 'character' | 'worldbook'): Promise<{ instanceId: string; entries: ManifestEntry[] } | null> {
    const { LinkedInstance } = await import('../entities/LinkedInstance.entity.ts');
    const instanceRepo = AppDataSource.getRepository(LinkedInstance);

    const instances = await instanceRepo.find({
        where: { user_id: userId, revoked_at: IsNull() },
        order: { created_at: 'DESC' },
    });

    if (instances.length === 0) return null;

    const instance = instances[0];
    const entries = await getManifest(instance.id, type);
    return { instanceId: instance.id, entries };
}
