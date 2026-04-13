import {
  PrimaryGeneratedColumn,
  Column,
  CreateDateColumn,
  UpdateDateColumn,
  ManyToOne,
  JoinColumn
} from 'typeorm';
import { User } from './User.entity.ts';

/** Base columns for all user-generated content */
export abstract class BaseAsset {
  @PrimaryGeneratedColumn('uuid')
  id: string;

  @Column({ type: 'varchar', length: 255 })
  name: string;

  @Column({ type: 'text', default: '' })
  description: string;

  @ManyToOne(() => User, { nullable: true, onDelete: 'SET NULL' })
  @JoinColumn({ name: 'owner_id' })
  owner: User | null;

  @Column({ type: 'uuid', nullable: true })
  owner_id: string | null;

  @Column({ type: 'varchar', length: 512, nullable: true })
  image_path: string | null;

  @Column({ type: 'int', default: 0 })
  downloads: number;

  @Column({ type: 'int', default: 0 })
  views: number;

  @Column({ type: 'int', default: 0 })
  favorites: number;

  @Column({ type: 'boolean', default: false })
  hidden: boolean;

  @CreateDateColumn()
  created_at: Date;

  @UpdateDateColumn()
  updated_at: Date;
}
