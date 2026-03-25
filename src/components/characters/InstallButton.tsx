import { useState, useRef, useEffect } from 'react';
import { Sparkles, Check, Loader2, WifiOff, ChevronDown, BookOpen } from 'lucide-react';
import { useAuth } from '../../hooks/useAuth';
import { useLinkedInstances, useInstallToLumiverse } from '../../hooks/useLumiverse';
import styles from './InstallButton.module.css';

interface Props {
  characterId: string;
  source: 'lumihub' | 'chub';
  hasEmbeddedLorebook?: boolean;
  className?: string;
}

const InstallButton: React.FC<Props> = ({ characterId, source, hasEmbeddedLorebook, className }) => {
  const { isAuthenticated } = useAuth();
  const { data: instances } = useLinkedInstances();
  const installMutation = useInstallToLumiverse();
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
      });

      if (result.success) {
        setSuccess(true);
        setTimeout(() => setSuccess(false), 2000);
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

  const getLabel = () => {
    if (installMutation.isPending) return 'Installing...';
    if (success) return 'Installed';
    if (installMutation.isError || (installMutation.data && !installMutation.data.success)) return 'Install Failed';
    return 'Install to Lumiverse';
  };

  const getIcon = () => {
    if (installMutation.isPending) return <Loader2 size={16} className="spin" />;
    if (success) return <Check size={16} />;
    if (!isAuthenticated || !hasOnline) return <WifiOff size={16} />;
    return <Sparkles size={16} />;
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
    </div>
  );
};

export default InstallButton;
