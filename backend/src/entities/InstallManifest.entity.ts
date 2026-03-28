import { Entity, PrimaryGeneratedColumn, Column, CreateDateColumn, ManyToOne, JoinColumn, Unique } from 'typeorm';
import { LinkedInstance } from './LinkedInstance.entity.ts';

@Entity('install_manifests')
@Unique(['instance_id', 'slug'])
export class InstallManifest {
    @PrimaryGeneratedColumn('uuid')
    id: string;

    @Column({ type: 'uuid' })
    instance_id: string;

    @ManyToOne(() => LinkedInstance)
    @JoinColumn({ name: 'instance_id' })
    instance: LinkedInstance;

    @Column({ type: 'varchar', length: 500 })
    slug: string;

    @Column({ type: 'varchar', length: 20, default: 'character' })
    entry_type: string;

    @Column({ type: 'varchar', length: 255 })
    character_name: string;

    @Column({ type: 'varchar', length: 255 })
    creator_name: string;

    @Column({ type: 'varchar', length: 50, default: 'local' })
    source: string;

    @Column({ type: 'bigint', nullable: true })
    installed_at: number | null;

    @CreateDateColumn()
    synced_at: Date;
}
