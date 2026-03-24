import { Entity, PrimaryGeneratedColumn, Column, CreateDateColumn, ManyToOne, JoinColumn } from 'typeorm';
import { User } from './User.entity.ts';

@Entity('linked_instances')
export class LinkedInstance {
    @PrimaryGeneratedColumn('uuid')
    id: string;

    @Column({ type: 'uuid' })
    user_id: string;

    @ManyToOne(() => User)
    @JoinColumn({ name: 'user_id' })
    user: User;

    @Column({ type: 'varchar', length: 255 })
    instance_name: string;

    @Column({ type: 'varchar', length: 64 })
    token_hash: string;

    @Column({ type: 'varchar', length: 8 })
    token_prefix: string;

    @Column({ type: 'timestamp', nullable: true })
    last_seen_at: Date | null;

    @Column({ type: 'timestamp', nullable: true })
    revoked_at: Date | null;

    @CreateDateColumn()
    created_at: Date;
}
