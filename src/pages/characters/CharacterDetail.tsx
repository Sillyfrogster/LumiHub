import { useEffect, useState } from 'react';
import { useParams, useLocation, useNavigate, Link } from 'react-router-dom';
import { ArrowLeft, Download, Star, Users, ExternalLink, Image, FileText } from 'lucide-react';
import type { UnifiedCharacterCard } from '../../types/character';
import type { ChubCharacterCard } from '../../types/chub';
import type { LumiHubCharacter } from '../../types/character';
import { getCharacter } from '../../api/characters';
import { fromLumiHub } from '../../types/character';
import CharacterTabs from '../../components/characters/CharacterTabs';
import InstallButton from '../../components/characters/InstallButton';
import styles from './CharacterDetail.module.css';

const CharacterDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const location = useLocation();
  const navigate = useNavigate();

  const passedCard = (location.state as { card?: UnifiedCharacterCard })?.card ?? null;

  const [card, setCard] = useState<UnifiedCharacterCard | null>(passedCard);
  const [loading, setLoading] = useState(!passedCard);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (passedCard || !id) return;

    setLoading(true);
    getCharacter(id)
      .then((res) => setCard(fromLumiHub(res.data)))
      .catch(() => setError('Character not found.'))
      .finally(() => setLoading(false));
  }, [id, passedCard]);

  if (loading) {
    return (
      <div className={styles.page}>
        <div className={styles.loadingState}>
          <div className={styles.loadingSkeleton} />
        </div>
      </div>
    );
  }

  if (error || !card) {
    return (
      <div className={styles.page}>
        <div className={styles.errorState}>
          <p>{error || 'Character not found.'}</p>
          <button className={styles.backBtnLarge} onClick={() => navigate('/characters')}>
            <ArrowLeft size={16} /> Back to Characters
          </button>
        </div>
      </div>
    );
  }

  const isChub = card.source === 'chub';
  const chubData = isChub ? (card.raw as ChubCharacterCard) : null;
  const lumiData = !isChub ? (card.raw as LumiHubCharacter) : null;

  const formattedDownloads = card.downloads > 1000
    ? `${(card.downloads / 1000).toFixed(1)}k`
    : String(card.downloads);

  const handleDownloadPng = () => {
    if (card.avatarUrl) {
      const a = document.createElement('a');
      a.href = card.avatarUrl;
      a.download = `${card.name}_card.png`;
      a.click();
    }
  };

  return (
    <div className={styles.page}>
      <button className={styles.backBtn} onClick={() => navigate(-1)}>
        <ArrowLeft size={16} />
        <span>Back</span>
      </button>

      <div className={styles.layout}>
        <div className={styles.imageColumn}>
          <div className={styles.imageWrap}>
            {card.avatarUrl ? (
              <img src={card.avatarUrl} alt={card.name} className={styles.image} />
            ) : (
              <div className={styles.imagePlaceholder}>{card.name.charAt(0)}</div>
            )}
          </div>
        </div>

        <div className={styles.contentColumn}>
          <h1 className={styles.name}>{card.name}</h1>
          <p className={styles.creator}>
            by{' '}
            {card.creatorDiscordId ? (
              <Link to={`/user/${card.creatorDiscordId}`} className={styles.creatorLink}>
                {card.creator}
              </Link>
            ) : (
              card.creator
            )}
            {lumiData?.character_version && (
              <span className={styles.version}> · v{lumiData.character_version}</span>
            )}
          </p>

          <div className={styles.stats}>
            <span className={styles.statChip}><Download size={13} />{formattedDownloads} downloads</span>
            {card.rating !== null && <span className={styles.statChip}><Star size={13} />{card.rating.toFixed(1)}</span>}
            {isChub && chubData?.interactions !== undefined && (
              <span className={styles.statChip}><Users size={13} />{chubData.interactions.toLocaleString()} chats</span>
            )}
            {chubData?.tokenCount !== undefined && (
              <span className={styles.statChip}><FileText size={13} />{chubData.tokenCount.toLocaleString()} tokens</span>
            )}
          </div>

          {card.tags.length > 0 && (
            <div className={styles.tagsRow}>
              {card.tags.map((tag, i) => (
                <span key={i} className={styles.tag}>{tag}</span>
              ))}
            </div>
          )}

          <div className={styles.actions}>
            {isChub ? (
              <>
                <InstallButton
                  characterId={card.id}
                  source="chub"
                  className={styles.installBtn}
                />
                <a href={chubData?.pageUrl} target="_blank" rel="noreferrer" className={styles.secondaryBtn}>
                  <ExternalLink size={14} /> View on Chub
                </a>
              </>
            ) : (
              <>
                <InstallButton
                  characterId={card.id}
                  source="lumihub"
                  className={styles.installBtn}
                />
                <button className={styles.secondaryBtn} onClick={handleDownloadPng}>
                  <Image size={14} /> Download PNG
                </button>
              </>
            )}
          </div>

          <CharacterTabs card={card} />
        </div>
      </div>
    </div>
  );
};

export default CharacterDetail;
