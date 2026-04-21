import { Entity, Column } from 'typeorm';
import { BaseAsset } from './BaseAsset.entity.ts';

/** Generation presets and templates */
@Entity('presets')
export class Preset extends BaseAsset {
  @Column({ type: 'jsonb', default: [] })
  tags: string[];

  @Column({ type: 'jsonb', default: {} })
  settings: Record<string, any>;
}
