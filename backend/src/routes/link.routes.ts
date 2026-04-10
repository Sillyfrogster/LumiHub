import { Hono } from 'hono';
import { requireAuth, type AuthEnv } from '../middleware/requireAuth.middleware.ts';
import { getCookie } from 'hono/cookie';
import * as LinkService from '../services/link.service.ts';
import * as CharacterService from '../services/characters.service.ts';
import * as WorldbookService from '../services/worldbooks.service.ts';
import { instanceManager } from '../ws/instance-connections.ts';
import { env } from '../env.ts';
import { verifySessionToken } from '../services/auth.service.ts';

const link = new Hono<AuthEnv>();

function escapeHtml(value: string): string {
    return value
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
}

function buildSafeHttpUrl(rawUrl: string, path: string): string | null {
    try {
        const url = new URL(rawUrl);
        if (url.protocol !== 'http:' && url.protocol !== 'https:') {
            return null;
        }
        return new URL(path, url).toString();
    } catch {
        return null;
    }
}

/**
 * PKCE Authorization endpoint.
 * The Lumiverse instance redirects the user's browser here.
 * If the user isn't logged in, redirect to Discord OAuth (then back here).
 */
link.get('/authorize', async (c, next) => {
  const token = getCookie(c, 'lumihub_session');
  if (!token) {
    // Not logged in — redirect to Discord, then back to this URL after login
    const returnUrl = c.req.url;
    return c.redirect(`/api/v1/auth/discord?return_to=${encodeURIComponent(returnUrl)}`);
  }
  try {
    const payload = await verifySessionToken(token);
    c.set('userId', payload.id as string);
    c.set('session', payload);
  } catch {
    // Expired token — redirect to Discord login
    const returnUrl = c.req.url;
    return c.redirect(`/api/v1/auth/discord?return_to=${encodeURIComponent(returnUrl)}`);
  }
  await next();
}, async (c) => {
    const codeChallenge = c.req.query('code_challenge');
    const codeChallengeMethod = c.req.query('code_challenge_method');
    const instanceName = c.req.query('instance_name') || 'My Lumiverse';
    const redirectOrigin = c.req.query('redirect_origin');

    if (!codeChallenge || codeChallengeMethod !== 'S256') {
        return c.json({ error: 'Invalid PKCE parameters' }, 400);
    }
    if (!redirectOrigin) {
        return c.json({ error: 'redirect_origin is required' }, 400);
    }

    const approveHref = buildSafeHttpUrl(redirectOrigin, '/api/v1/lumihub/callback?code=');
    if (!approveHref) {
        return c.json({ error: 'redirect_origin must be a valid http(s) URL' }, 400);
    }

    const userId = c.get('userId');
    const code = await LinkService.createAuthorizationCode(userId, codeChallenge, instanceName, redirectOrigin);
    const callbackHref = `${approveHref}${encodeURIComponent(code)}`;

    // Return an HTML approval page
    const html = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Link Lumiverse Instance — LumiHub</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: #0a0a0f; color: #e0e0e8; display: flex; align-items: center; justify-content: center; min-height: 100vh; }
    .card { background: #14141e; border: 1px solid #2a2a3a; border-radius: 12px; padding: 2rem; max-width: 420px; width: 100%; text-align: center; }
    h1 { font-size: 1.25rem; margin-bottom: 0.5rem; }
    p { color: #9090a8; font-size: 0.9rem; margin-bottom: 1.5rem; }
    .instance-name { color: #a78bfa; font-weight: 600; }
    .actions { display: flex; gap: 0.75rem; justify-content: center; }
    button, a.btn { padding: 0.6rem 1.5rem; border-radius: 8px; font-size: 0.9rem; cursor: pointer; text-decoration: none; border: none; font-weight: 500; }
    .approve { background: #7c3aed; color: white; }
    .approve:hover { background: #6d28d9; }
    .deny { background: #1e1e2e; color: #9090a8; border: 1px solid #2a2a3a; }
    .deny:hover { background: #2a2a3a; }
  </style>
</head>
<body>
  <div class="card">
    <h1>Link Lumiverse Instance</h1>
        <p>Allow <span class="instance-name">${escapeHtml(instanceName)}</span> to connect to your LumiHub account?</p>
    <p>This will let you install characters directly from LumiHub to this Lumiverse instance.</p>
    <div class="actions">
            <a class="btn approve" href="${escapeHtml(callbackHref)}">Approve</a>
      <button class="deny" onclick="window.close()">Deny</button>
    </div>
  </div>
</body>
</html>`;

    return c.html(html);
});

/**
 * Token exchange endpoint (no auth required — the code proves authorization).
 * Lumiverse's backend calls this server-to-server.
 */
link.post('/token', async (c) => {
    const body = await c.req.json();
    const { code, code_verifier } = body;

    if (!code || !code_verifier) {
        return c.json({ error: 'code and code_verifier are required' }, 400);
    }

    const result = await LinkService.exchangeCode(code, code_verifier);
    if (!result) {
        return c.json({ error: 'Invalid or expired authorization code, or PKCE verification failed' }, 400);
    }

    // Construct WS URL
    const publicUrl = env.LUMIHUB_PUBLIC_URL;
    const wsProtocol = publicUrl.startsWith('https') ? 'wss' : 'ws';
    const wsHost = publicUrl.replace(/^https?:\/\//, '');
    const wsUrl = `${wsProtocol}://${wsHost}/api/v1/ws/instance`;

    return c.json({
        token: result.token,
        instance_id: result.instanceId,
        ws_url: wsUrl,
    });
});

/**
 * List linked instances for the current user.
 */
link.get('/instances', requireAuth, async (c) => {
    const userId = c.get('userId');
    const instances = await LinkService.listInstances(userId);
    return c.json({ data: instances });
});

/**
 * Unlink (revoke) an instance.
 */
link.delete('/instances/:id', requireAuth, async (c) => {
    const userId = c.get('userId');
    const instanceId = c.req.param('id');
    const revoked = await LinkService.revokeInstance(instanceId, userId);
    if (!revoked) {
        return c.json({ error: 'Instance not found or already unlinked' }, 404);
    }
    return c.json({ message: 'Instance unlinked' });
});

/**
 * Trigger a remote install to a linked Lumiverse instance.
 */
link.post('/install', requireAuth, async (c) => {
    const userId = c.get('userId');
    const body = await c.req.json();
    const { instance_id, character_id, source, include_worldbook, chub_slug } = body;

    if (!instance_id || !character_id) {
        return c.json({ error: 'instance_id and character_id are required' }, 400);
    }

    // Verify the instance belongs to this user and is online
    const instances = await LinkService.listInstances(userId);
    const instance = instances.find((i) => i.id === instance_id);
    if (!instance) {
        return c.json({ error: 'Instance not found' }, 404);
    }
    if (!instance.is_online) {
        return c.json({ error: 'Instance is offline' }, 503);
    }

    // Build install payload
    let payload: Record<string, unknown>;

    if (source === 'chub') {
        // For Chub characters, send the import URL for Lumiverse to fetch directly
        payload = {
            source: 'chub',
            characterId: character_id,
            characterName: character_id,
            importUrl: `https://chub.ai/characters/${character_id}`,
            importEmbeddedWorldbook: !!include_worldbook,
            chubSlug: (chub_slug || character_id).toLowerCase(),
        };

        // Fetch gallery image URLs if the character has a gallery
        try {
            const chubDetailRes = await fetch(
                `https://gateway.chub.ai/api/characters/${character_id}?full=true`,
                { headers: { 'Accept': 'application/json', 'User-Agent': 'LumiHub' } },
            );
            if (chubDetailRes.ok) {
                const chubDetail = await chubDetailRes.json() as Record<string, any>;
                const projectId = chubDetail.node?.id;
                if (projectId && chubDetail.node?.hasGallery) {
                    const galleryRes = await fetch(
                        `https://gateway.chub.ai/api/gallery/project/${projectId}`,
                        { headers: { 'Accept': 'application/json', 'User-Agent': 'LumiHub' } },
                    );
                    if (galleryRes.ok) {
                        const galleryData = await galleryRes.json() as Record<string, any>;
                        const urls = (galleryData.nodes || [])
                            .filter((n: any) => n.primary_image_path)
                            .map((n: any) => n.primary_image_path as string);
                        if (urls.length > 0) {
                            payload.galleryImageUrls = urls;
                        }
                    }
                }
            }
        } catch { /* gallery fetch is best-effort */ }
    } else {
        // For LumiHub characters, fetch the card
        const character = await CharacterService.getCharacterById(character_id);
        if (!character) {
            return c.json({ error: 'Character not found' }, 404);
        }

        // If character has charx assets (expressions, gallery, etc.), send a download URL
        // so Lumiverse fetches the full .charx archive directly
        const hasAssets = await CharacterService.hasCharxAssets(character_id);
        if (hasAssets) {
            payload = {
                source: 'lumihub',
                characterId: character_id,
                characterName: character.name,
                importUrl: `${env.LUMIHUB_PUBLIC_URL}/api/v1/characters/${character_id}/charx`,
                importEmbeddedWorldbook: !!include_worldbook,
            };
        } else {
            // Simple character — send inline card data + avatar
            const cardData = {
                spec: 'chara_card_v3',
                spec_version: '3.0',
                data: {
                    name: character.name,
                    description: character.description,
                    personality: character.personality,
                    scenario: character.scenario,
                    first_mes: character.first_mes,
                    mes_example: character.mes_example,
                    alternate_greetings: character.alternate_greetings,
                    group_only_greetings: character.group_only_greetings,
                    system_prompt: character.system_prompt,
                    post_history_instructions: character.post_history_instructions,
                    creator: character.creator,
                    creator_notes: character.creator_notes,
                    creator_notes_multilingual: character.creator_notes_multilingual,
                    character_version: character.character_version,
                    tags: character.tags,
                    nickname: character.nickname,
                    source: character.source,
                    assets: character.assets,
                    character_book: character.character_book,
                    extensions: character.extensions,
                    creation_date: character.creation_date,
                    modification_date: character.modification_date,
                },
            };

            payload = {
                source: 'lumihub',
                characterId: character_id,
                characterName: character.name,
                cardData,
                importEmbeddedWorldbook: !!include_worldbook,
            };

            // Include avatar if available
            if (character.image_path) {
                try {
                    const path = await import('path');
                    const imgPath = path.resolve(env.UPLOADS_DIR, character.image_path.replace(/^uploads\//, ''));
                    const file = Bun.file(imgPath);
                    if (await file.exists()) {
                        const buf = await file.arrayBuffer();
                        payload.avatarBase64 = Buffer.from(buf).toString('base64');
                        payload.avatarMime = file.type || 'image/png';
                    }
                } catch { /* avatar unavailable, proceed without */ }
            }
        }
    }

    try {
        const result = await instanceManager.sendRequest(instance_id, 'install_character', payload);
        const resultPayload = result.payload as Record<string, unknown>;
        const success = resultPayload?.success ?? false;

        // Increment download counter on successful install
        if (success && source !== 'chub') {
            await CharacterService.incrementDownloads(character_id).catch(() => {});
        }

        return c.json({
            success,
            characterId: resultPayload?.characterId,
            error: resultPayload?.error,
        });
    } catch (err: any) {
        return c.json({ error: err.message || 'Install request failed' }, 504);
    }
});

/**
 * Trigger a remote worldbook install to a linked Lumiverse instance.
 */
link.post('/install-worldbook', requireAuth, async (c) => {
    const userId = c.get('userId');
    const body = await c.req.json();
    const { instance_id, worldbook_id, source } = body;

    if (!instance_id || !worldbook_id) {
        return c.json({ error: 'instance_id and worldbook_id are required' }, 400);
    }

    const instances = await LinkService.listInstances(userId);
    const instance = instances.find((i) => i.id === instance_id);
    if (!instance) return c.json({ error: 'Instance not found' }, 404);
    if (!instance.is_online) return c.json({ error: 'Instance is offline' }, 503);

    let payload: Record<string, unknown>;

    if (source === 'chub') {
        // Chub lorebook — fetch from Chub API ourselves and send inline
        const apiPath = worldbook_id.replace(/^lorebooks\//, '');
        const chubUrl = `https://api.chub.ai/api/lorebooks/${apiPath}?full=true`;

        let chubRes: Response;
        try {
            chubRes = await fetch(chubUrl, {
                headers: { 'Accept': 'application/json', 'User-Agent': 'LumiHub' },
            });
        } catch (err: any) {
            return c.json({ error: `Failed to reach Chub API: ${err.message}` }, 502);
        }

        if (!chubRes.ok) {
            // Try gateway as fallback
            const gwUrl = `https://gateway.chub.ai/api/lorebooks/${apiPath}?full=true`;
            try {
                chubRes = await fetch(gwUrl, {
                    headers: { 'Accept': 'application/json', 'User-Agent': 'LumiHub' },
                });
            } catch (err: any) {
                return c.json({ error: `Failed to reach Chub API: ${err.message}` }, 502);
            }
            if (!chubRes.ok) {
                return c.json({ error: `Chub API returned ${chubRes.status} for lorebook "${apiPath}"` }, 502);
            }
        }

        const chubData = (await chubRes.json()) as Record<string, any>;
        const def = chubData.node?.definition;
        if (!def) {
            return c.json({ error: 'No definition found in Chub lorebook response' }, 502);
        }

        const rawEntries = def.embedded_lorebook?.entries || [];
        const name = def.name || apiPath.split('/').pop() || worldbook_id;
        const description = def.description || '';

        payload = {
            source: 'chub',
            worldbookId: worldbook_id,
            worldbookName: name,
            worldbookData: {
                name,
                description,
                entries: rawEntries,
            },
        };
    } else {
        // LumiHub worldbook — send inline entries
        const wb = await WorldbookService.getWorldbookById(worldbook_id);
        if (!wb) return c.json({ error: 'Worldbook not found' }, 404);

        const entries = WorldbookService.normalizeLorebookEntries(wb.entries);
        payload = {
            source: 'lumihub',
            worldbookId: worldbook_id,
            worldbookName: wb.name,
            worldbookData: {
                name: wb.name,
                description: wb.description,
                entries,
            },
        };
    }

    try {
        const result = await instanceManager.sendRequest(instance_id, 'install_worldbook', payload);
        const resultPayload = result.payload as Record<string, unknown>;
        const success = resultPayload?.success ?? false;

        // Increment download counter on successful install
        if (success && source !== 'chub') {
            await WorldbookService.incrementDownloads(worldbook_id).catch(() => {});
        }

        return c.json({
            success,
            worldbookId: resultPayload?.worldbookId,
            error: resultPayload?.error,
        });
    } catch (err: any) {
        return c.json({ error: err.message || 'Install request failed' }, 504);
    }
});

/**
 * Get the install manifest for the user's linked instance.
 * Returns the list of characters and world books installed on their Lumiverse instance.
 * Optional ?type=character|worldbook filter.
 */
link.get('/manifest', requireAuth, async (c) => {
    const userId = c.get('userId');
    const type = c.req.query('type') as 'character' | 'worldbook' | undefined;
    const ManifestService = await import('../services/manifest.service.ts');

    const result = await ManifestService.getManifestForUser(userId, type || undefined);
    if (!result) {
        return c.json({ entries: [], instance_id: null });
    }
    return c.json({ entries: result.entries, instance_id: result.instanceId });
});

/**
 * Quick check if a specific slug is installed on the user's linked instance.
 */
link.get('/manifest/check', requireAuth, async (c) => {
    const userId = c.get('userId');
    const slug = c.req.query('slug');
    if (!slug) return c.json({ error: 'slug query parameter is required' }, 400);

    const ManifestService = await import('../services/manifest.service.ts');
    const result = await ManifestService.getManifestForUser(userId);
    if (!result) return c.json({ installed: false });

    const entry = await ManifestService.checkSlug(result.instanceId, slug);
    return c.json({ installed: !!entry, entry });
});

export default link;
