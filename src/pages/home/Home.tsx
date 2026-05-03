import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Users, Book, Palette, Sparkles, ArrowRight, Plug, Zap, Shield, TrendingUp } from 'lucide-react';
import { listCharacters } from '../../api/characters';
import { fromLumiHub, type UnifiedCharacterCard } from '../../types/character';
import { useAuth } from '../../hooks/useAuth';
import CharacterCard from '../../components/characters/CharacterCard';
import LazyImage from '../../components/shared/LazyImage';
import styles from './Home.module.css';

const Home = () => {
  const navigate = useNavigate();
  const { user } = useAuth();
  const [trending, setTrending] = useState<UnifiedCharacterCard[]>([]);
  const [trendingLoading, setTrendingLoading] = useState(true);

  useEffect(() => {
    listCharacters({ sort: 'downloads', order: 'desc', limit: 12 })
      .then((res) => {
        let cards = res.data.map(fromLumiHub);
        if (!user?.settings?.nsfwEnabled) {
          cards = cards.filter(c => !c.nsfw);
        }
        setTrending(cards);
      })
      .catch((err) => console.error('Failed to fetch popular characters:', err))
      .finally(() => setTrendingLoading(false));
  }, [user?.settings?.nsfwEnabled]);

  return (
    <div className={styles.page}>

      {/* Compact Hero */}
      <div className={styles.heroContainer}>
        <section className={styles.hero}>
          <h1 className={styles.heroTitle}>
            Welcome to <span className={styles.heroAccent}>LumiHub</span>
          </h1>
          <p className={styles.heroSub}>
            I'm Lumia's pink-haired twin. Discover, share, and effortlessly install assets directly into your local universe.
          </p>
          <div className={styles.heroCtas}>
            <Link to="/characters" className={styles.ctaPrimary}>
              Browse Characters
              <ArrowRight size={16} />
            </Link>
          </div>
        </section>
        <LazyImage src="/lumihub-mascot.png" alt="LumiHub Mascot" className={styles.heroMascot} containerClassName={styles.heroMascotWrap} style={{ height: 'auto', objectFit: 'contain' }} />
      </div>

      {/* Trending Characters */}
      <section className={styles.section}>
        <div className={styles.sectionHeader}>
          <TrendingUp size={18} className={styles.sectionIcon} />
          <h2 className={styles.sectionTitle}>Popular</h2>
          <Link to="/characters" className={styles.seeAll}>
            See all <ArrowRight size={14} />
          </Link>
        </div>

        <div className={styles.trendingRow}>
          {trendingLoading ? (
            Array.from({ length: 6 }).map((_, i) => (
              <div key={i} className={styles.trendingSkeleton} />
            ))
          ) : trending.length > 0 ? (
            trending.map((card) => (
              <div key={card.id} className={styles.trendingCard}>
                <CharacterCard
                  card={card}
                  onClick={() => navigate(`/characters/${encodeURIComponent(card.id)}`, { state: { card } })}
                />
              </div>
            ))
          ) : (
            <p className={styles.trendingEmpty}>No trending characters available right now.</p>
          )}
        </div>
      </section>

      {/* Category Chips */}
      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Browse by type</h2>
        <div className={styles.categoryChips}>
          <Link to="/characters" className={styles.chip}>
            <Users size={16} />
            Characters
          </Link>
          <Link to="/worldbooks" className={styles.chip}>
            <Book size={16} />
            Worldbooks
          </Link>
          <Link to="/themes" className={styles.chip}>
            <Palette size={16} />
            Themes
          </Link>
          <Link to="/presets" className={styles.chip}>
            <Sparkles size={16} />
            Presets
          </Link>
        </div>
      </section>

      {/* Hub Connector — slim banner */}
      <section className={styles.connectorBanner}>
        <div className={styles.connectorLeft}>
          <div className={styles.connectorIconWrap}>
            <Plug size={20} />
          </div>
          <div>
            <h3 className={styles.connectorTitle}>Hub Connector</h3>
            <p className={styles.connectorDesc}>
              Install the extension in Lumiverse for one-click asset delivery.
            </p>
          </div>
        </div>
        <div className={styles.connectorFeatures}>
          <div className={styles.connectorFeature}>
            <Zap size={14} />
            <span>Instant install</span>
          </div>
          <div className={styles.connectorFeature}>
            <Shield size={14} />
            <span>Secure</span>
          </div>
        </div>
      </section>
    </div>
  );
};

export default Home;
