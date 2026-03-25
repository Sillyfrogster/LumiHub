import { AppDataSource } from '../db/connection.ts';
import { User } from '../entities/User.entity.ts';
import { env } from '../env.ts';

/**
 * Upserts a user from Discord data.
 * If the user exists (by discord_id), updates their profile and refresh token.
 * If they don't exist, creates a new user.
 */
export async function upsertDiscordUser(discordData: any, refreshToken: string) {
    const userRepository = AppDataSource.getRepository(User);
    let user = await userRepository.findOneBy({ discord_id: discordData.id });
    const avatarUrl = discordData.avatar
        ? `https://cdn.discordapp.com/avatars/${discordData.id}/${discordData.avatar}.png`
        : null;
    const bannerUrl = discordData.banner
        ? `https://cdn.discordapp.com/banners/${discordData.id}/${discordData.banner}.png?size=1024`
        : null;

    const isModerator = env.MODERATOR_DISCORD_IDS.includes(discordData.id);
    const assignedRole = isModerator ? 'moderator' : 'user';

    if (user) {
        user.username = discordData.username;
        user.display_name = discordData.global_name || discordData.username;
        user.avatar = avatarUrl || '';
        user.role = assignedRole;

        if (!user.banner) user.banner = bannerUrl;
        user.refresh_token = refreshToken;
        await userRepository.save(user);
    } else {
        user = userRepository.create({
            discord_id: discordData.id,
            username: discordData.username,
            display_name: discordData.global_name || discordData.username,
            avatar: avatarUrl || '',
            banner: bannerUrl,
            role: assignedRole,
            refresh_token: refreshToken
        });
        await userRepository.save(user);
    }

    return user;
}
