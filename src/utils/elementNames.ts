/** Map from data-studio attribute values to human-readable element names. */
const ELEMENT_NAMES: Record<string, string> = {
  // Layout
  'shell': 'Page Shell',
  'header': 'Navigation Bar',
  'nav': 'Navigation Links',
  'main': 'Main Content Area',

  // Profile page
  'page-wrapper': 'Profile Page',
  'edit-button': 'Edit Button',
  'custom-content': 'Custom Content',

  // Default template
  'profile-container': 'Profile Container',
  'banner': 'Profile Banner',
  'profile-header': 'Profile Header',
  'avatar': 'Avatar',
  'identity': 'Identity Section',
  'name': 'Display Name',
  'handle': 'Username Handle',
  'role-badge': 'Role Badge',
  'stats': 'Stats Row',
  'stat-uploads': 'Uploads Stat',
  'stat-downloads': 'Downloads Stat',
  'stat-joined': 'Joined Date Stat',
  'tabs': 'Content Tabs',
  'tab-characters': 'Characters Tab',
  'tab-worldbooks': 'Worldbooks Tab',
  'tab-themes': 'Themes Tab',
  'tab-presets': 'Presets Tab',
  'content': 'Content Area',
  'character-grid': 'Character Grid',
  'character-card': 'Character Card',
  'empty-state': 'Empty State',
};

/** Get a friendly display name for an element by its data-studio value. */
export function getElementName(studioValue: string): string {
  return ELEMENT_NAMES[studioValue] || studioValue.replace(/-/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
}

/** Get all known element name entries. */
export function getAllElementNames(): [string, string][] {
  return Object.entries(ELEMENT_NAMES);
}

export default ELEMENT_NAMES;
