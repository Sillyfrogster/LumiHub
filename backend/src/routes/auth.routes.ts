import { Hono } from 'hono';
import { setCookie, getCookie, deleteCookie } from 'hono/cookie';
import { env } from '../env.ts';
import { randomUUID } from 'crypto';
import { getDiscordAuthUrl, exchangeCode, fetchDiscordUser, createSessionToken, createRefreshToken, hashToken } from '../services/auth.service.ts';
import { upsertDiscordUser } from '../services/user.service.ts';
import { AppDataSource } from '../db/connection.ts';
import { User } from '../entities/User.entity.ts';

const auth = new Hono();

/** Redirect to Discord OAuth */
auth.get('/discord', (c) => {
    const generatedState = randomUUID();

    // Store return_to URL so we can redirect back after login
    const returnTo = c.req.query('return_to');
    if (returnTo) {
        setCookie(c, 'oauth_return_to', returnTo, {
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
        maxAge: 60 * 15,
        sameSite: 'Lax'
    });

    setCookie(c, 'refresh_token', refreshToken, {
        httpOnly: true,
        secure: env.NODE_ENV === 'production',
        path: '/api/v1/auth/refresh',
        maxAge: 60 * 60 * 24 * 30,
        sameSite: 'Strict'
    });

    // Redirect to return_to URL if set (e.g. from Lumiverse PKCE link flow), otherwise home
    const returnTo = getCookie(c, 'oauth_return_to');
    if (returnTo) {
        deleteCookie(c, 'oauth_return_to', { path: '/' });
        return c.redirect(returnTo);
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
        deleteCookie(c, 'refresh_token', { path: '/api/v1/auth/refresh' });
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
        maxAge: 60 * 15,
        sameSite: 'Lax'
    });

    setCookie(c, 'refresh_token', newRefreshToken, {
        httpOnly: true,
        secure: env.NODE_ENV === 'production',
        path: '/api/v1/auth/refresh',
        maxAge: 60 * 60 * 24 * 30,
        sameSite: 'Strict'
    });

    return c.json({ message: "Token refreshed" }, 200);
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
    deleteCookie(c, 'refresh_token', { path: '/api/v1/auth/refresh' });

    return c.json({ message: "Logged out" }, 200);
});

export default auth;