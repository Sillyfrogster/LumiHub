import { useState, useRef, useEffect } from 'react';
import { Sparkles, Check, Loader2, WifiOff, ChevronDown, BookOpen, RefreshCw } from 'lucide-react';
import { useAuth } from '../../hooks/useAuth';
import { useLinkedInstances, useInstallToLumiverse } from '../../hooks/useLumiverse';
import { useIsInstalled, useIsWorldBookInstalled, getCardSlug, dismissInstallGuess } from '../../hooks/useInstallManifest';
import { useQueryClient } from '@tanstack/react-query';
import type { UnifiedCharacterCard } from '../../types/character';
import type { UnifiedWorldBook } from '../../types/worldbook';
import styles from './InstallButton.module.css';

interface Props {
  characterId: string;
  source: 'lumihub' | 'chub';
  card?: UnifiedCharacterCard;
  worldBook?: UnifiedWorldBook;
  hasEmbeddedLorebook?: boolean;
  className?: string;
}

const InstallButton: React.FC<Props> = ({ characterId, source, card, worldBook, hasEmbeddedLorebook, className }) => {
  const { isAuthenticated } = useAuth();
  const { data: instances } = useLinkedInstances();
  const installMutation = useInstallToLumiverse();
  const { isInstalled: isCharInstalled, isGuess: isCharGuess } = useIsInstalled(card);
  const { isInstalled: isWbInstalled } = useIsWorldBookInstalled(worldBook);
  const [dismissed, setDismissed] = useState(false);
  const isInstalled = (isCharInstalled && !(isCharGuess && dismissed)) || isWbInstalled;
  const isGuess = isCharGuess && !dismissed;
  const queryClient = useQueryClient();
  const [success, setSuccess] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  const onlineInstances = instances?.filter((i) => i.is_online) ?? [];
  const hasLinked = (instances?.length ?? 0) > 0;
  const hasOnline = onlineInstances.length > 0;
  const disabled = !hasOnline || installMutation.isPending;

  useEffect(() => {
    if (!menuOpen) return;
    const handler = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [menuOpen]);

  const handleDismissGuess = () => {
    if (card) dismissInstallGuess(getCardSlug(card));
    setDismissed(true);
  };

  const doInstall = async (includeWorldbook: boolean) => {
    setMenuOpen(false);
    if (disabled) return;

    const targetInstance = onlineInstances[0];
    try {
      const result = await installMutation.mutateAsync({
        instanceId: targetInstance.id,
        characterId,
        source,
        includeWorldbook,
        chubSlug: source === 'chub' ? characterId.toLowerCase() : undefined,
      });

      if (result.success) {
        setSuccess(true);
        setTimeout(() => setSuccess(false), 2000);
        // Refresh manifest so the button updates to "Update Card".
        // Delay to allow Lumiverse's debounced manifest_sync (5s) to reach LumiHub first.
        setTimeout(() => {
          queryClient.invalidateQueries({ queryKey: ['link', 'manifest'] });
        }, 6000);
      }
    } catch {
      // Error is available via installMutation.error
    }
  };

  const handleClick = () => {
    if (hasEmbeddedLorebook && hasOnline && !installMutation.isPending) {
      setMenuOpen(!menuOpen);
    } else {
      doInstall(false);
    }
  };

  const isWorldBook = !!worldBook;
  const updateLabel = isWorldBook ? 'Update Book' : 'Update Card';

  const getLabel = () => {
    if (installMutation.isPending) return isInstalled ? 'Updating...' : 'Installing...';
    if (success) return isInstalled ? 'Updated' : 'Installed';
    if (installMutation.isError || (installMutation.data && !installMutation.data.success)) return isInstalled ? 'Update Failed' : 'Install Failed';
    return isInstalled ? updateLabel : 'Install to Lumiverse';
  };

  const getIcon = () => {
    if (installMutation.isPending) return <Loader2 size={16} className="spin" />;
    if (success) return <Check size={16} />;
    if (!isAuthenticated || !hasOnline) return <WifiOff size={16} />;
    return isInstalled ? <RefreshCw size={16} /> : <Sparkles size={16} />;
  };

  const getTitle = () => {
    if (!isAuthenticated) return 'Sign in to install directly to your Lumiverse instance';
    if (!hasLinked) return 'Link a Lumiverse instance in your settings to enable direct install';
    if (!hasOnline) return 'Your Lumiverse instance is offline';
    return `Install to ${onlineInstances[0].instance_name}`;
  };

  const showDropdownArrow = hasEmbeddedLorebook && hasOnline && !installMutation.isPending && !success;

  return (
    <div className={styles.installWrap} ref={menuRef}>
      <button
        className={className}
        onClick={handleClick}
        disabled={disabled}
        title={getTitle()}
      >
        {getIcon()}
        {getLabel()}
        {showDropdownArrow && <ChevronDown size={12} className={styles.chevron} />}
      </button>

      {menuOpen && (
        <div className={styles.dropdown}>
          <button
            className={styles.dropdownOption}
            onMouseDown={(e) => { e.preventDefault(); doInstall(false); }}
          >
            <Sparkles size={14} />
            <span>Install Character</span>
          </button>
          <button
            className={styles.dropdownOption}
            onMouseDown={(e) => { e.preventDefault(); doInstall(true); }}
          >
            <BookOpen size={14} />
            <span>Install with Lorebook</span>
          </button>
        </div>
      )}

      {isGuess && !installMutation.isPending && !success && (
        <button className={styles.guessHint} onClick={handleDismissGuess}>
          Not this card?
        </button>
      )}
    </div>
  );
};

export default InstallButton;
