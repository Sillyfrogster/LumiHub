import { Hono } from 'hono';
import { setCookie, getCookie, deleteCookie } from 'hono/cookie';
import { env } from '../env.ts';
import { randomUUID } from 'crypto';
import { getDiscordAuthUrl, exchangeCode, fetchDiscordUser, createSessionToken, createRefreshToken, hashToken, REFRESH_TOKEN_PATH, SESSION_TTL_SECONDS } from '../services/auth.service.ts';
import { upsertDiscordUser } from '../services/user.service.ts';
import { AppDataSource } from '../db/connection.ts';
import { User } from '../entities/User.entity.ts';

const auth = new Hono();

function normalizeReturnTo(returnTo: string, requestUrl: string): string | null {
    try {
        const target = new URL(returnTo, requestUrl);
        const allowedOrigin = new URL(requestUrl).origin;
        if (target.origin !== allowedOrigin) {
            return null;
        }
        return target.toString();
    } catch {
        return null;
    }
}

/** Redirect to Discord OAuth */
auth.get('/discord', (c) => {
    const generatedState = randomUUID();

    // Store return_to URL so we can redirect back after login
    const returnTo = c.req.query('return_to');
    if (returnTo) {
        const safeReturnTo = normalizeReturnTo(returnTo, c.req.url);
        if (!safeReturnTo) {
            return c.json({ error: 'Invalid return_to URL' }, 400);
        }
        setCookie(c, 'oauth_return_to', safeReturnTo, {
            httpOnly: true,
            secure: env.NODE_ENV === 'production',
            path: '/',
            maxAge: 60 * 10,
            sameSite: 'Lax',
        });
    }

    setCookie(c, 'oauth_state', generatedState, {
        httpOnly: true,
        secure: env.NODE_ENV === 'production',
        path: '/',
        maxAge: 60 * 10,
        sameSite: 'Lax',
    })
    return c.redirect(getDiscordAuthUrl(generatedState), 302)
})

/** Discord OAuth callback: exchange code, create/update db user, set cookies */
auth.get('/discord/callback', async (c) => {
    const ourState = getCookie(c, 'oauth_state');
    const discordRedirectState = c.req.query('state');

    if (ourState !== discordRedirectState) {
        return c.json({
            error: "Invalid State",
            message: "Redirected state does not match our state."
        }, 400)
    }

    const code = c.req.query('code');
    if (!code) return c.json({ error: "Missing code", message: "Code not found in redirect." }, 400)

    const tokenData = await exchangeCode(code);
    if (!tokenData) return c.json({ error: "Exchange Failed", message: "Failed to exchange code for token." }, 400)

    const userData = await fetchDiscordUser(tokenData.access_token);
    if (!userData) return c.json({ error: "User Fetch Failed", message: "Failed to fetch user from Discord." }, 400)

    const refreshToken = createRefreshToken();
    const hashedRefresh = hashToken(refreshToken);
    const dbUser = await upsertDiscordUser(userData, hashedRefresh);
    const sessionToken = await createSessionToken(dbUser);

    setCookie(c, 'lumihub_session', sessionToken, {
        httpOnly: true,
        secure: env.NODE_ENV === 'production',
        path: '/',
        maxAge: SESSION_TTL_SECONDS,
        sameSite: 'Lax'
    });

    setCookie(c, 'refresh_token', refreshToken, {
        httpOnly: true,
        secure: env.NODE_ENV === 'production',
        path: REFRESH_TOKEN_PATH,
        maxAge: 60 * 60 * 24 * 30,
        sameSite: 'Strict'
    });

    // Redirect to return_to URL if set (e.g. from Lumiverse PKCE link flow), otherwise home
    const returnTo = getCookie(c, 'oauth_return_to');
    if (returnTo) {
        deleteCookie(c, 'oauth_return_to', { path: '/' });
        const safeReturnTo = normalizeReturnTo(returnTo, c.req.url);
        if (safeReturnTo) {
            return c.redirect(safeReturnTo);
        }
    }
    return c.redirect('/');
})

/** Refresh JWT session (with token rotation) */
auth.post('/refresh', async (c) => {
    const refreshToken = getCookie(c, 'refresh_token');
    if (!refreshToken) return c.json({ error: "No refresh token" }, 401);

    const hashedIncoming = hashToken(refreshToken);
    const userRepository = AppDataSource.getRepository(User);
    const user = await userRepository.findOneBy({ refresh_token: hashedIncoming });

    if (!user) {
        deleteCookie(c, 'lumihub_session', { path: '/' });
        deleteCookie(c, 'refresh_token', { path: REFRESH_TOKEN_PATH });
        return c.json({ error: "Invalid refresh token" }, 401);
    }

    // Rotate refresh token
    const newRefreshToken = createRefreshToken();
    user.refresh_token = hashToken(newRefreshToken);
    await userRepository.save(user);

    const newSessionToken = await createSessionToken(user);

    setCookie(c, 'lumihub_session', newSessionToken, {
        httpOnly: true,
        secure: env.NODE_ENV === 'production',
        path: '/',
        maxAge: SESSION_TTL_SECONDS,
        sameSite: 'Lax'
    });

    setCookie(c, 'refresh_token', newRefreshToken, {
        httpOnly: true,
        secure: env.NODE_ENV === 'production',
        path: REFRESH_TOKEN_PATH,
        maxAge: 60 * 60 * 24 * 30,
        sameSite: 'Strict'
    });

    return c.json({
        user: {
            id: user.id,
            discordId: user.discord_id,
            username: user.username,
            displayName: user.display_name,
            avatar: user.avatar,
            banner: user.banner,
            role: user.role,
            createdAt: user.created_at
        }
    }, 200);
});

/** Logout: clear cookies and nullify refresh token in DB */
auth.post('/logout', async (c) => {
    const refreshToken = getCookie(c, 'refresh_token');

    if (refreshToken) {
        const hashedIncoming = hashToken(refreshToken);
        const userRepository = AppDataSource.getRepository(User);
        const user = await userRepository.findOneBy({ refresh_token: hashedIncoming });
        if (user) {
            user.refresh_token = null;
            await userRepository.save(user);
        }
    }

    deleteCookie(c, 'lumihub_session', { path: '/' });
    deleteCookie(c, 'refresh_token', { path: REFRESH_TOKEN_PATH });

    return c.json({ message: "Logged out" }, 200);
});

export default auth;
