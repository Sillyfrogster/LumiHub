import { useCallback } from 'react';
import AssetManager from '../profile/AssetManager';
import { useProfileStudioStore } from '../../store/useProfileStudioStore';

const AssetsTab = () => {
  const { activeTab, setTab, draftHtml, draftCss, setDraftHtml, setDraftCss } = useProfileStudioStore();

  const handleInsertUrl = useCallback(
    (url: string) => {
      const assetUrl = url.startsWith('/') ? url : `/${url}`;

      // If coming from CSS tab, insert as url() value
      // If coming from HTML tab, insert as <img> tag
      // Default: append to CSS since that's most common use
      if (activeTab === 'html') {
        setDraftHtml(draftHtml + `\n<img src="${assetUrl}" alt="" />`);
        setTab('html');
      } else {
        setDraftCss(draftCss + `\n/* Inserted asset: ${assetUrl} */\n/* background-image: url('${assetUrl}'); */`);
        setTab('css');
      }
    },
    [activeTab, draftHtml, draftCss, setDraftHtml, setDraftCss, setTab]
  );

  return <AssetManager onInsertUrl={handleInsertUrl} />;
};

export default AssetsTab;
