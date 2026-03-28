import { FileCode, Paintbrush, Image } from 'lucide-react';
import { useProfileStudioStore, type StudioTab } from '../../store/useProfileStudioStore';
import styles from './PanelTabBar.module.css';

const TABS: { id: StudioTab; label: string; icon: React.ElementType }[] = [
  { id: 'html', label: 'HTML', icon: FileCode },
  { id: 'css', label: 'CSS', icon: Paintbrush },
  { id: 'assets', label: 'Assets', icon: Image },
];

const PanelTabBar = () => {
  const { activeTab, setTab } = useProfileStudioStore();

  return (
    <div className={styles.tabBar}>
      {TABS.map((tab) => (
        <button
          key={tab.id}
          className={`${styles.tab} ${activeTab === tab.id ? styles.tabActive : ''}`}
          onClick={() => setTab(tab.id)}
        >
          <tab.icon size={14} />
          {tab.label}
        </button>
      ))}
    </div>
  );
};

export default PanelTabBar;
