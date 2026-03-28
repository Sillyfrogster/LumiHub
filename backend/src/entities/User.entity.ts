import { Entity, PrimaryGeneratedColumn, Column, CreateDateColumn } from 'typeorm';

@Entity('users')
export class User {
    @PrimaryGeneratedColumn('uuid')
    id: string;

    @Column({ type: 'varchar', length: 255, unique: true })
    discord_id: string;

    @Column({ type: 'varchar', length: 255, unique: true })
    username: string;

    @Column({ type: 'varchar', length: 512, nullable: true })
    banner: string | null;

    @Column({ type: 'text', nullable: true })
    custom_css: string | null;

    @Column({ type: 'text', nullable: true })
    custom_html: string | null;

    @Column({ type: 'varchar', length: 255, nullable: true })
    display_name: string;

    @Column({ type: 'varchar', length: 255, nullable: true })
    avatar: string;

    @Column({ type: 'varchar', length: 255, default: 'user' })
    role: string;

    @Column({ type: 'varchar', length: 255, nullable: true })
    refresh_token: string | null;

    @Column({ type: 'varchar', length: 255, nullable: true })
    custom_display_name: string | null;

    @Column({ type: 'boolean', default: false })
    nsfw_enabled: boolean;

    @Column({ type: 'boolean', default: false })
    nsfw_unblurred: boolean;

    @Column({ type: 'jsonb', default: [] })
    default_include_tags: string[];

    @Column({ type: 'jsonb', default: [] })
    default_exclude_tags: string[];

    @CreateDateColumn()
    created_at: Date;
}