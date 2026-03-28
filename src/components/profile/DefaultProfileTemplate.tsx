import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { User, Shield, LayoutGrid, Users, Settings, Palette } from 'lucide-react';
import { useCharacters } from '../../hooks/useCharacters';
import CharacterCard from '../characters/CharacterCard';
import LazyImage from '../shared/LazyImage';
import type { UserProfileData } from '../../hooks/useUserProfile';
import styles from './DefaultProfileTemplate.module.css';

type FilterTab = 'characters' | 'worldbooks' | 'presets' | 'themes';

const TABS: { id: FilterTab; label: string; icon: React.ElementType }[] = [
  { id: 'characters', label: 'Characters', icon: Users },
  { id: 'worldbooks', label: 'Worldbooks', icon: LayoutGrid },
  { id: 'themes', label: 'Themes', icon: Palette },
  { id: 'presets', label: 'Presets', icon: Settings },
];

interface DefaultProfileTemplateProps {
  profile: UserProfileData;
}

const DefaultProfileTemplate = ({ profile }: DefaultProfileTemplateProps) => {
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<FilterTab>('characters');

  const { characters, loading: charactersLoading } = useCharacters({
    ownerId: profile.id,
    ignoreStore: true,
    enabled: !!profile.id,
  });

  const totalDownloads = characters.reduce((sum, c) => sum + c.downloads, 0);
  const formatCount = (n: number) => n > 1000 ? `${(n / 1000).toFixed(1)}k` : String(n);

  return (
    <div className={styles.profileContainer} data-studio="profile-container">
      {/* Banner */}
      <div className={styles.banner} data-studio="banner">
        {profile.banner ? (
          <LazyImage src={profile.banner} alt="" className={styles.bannerImage} />
        ) : (
          <div className={styles.bannerFallback} />
        )}
        <div className={styles.bannerOverlay} />
      </div>

      {/* Header */}
      <div className={styles.profileHeader} data-studio="profile-header">
        <div className={styles.avatarWrapper} data-studio="avatar">
          {profile.avatar ? (
            <LazyImage src={profile.avatar} alt={profile.displayName || profile.username} className={styles.avatar} />
          ) : (
            <div className={styles.avatarFallback}>
              {(profile.displayName || profile.username).charAt(0).toUpperCase()}
            </div>
          )}
        </div>

        <div className={styles.identity} data-studio="identity">
          <div className={styles.nameRow}>
            <h1 className={styles.displayName} data-studio="name">
              {profile.displayName || profile.username}
            </h1>
            {profile.role && profile.role !== 'user' && (
              <div className={styles.roleBadge} data-studio="role-badge">
                <Shield size={11} />
                {profile.role.charAt(0).toUpperCase() + profile.role.slice(1)}
              </div>
            )}
          </div>
          <p className={styles.handle} data-studio="handle">@{profile.username}</p>
        </div>
      </div>

      {/* Stats */}
      <div className={styles.statsRow} data-studio="stats">
        <div className={styles.stat} data-studio="stat-uploads">
          <span className={styles.statValue}>{formatCount(characters.length)}</span>
          <span className={styles.statLabel}>Uploads</span>
        </div>
        <div className={styles.statDivider} />
        <div className={styles.stat} data-studio="stat-downloads">
          <span className={styles.statValue}>{formatCount(totalDownloads)}</span>
          <span className={styles.statLabel}>Downloads</span>
        </div>
        <div className={styles.statDivider} />
        <div className={styles.stat} data-studio="stat-joined">
          <span className={styles.statValue}>
            {new Date(profile.createdAt).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })}
          </span>
          <span className={styles.statLabel}>Joined</span>
        </div>
      </div>

      {/* Tab Bar */}
      <div className={styles.tabBarWrapper} data-studio="tabs">
        <div className={styles.tabBar}>
          {TABS.map((tab) => (
            <button
              key={tab.id}
              className={`${styles.tab} ${activeTab === tab.id ? styles.tabActive : ''}`}
              onClick={() => setActiveTab(tab.id)}
              data-studio={`tab-${tab.id}`}
            >
              <tab.icon size={15} />
              {tab.label}
            </button>
          ))}
        </div>
      </div>

      {/* Content */}
      <div className={styles.content} data-studio="content">
        {activeTab === 'characters' && (
          <>
            {charactersLoading ? (
              <div className={styles.gridLoading}>Loading characters...</div>
            ) : characters.length > 0 ? (
              <div className={styles.assetGrid} data-studio="character-grid">
                {characters.map((card) => (
                  <div key={card.id} data-studio="character-card">
                    <CharacterCard
                      card={card}
                      onClick={() => navigate(`/characters/${encodeURIComponent(card.id)}`, { state: { card } })}
                    />
                  </div>
                ))}
              </div>
            ) : (
              <div className={styles.emptyState} data-studio="empty-state">
                <Users size={40} opacity={0.4} />
                <h3>No Characters Yet</h3>
                <p>This user hasn't uploaded any public characters.</p>
              </div>
            )}
          </>
        )}

        {activeTab === 'worldbooks' && (
          <div className={styles.emptyState} data-studio="empty-state">
            <LayoutGrid size={40} opacity={0.4} />
            <h3>No Worldbooks Yet</h3>
            <p>This user hasn't published any worldbooks.</p>
          </div>
        )}
        {activeTab === 'presets' && (
          <div className={styles.emptyState} data-studio="empty-state">
            <Settings size={40} opacity={0.4} />
            <h3>No Presets Yet</h3>
            <p>When preset sharing launches, this creator's published generation settings will appear here.</p>
          </div>
        )}
        {activeTab === 'themes' && (
          <div className={styles.emptyState} data-studio="empty-state">
            <Palette size={40} opacity={0.4} />
            <h3>No Themes Yet</h3>
            <p>When theme support launches, this creator's custom color palettes and UI themes will appear here.</p>
          </div>
        )}
      </div>
    </div>
  );
};

export default DefaultProfileTemplate;
