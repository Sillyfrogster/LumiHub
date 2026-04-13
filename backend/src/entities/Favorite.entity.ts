import { Entity, PrimaryGeneratedColumn, Column, CreateDateColumn, Unique } from 'typeorm';

/**
 * Tracks which user has favorited which asset.
 * Each user may favorite any given asset at most once (enforced by the unique constraint).
 */
@Entity('favorites')
@Unique(['user_id', 'asset_type', 'asset_id'])
export class Favorite {
  @PrimaryGeneratedColumn('uuid')
  id: string;

  @Column({ type: 'uuid' })
  user_id: string;

  /** Discriminator so the same table covers both characters and worldbooks. */
  @Column({ type: 'varchar', length: 32 })
  asset_type: 'character' | 'worldbook';

  @Column({ type: 'uuid' })
  asset_id: string;

  @CreateDateColumn()
  created_at: Date;
}
