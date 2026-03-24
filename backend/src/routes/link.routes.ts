import { Hono } from 'hono';
import { requireAuth, type AuthEnv } from '../middleware/requireAuth.middleware.ts';
import { getCookie } from 'hono/cookie';
import { verify } from 'hono/jwt';
import { env } from '../env.ts';
import * as LinkService from '../services/link.service.ts';
import * as CharacterService from '../services/characters.service.ts';
import { instanceManager } from '../ws/instance-connections.ts';

const link = new Hono<AuthEnv>();

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
    const payload = await verify(token, env.JWT_SECRET, 'HS256');
    c.set('userId', payload.id as string);
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

    const userId = c.get('userId');
    const code = await LinkService.createAuthorizationCode(userId, codeChallenge, instanceName, redirectOrigin);

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
    <p>Allow <span class="instance-name">${instanceName.replace(/[<>&"]/g, '')}</span> to connect to your LumiHub account?</p>
    <p>This will let you install characters directly from LumiHub to this Lumiverse instance.</p>
    <div class="actions">
      <a class="btn approve" href="${redirectOrigin}/api/v1/lumihub/callback?code=${code}">Approve</a>
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
    const { instance_id, character_id, source } = body;

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
        };
    } else {
        // For LumiHub characters, fetch the card and send inline
        const character = await CharacterService.getCharacterById(character_id);
        if (!character) {
            return c.json({ error: 'Character not found' }, 404);
        }

        // Build CCSv3 card data
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

    try {
        const result = await instanceManager.sendRequest(instance_id, 'install_character', payload);
        const resultPayload = result.payload as Record<string, unknown>;
        return c.json({
            success: resultPayload?.success ?? false,
            characterId: resultPayload?.characterId,
            error: resultPayload?.error,
        });
    } catch (err: any) {
        return c.json({ error: err.message || 'Install request failed' }, 504);
    }
});

export default link;
