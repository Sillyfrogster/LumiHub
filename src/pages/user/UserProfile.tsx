import { useMemo, useEffect, useCallback, useRef } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import { User, Edit3 } from 'lucide-react';
import DOMPurify from 'dompurify';
import { useUserProfile } from '../../hooks/useUserProfile';
import { useAuth } from '../../hooks/useAuth';
import { useQueryClient } from '@tanstack/react-query';
import { saveProfile, resetProfile } from '../../api/profile';
import { useProfileStudioStore } from '../../store/useProfileStudioStore';
import { useElementPicker } from '../../hooks/useElementPicker';
import { generateRuleStub } from '../../utils/selectorGenerator';
import DefaultProfileTemplate from '../../components/profile/DefaultProfileTemplate';
import ProfileStudioPanel from '../../components/profile-studio/ProfileStudioPanel';
import ElementHighlight from '../../components/profile-studio/ElementHighlight';
import BeginnerContextMenu from '../../components/profile-studio/BeginnerContextMenu';
import styles from './UserProfile.module.css';

const STYLE_TAG_ID = 'lumihub-profile-custom-css';

interface UserProfileProps {
  previewMode?: boolean;
  previewDiscordId?: string;
}

const UserProfile = ({ previewMode = false, previewDiscordId }: UserProfileProps) => {
  const { discordId: routeDiscordId } = useParams<{ discordId: string }>();
  const [searchParams] = useSearchParams();
  const discordId = previewMode ? previewDiscordId : routeDiscordId;
  const { user } = useAuth();
  const queryClient = useQueryClient();

  const { data: profile, isLoading: profileLoading, error: profileError } = useUserProfile(discordId || '');

  const studio = useProfileStudioStore();
  const debounceRef = useRef<ReturnType<typeof setTimeout>>();

  const isOwner = user?.discordId === profile?.discordId;
  const studioParam = searchParams.get('studio');
  const forceReset = studioParam === 'reset';
  const forceStudio = studioParam === '1' || forceReset;

  const handleElementSelect = useCallback(
    (element: HTMLElement) => {
      if (studio.editorMode === 'advanced') {
        const stub = generateRuleStub(element);
        studio.setDraftCss(studio.draftCss + '\n' + stub);
        studio.setTab('css');
      }
      if (studio.editorMode === 'beginner' && studio.isPickerActive) {
        studio.togglePicker();
      }
    },
    [studio.editorMode, studio.draftCss, studio.isPickerActive]
  );

  const {
    hoveredElement,
    hoveredRect,
    selectedElement,
    selectedRect,
    clearSelection,
  } = useElementPicker({
    isActive: studio.isOpen && studio.isPickerActive,
    onDeactivate: () => studio.togglePicker(),
    onSelect: handleElementSelect,
  });

  useEffect(() => {
    if (forceReset && isOwner) {
      resetProfile().then(() => {
        queryClient.invalidateQueries({ queryKey: ['userProfile', discordId] });
      });
    }
  }, [forceReset, isOwner, discordId, queryClient]);

  useEffect(() => {
    if (forceStudio && isOwner && profile && !studio.isOpen) {
      studio.open(profile.customHtml, profile.customCss);
    }
  }, [forceStudio, isOwner, profile]);

  useEffect(() => {
    if (!isOwner || previewMode) return;

    const handler = (e: KeyboardEvent) => {
      if (e.ctrlKey && e.shiftKey && e.key === 'E') {
        e.preventDefault();
        if (studio.isOpen) {
          studio.close();
        } else if (profile) {
          studio.open(profile.customHtml, profile.customCss);
        }
      }
    };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, [isOwner, previewMode, studio.isOpen, profile]);

  useEffect(() => {
    const cssContent = studio.isOpen ? studio.draftCss : (profile?.customCss || '');

    if (forceStudio && !studio.isOpen) {
      const existing = document.getElementById(STYLE_TAG_ID);
      if (existing) existing.remove();
      return;
    }

    if (!cssContent) {
      const existing = document.getElementById(STYLE_TAG_ID);
      if (existing) existing.remove();
      return;
    }

    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      let styleTag = document.getElementById(STYLE_TAG_ID) as HTMLStyleElement | null;
      if (!styleTag) {
        styleTag = document.createElement('style');
        styleTag.id = STYLE_TAG_ID;
        document.head.appendChild(styleTag);
      }
      styleTag.textContent = cssContent;
    }, studio.isOpen ? 300 : 0);

    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [studio.isOpen, studio.draftCss, profile?.customCss, forceStudio]);

  useEffect(() => {
    return () => {
      const tag = document.getElementById(STYLE_TAG_ID);
      if (tag) tag.remove();
    };
  }, []);

  const displayHtml = useMemo(() => {
    if (forceStudio && !studio.isOpen) return null;

    const rawHtml = studio.isOpen ? studio.draftHtml : (profile?.customHtml || '');
    if (!rawHtml) return null;

    return DOMPurify.sanitize(rawHtml, {
      FORBID_TAGS: ['script', 'iframe', 'object', 'embed', 'form', 'input', 'textarea', 'select', 'button', 'link', 'meta', 'base'],
      FORBID_ATTR: ['onerror', 'onload', 'onclick', 'onmouseover', 'onfocus', 'onblur', 'onsubmit', 'onchange', 'oninput', 'onkeydown', 'onkeyup', 'onkeypress'],
      ALLOW_DATA_ATTR: true,
    });
  }, [studio.isOpen, studio.draftHtml, profile?.customHtml, forceStudio]);

  const handleSave = useCallback(async () => {
    studio.setSaving(true);
    try {
      await saveProfile({ html: studio.draftHtml, css: studio.draftCss });
      studio.markClean();
      queryClient.invalidateQueries({ queryKey: ['userProfile', discordId] });
    } catch (err) {
      console.error('Failed to save profile:', err);
      alert('Failed to save profile. Please try again.');
    } finally {
      studio.setSaving(false);
    }
  }, [studio.draftHtml, studio.draftCss, discordId, queryClient]);

  const handleReset = useCallback(async () => {
    try {
      await resetProfile();
      studio.open(null, null);
      queryClient.invalidateQueries({ queryKey: ['userProfile', discordId] });
    } catch (err) {
      console.error('Failed to reset profile:', err);
    }
  }, [discordId, queryClient]);

  if (profileLoading) {
    return <div className={styles.loadingState}>Loading profile...</div>;
  }

  if (profileError || !profile) {
    return (
      <div className={styles.notFoundState}>
        <User size={64} opacity={0.5} />
        <h1>User Not Found</h1>
        <p>This profile does not exist or has been removed.</p>
      </div>
    );
  }

  return (
    <div className={styles.pageWrapper} data-studio="page-wrapper">
      {/* Profile Studio open button */}
      {isOwner && !previewMode && !studio.isOpen && (
        <button
          className={styles.editThemeBtn}
          onClick={() => studio.open(profile.customHtml, profile.customCss)}
          data-studio="edit-button"
        >
          <Edit3 size={16} /> Edit Profile
        </button>
      )}

      {/* Render custom HTML or default template */}
      {displayHtml ? (
        <div
          className={styles.customContent}
          data-studio="custom-content"
          dangerouslySetInnerHTML={{ __html: displayHtml }}
        />
      ) : (
        <DefaultProfileTemplate profile={profile} />
      )}

      {/* Element picker highlight overlays */}
      {studio.isOpen && (studio.isPickerActive || (studio.editorMode === 'beginner' && selectedElement)) && (
        <ElementHighlight
          hoveredElement={studio.isPickerActive ? hoveredElement : null}
          hoveredRect={studio.isPickerActive ? hoveredRect : null}
          selectedElement={selectedElement}
          selectedRect={selectedRect}
        />
      )}

      {/* Beginner mode context menu for selected element */}
      {studio.isOpen && studio.editorMode === 'beginner' && selectedElement && selectedRect && (
        <BeginnerContextMenu element={selectedElement} rect={selectedRect} onClose={clearSelection} />
      )}

      {/* Profile Studio floating panel */}
      {studio.isOpen && (
        <ProfileStudioPanel
          onSave={handleSave}
          onReset={handleReset}
          onSelectElement={handleElementSelect}
        />
      )}
    </div>
  );
};

export default UserProfile;
