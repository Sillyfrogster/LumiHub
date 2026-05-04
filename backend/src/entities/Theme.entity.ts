import { Entity, Column } from 'typeorm';
import { BaseAsset } from './BaseAsset.entity.ts';

/** Custom UI themes and color palettes */
@Entity('themes')
export class Theme extends BaseAsset {
  @Column({ type: 'jsonb', default: {} })
  config: Record<string, any>;

  @Column({ type: 'jsonb', default: [] })
  tags: string[];

  @Column({ type: 'int', default: 1 })
  schema_version: number;

  @Column({ type: 'jsonb', default: {} })
  compatibility: Record<string, any>;

  @Column({ type: 'text', nullable: true })
  custom_css: string | null;

  @Column({ type: 'varchar', length: 255, nullable: true })
  asset_bundle_id: string | null;
}
