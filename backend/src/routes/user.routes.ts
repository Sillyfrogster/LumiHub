import { Hono } from 'hono';
import { jwt } from 'hono/jwt';
import { env } from '../env.ts';
import { AppDataSource } from '../db/connection.ts';
import { User } from '../entities/User.entity.ts';
import { sanitizeProfileHtml, sanitizeProfileCss } from '../services/profile-sanitizer.service.ts';

const users = new Hono();

/** Get public user profile by Discord ID */
users.get('/profile/:discordId', async (c) => {
    const userRepository = AppDataSource.getRepository(User);
    const user = await userRepository.findOneBy({ discord_id: c.req.param('discordId') });

    if (!user) {
        return c.json({ error: "User not found" }, 404);
    }

    return c.json({
        id: user.id,
        discordId: user.discord_id,
        username: user.username,
        displayName: user.custom_display_name || user.display_name,
        avatar: user.avatar,
        banner: user.banner,
        customCss: user.custom_css,
        customHtml: user.custom_html,
        role: user.role,
        createdAt: user.created_at
    });
});

/** Protect all user routes with JWT */
users.use('/*', jwt({
    secret: env.JWT_SECRET,
    cookie: 'lumihub_session',
    alg: 'HS256'
}));

/** Save profile HTML and/or CSS (Profile Studio) */
users.put('/@me/profile', async (c) => {
    const payload = c.get('jwtPayload') as { id: string };
    const userRepository = AppDataSource.getRepository(User);
    const user = await userRepository.findOneBy({ id: payload.id });

    if (!user) {
        return c.json({ error: 'User not found' }, 404);
    }

    const body = await c.req.json();

    if (typeof body.html === 'string') {
        if (body.html.length > 100000) {
            return c.json({ error: 'HTML too large (max 100KB)' }, 400);
        }
        user.custom_html = body.html.trim().length > 0 ? sanitizeProfileHtml(body.html) : null;
    }

    if (typeof body.css === 'string') {
        if (body.css.length > 50000) {
            return c.json({ error: 'CSS too large (max 50KB)' }, 400);
        }
        user.custom_css = body.css.trim().length > 0 ? sanitizeProfileCss(body.css) : null;
    }

    await userRepository.save(user);

    return c.json({
        message: 'Profile updated',
        html: user.custom_html,
        css: user.custom_css,
    });
});

/** Reset profile to defaults */
users.post('/@me/profile/reset', async (c) => {
    const payload = c.get('jwtPayload') as { id: string };
    const userRepository = AppDataSource.getRepository(User);
    const user = await userRepository.findOneBy({ id: payload.id });

    if (!user) {
        return c.json({ error: 'User not found' }, 404);
    }

    user.custom_html = null;
    user.custom_css = null;
    await userRepository.save(user);

    return c.json({ message: 'Profile reset to default' });
});

/** Get current user preferences */
users.get('/@me/settings', async (c) => {
    const payload = c.get('jwtPayload') as { id: string };
    const userRepository = AppDataSource.getRepository(User);
    const user = await userRepository.findOneBy({ id: payload.id });

    if (!user) {
        return c.json({ error: "User not found" }, 404);
    }

    return c.json({
        customDisplayName: user.custom_display_name || '',
        nsfwEnabled: user.nsfw_enabled,
        nsfwUnblurred: user.nsfw_unblurred,
        defaultIncludeTags: user.default_include_tags,
        defaultExcludeTags: user.default_exclude_tags,
    });
});

/** Update current user preferences */
users.put('/@me/settings', async (c) => {
    const payload = c.get('jwtPayload') as { id: string };
    const userRepository = AppDataSource.getRepository(User);
    const user = await userRepository.findOneBy({ id: payload.id });

    if (!user) {
        return c.json({ error: "User not found" }, 404);
    }

    const body = await c.req.json();

    if (typeof body.customDisplayName === 'string') {
        const trimmed = body.customDisplayName.trim();
        user.custom_display_name = trimmed.length > 0 ? trimmed.slice(0, 64) : null;
    }
    if (typeof body.nsfwEnabled === 'boolean') {
        user.nsfw_enabled = body.nsfwEnabled;
    }
    if (typeof body.nsfwUnblurred === 'boolean') {
        user.nsfw_unblurred = body.nsfwUnblurred;
    }
    if (Array.isArray(body.defaultIncludeTags)) {
        user.default_include_tags = body.defaultIncludeTags
            .filter((t: unknown) => typeof t === 'string')
            .map((t: string) => t.trim().toLowerCase())
            .filter(Boolean)
            .slice(0, 50);
    }
    if (Array.isArray(body.defaultExcludeTags)) {
        user.default_exclude_tags = body.defaultExcludeTags
            .filter((t: unknown) => typeof t === 'string')
            .map((t: string) => t.trim().toLowerCase())
            .filter(Boolean)
            .slice(0, 50);
    }

    await userRepository.save(user);

    return c.json({ message: 'Settings updated' });
});

/** Get current user profile */
users.get('/@me', async (c) => {
    const payload = c.get('jwtPayload') as { id: string, username: string, exp: number, iss: string };
    const userRepository = AppDataSource.getRepository(User);
    const user = await userRepository.findOneBy({ id: payload.id });

    if (!user) {
        return c.json({ error: "User not found" }, 404);
    }

    return c.json({
        id: user.id,
        discordId: user.discord_id,
        username: user.username,
        displayName: user.custom_display_name || user.display_name,
        avatar: user.avatar,
        role: user.role,
        createdAt: user.created_at
    });
});

export default users;
