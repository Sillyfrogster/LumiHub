import {
  Entity,
  PrimaryGeneratedColumn,
  Column,
  CreateDateColumn,
  ManyToOne,
  JoinColumn,
} from 'typeorm';
import { Character } from './Character.entity.ts';

@Entity('character_images')
export class CharacterImage {
  @PrimaryGeneratedColumn('uuid')
  id: string;

  @Column({ type: 'uuid' })
  character_id: string;

  @ManyToOne(() => Character, { onDelete: 'CASCADE' })
  @JoinColumn({ name: 'character_id' })
  character: Character;

  @Column({ type: 'varchar', length: 32 })
  image_type: 'avatar' | 'avatar_alt' | 'expression' | 'gallery';

  @Column({ type: 'varchar', length: 255, nullable: true })
  label: string | null;

  @Column({ type: 'varchar', length: 512 })
  file_path: string;

  @Column({ type: 'varchar', length: 64 })
  mime_type: string;

  @Column({ type: 'int', default: 0 })
  file_size: number;

  @Column({ type: 'int', default: 0 })
  sort_order: number;

  @CreateDateColumn()
  created_at: Date;
}
