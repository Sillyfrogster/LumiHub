import { randomBytes, createHash } from "crypto";
import { AppDataSource } from "../db/connection.ts";
import { LinkedInstance } from "../entities/LinkedInstance.entity.ts";
import { LinkCode } from "../entities/LinkCode.entity.ts";
import { hashToken } from "./auth.service.ts";
import { instanceManager } from "../ws/instance-connections.ts";
import { IsNull } from "typeorm";

const instanceRepo = () => AppDataSource.getRepository(LinkedInstance);
const codeRepo = () => AppDataSource.getRepository(LinkCode);

/** Create a PKCE authorization code for instance linking. */
export async function createAuthorizationCode(
    userId: string,
    codeChallenge: string,
    instanceName: string,
    redirectOrigin: string
): Promise<string> {
    const code = randomBytes(32).toString("hex");
    const expiresAt = new Date(Date.now() + 5 * 60 * 1000); // 5 minutes

    const linkCode = codeRepo().create({
        code,
        user_id: userId,
        code_challenge: codeChallenge,
        instance_name: instanceName,
        redirect_origin: redirectOrigin,
        expires_at: expiresAt,
        consumed: false,
    });
    await codeRepo().save(linkCode);
    return code;
}

/** Exchange an authorization code + PKCE verifier for a link token. */
export async function exchangeCode(
    code: string,
    codeVerifier: string
): Promise<{ token: string; instanceId: string } | null> {
    const linkCode = await codeRepo().findOneBy({ code });
    if (!linkCode) return null;
    if (linkCode.consumed) return null;
    if (new Date() > linkCode.expires_at) return null;

    // Verify PKCE: SHA-256(code_verifier) as base64url must equal code_challenge
    const expectedChallenge = createHash("sha256")
        .update(codeVerifier)
        .digest("base64url");
    if (expectedChallenge !== linkCode.code_challenge) return null;

    // Mark as consumed
    linkCode.consumed = true;
    await codeRepo().save(linkCode);

    // Generate link token
    const token = randomBytes(48).toString("hex");

    // Create linked instance
    const instance = instanceRepo().create({
        user_id: linkCode.user_id,
        instance_name: linkCode.instance_name,
        token_hash: hashToken(token),
        token_prefix: token.substring(0, 8),
    });
    await instanceRepo().save(instance);

    return { token, instanceId: instance.id };
}

/** Validate a link token and return the associated instance. */
export async function validateLinkToken(token: string): Promise<LinkedInstance | null> {
    const hash = hashToken(token);
    return instanceRepo().findOneBy({
        token_hash: hash,
        revoked_at: IsNull(),
    });
}

/** Revoke a linked instance. */
export async function revokeInstance(instanceId: string, userId: string): Promise<boolean> {
    const result = await instanceRepo().update(
        { id: instanceId, user_id: userId, revoked_at: IsNull() },
        { revoked_at: new Date() }
    );
    if (result.affected && result.affected > 0) {
        instanceManager.disconnectInstance(instanceId);
        return true;
    }
    return false;
}

/** List all non-revoked instances for a user, with online status. */
export async function listInstances(userId: string) {
    const instances = await instanceRepo().findBy({
        user_id: userId,
        revoked_at: IsNull(),
    });
    return instances.map((inst) => ({
        id: inst.id,
        instance_name: inst.instance_name,
        token_prefix: inst.token_prefix,
        last_seen_at: inst.last_seen_at,
        is_online: instanceManager.isOnline(inst.id),
        created_at: inst.created_at,
    }));
}

/** Update last_seen_at for an instance. */
export async function updateLastSeen(instanceId: string): Promise<void> {
    await instanceRepo().update({ id: instanceId }, { last_seen_at: new Date() });
}
