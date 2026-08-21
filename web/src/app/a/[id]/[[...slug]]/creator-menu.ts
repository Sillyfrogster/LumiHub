export const OPEN_CREATOR_MENU = "illarin:open-creator-menu";

export function openCreatorMenu() {
  window.dispatchEvent(new Event(OPEN_CREATOR_MENU));
}
