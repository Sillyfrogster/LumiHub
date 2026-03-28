import { getElementName } from './elementNames';

interface SelectorResult {
  selector: string;
  friendlyName: string;
}

function isHashedClass(className: string): boolean {
  return /^_/.test(className) || /[A-Za-z]_[a-z0-9]{5,}$/.test(className);
}

export function generateSelector(element: HTMLElement): SelectorResult {
  const studioAttr = element.getAttribute('data-studio');
  if (studioAttr) {
    return {
      selector: `[data-studio="${studioAttr}"]`,
      friendlyName: getElementName(studioAttr),
    };
  }

  const id = element.id;
  if (id && !isHashedClass(id) && !id.startsWith('lumihub-')) {
    const tag = element.tagName.toLowerCase();
    return {
      selector: `#${CSS.escape(id)}`,
      friendlyName: `${tag}#${id}`,
    };
  }

  const classes = Array.from(element.classList).filter((c) => !isHashedClass(c));
  if (classes.length > 0) {
    const classSelector = classes.map((c) => `.${CSS.escape(c)}`).join('');
    return {
      selector: classSelector,
      friendlyName: classes.join(' '),
    };
  }

  const path = buildPathSelector(element);
  const tag = element.tagName.toLowerCase();
  return {
    selector: path,
    friendlyName: tag,
  };
}

function buildPathSelector(element: HTMLElement): string {
  const parts: string[] = [];
  let current: HTMLElement | null = element;

  while (current && current !== document.body && current !== document.documentElement) {
    const studioAttr = current.getAttribute('data-studio');
    if (studioAttr && current !== element) {
      parts.unshift(`[data-studio="${studioAttr}"]`);
      break;
    }

    const tag = current.tagName.toLowerCase();
    const parent = current.parentElement;

    if (parent) {
      const siblings = Array.from(parent.children).filter(
        (child) => child.tagName === current!.tagName
      );
      if (siblings.length > 1) {
        const index = siblings.indexOf(current) + 1;
        parts.unshift(`${tag}:nth-child(${index})`);
      } else {
        parts.unshift(tag);
      }
    } else {
      parts.unshift(tag);
    }

    current = parent as HTMLElement | null;
  }

  return parts.join(' > ');
}

export function generateRuleStub(element: HTMLElement): string {
  const { selector, friendlyName } = generateSelector(element);
  return `${selector} {\n  /* ${friendlyName} */\n  \n}\n`;
}
