import type { EntityTarget } from 'typeorm';
import { Character } from '../entities/Character.entity.ts';
import { Worldbook } from '../entities/Worldbook.entity.ts';
import { Theme } from '../entities/Theme.entity.ts';
import { Preset } from '../entities/Preset.entity.ts';

export type HubAssetType = 'character' | 'worldbook' | 'theme' | 'preset';
export type HubAssetRouteSlug = 'characters' | 'worldbooks' | 'themes' | 'presets';

export interface HubAssetDefinition {
  type: HubAssetType;
  routeSlug: HubAssetRouteSlug;
  tableName: string;
  displayName: string;
  entity: EntityTarget<any>;
  leaderboardEligible: boolean;
}

export const HUB_ASSETS = {
  character: {
    type: 'character',
    routeSlug: 'characters',
    tableName: 'characters',
    displayName: 'Character',
    entity: Character,
    leaderboardEligible: true,
  },
  worldbook: {
    type: 'worldbook',
    routeSlug: 'worldbooks',
    tableName: 'worldbooks',
    displayName: 'Worldbook',
    entity: Worldbook,
    leaderboardEligible: true,
  },
  theme: {
    type: 'theme',
    routeSlug: 'themes',
    tableName: 'themes',
    displayName: 'Theme',
    entity: Theme,
    leaderboardEligible: false,
  },
  preset: {
    type: 'preset',
    routeSlug: 'presets',
    tableName: 'presets',
    displayName: 'Preset',
    entity: Preset,
    leaderboardEligible: false,
  },
} as const satisfies Record<HubAssetType, HubAssetDefinition>;

export const HUB_ASSET_TYPES = Object.keys(HUB_ASSETS) as HubAssetType[];
export const HUB_ASSET_ROUTE_SLUGS = Object.values(HUB_ASSETS).map((asset) => asset.routeSlug);
export const LEADERBOARD_ASSET_ROUTE_SLUGS = Object.values(HUB_ASSETS)
  .filter((asset) => asset.leaderboardEligible)
  .map((asset) => asset.routeSlug);

export function isHubAssetType(value: string): value is HubAssetType {
  return Object.prototype.hasOwnProperty.call(HUB_ASSETS, value);
}

export function isHubAssetRouteSlug(value: string): value is HubAssetRouteSlug {
  return HUB_ASSET_ROUTE_SLUGS.includes(value as HubAssetRouteSlug);
}

export function getHubAsset(type: HubAssetType): HubAssetDefinition {
  return HUB_ASSETS[type];
}

export function getHubAssetByRouteSlug(routeSlug: HubAssetRouteSlug): HubAssetDefinition {
  return Object.values(HUB_ASSETS).find((asset) => asset.routeSlug === routeSlug)!;
}
