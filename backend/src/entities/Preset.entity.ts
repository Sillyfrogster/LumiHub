import { Entity, Column } from 'typeorm';
import { BaseAsset } from './BaseAsset.entity.ts';

/** Generation presets and templates */
@Entity('presets')
export class Preset extends BaseAsset {
  @Column({ type: 'jsonb', default: {} })
  preset: Record<string, any>;

  @Column({ type: 'jsonb', default: [] })
  tags: string[];

  @Column({ type: 'int', default: 1 })
  schema_version: number;

  @Column({ type: 'jsonb', default: {} })
  compatibility: Record<string, any>;
}
