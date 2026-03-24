import { Entity, Column, ManyToOne, JoinColumn, PrimaryColumn } from 'typeorm';
import { User } from './User.entity.ts';

@Entity('link_codes')
export class LinkCode {
    @PrimaryColumn({ type: 'varchar', length: 64 })
    code: string;

    @Column({ type: 'uuid' })
    user_id: string;

    @ManyToOne(() => User)
    @JoinColumn({ name: 'user_id' })
    user: User;

    @Column({ type: 'varchar', length: 128 })
    code_challenge: string;

    @Column({ type: 'varchar', length: 255 })
    instance_name: string;

    @Column({ type: 'varchar', length: 512 })
    redirect_origin: string;

    @Column({ type: 'timestamp' })
    expires_at: Date;

    @Column({ type: 'boolean', default: false })
    consumed: boolean;
}
