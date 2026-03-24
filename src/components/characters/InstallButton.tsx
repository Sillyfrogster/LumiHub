import { useState } from 'react';
import { Sparkles, Check, Loader2, WifiOff } from 'lucide-react';
import { useAuth } from '../../hooks/useAuth';
import { useLinkedInstances, useInstallToLumiverse } from '../../hooks/useLumiverse';

interface Props {
  characterId: string;
  source: 'lumihub' | 'chub';
  className?: string;
}

const InstallButton: React.FC<Props> = ({ characterId, source, className }) => {
  const { isAuthenticated } = useAuth();
  const { data: instances } = useLinkedInstances();
  const installMutation = useInstallToLumiverse();
  const [success, setSuccess] = useState(false);

  const onlineInstances = instances?.filter((i) => i.is_online) ?? [];
  const hasLinked = (instances?.length ?? 0) > 0;
  const hasOnline = onlineInstances.length > 0;

  const handleClick = async () => {
    if (!hasOnline || installMutation.isPending) return;

    // Use first online instance (could add picker for multiple in the future)
    const targetInstance = onlineInstances[0];

    try {
      const result = await installMutation.mutateAsync({
        instanceId: targetInstance.id,
        characterId,
        source,
      });

      if (result.success) {
        setSuccess(true);
        setTimeout(() => setSuccess(false), 2000);
      }
    } catch {
      // Error is available via installMutation.error
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

  return (
    <button
      className={className}
      onClick={handleClick}
      disabled={!hasOnline || installMutation.isPending}
      title={getTitle()}
    >
      {getIcon()}
      {getLabel()}
    </button>
  );
};

export default InstallButton;
