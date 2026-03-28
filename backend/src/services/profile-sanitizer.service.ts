import DOMPurify from 'isomorphic-dompurify';

const ALLOWED_TAGS = [
    'div', 'span', 'p', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
    'a', 'img', 'video', 'source', 'section', 'article',
    'header', 'footer', 'nav', 'ul', 'ol', 'li',
    'strong', 'em', 'b', 'i', 'u', 's', 'br', 'hr',
    'table', 'thead', 'tbody', 'tfoot', 'tr', 'th', 'td',
    'figure', 'figcaption', 'blockquote', 'pre', 'code',
    'dl', 'dt', 'dd', 'abbr', 'mark', 'small', 'sub', 'sup',
    'details', 'summary', 'picture',
];

const FORBID_TAGS = [
    'script', 'iframe', 'object', 'embed', 'applet',
    'form', 'input', 'textarea', 'select', 'button',
    'link', 'meta', 'base', 'style', 'noscript',
];

const ALLOWED_ATTR = [
    'class', 'id', 'href', 'src', 'alt', 'title', 'target', 'rel', 'style',
    'width', 'height', 'loading', 'decoding',
    'autoplay', 'muted', 'loop', 'playsinline', 'poster', 'controls', 'preload',
    'colspan', 'rowspan', 'scope',
    'open',
    'type',
];

export function sanitizeProfileHtml(html: string): string {
    return DOMPurify.sanitize(html, {
        ALLOWED_TAGS,
        FORBID_TAGS,
        ALLOWED_ATTR,
        ALLOW_DATA_ATTR: true,
        FORBID_CONTENTS: ['script', 'style'],
        ADD_ATTR: ['target'],
    });
}

export function sanitizeProfileCss(css: string): string {
    let safe = css;

    safe = safe.replace(/expression\s*\(/gi, '');
    safe = safe.replace(/url\s*\(\s*['"]?\s*javascript\s*:/gi, 'url(');
    safe = safe.replace(/url\s*\(\s*['"]?\s*data\s*:\s*text\/html/gi, 'url(');
    safe = safe.replace(/behavior\s*:/gi, '/* blocked */');
    safe = safe.replace(/-moz-binding\s*:/gi, '/* blocked */');
    safe = safe.replace(/@import\s+[^;]+;?/gi, '/* @import blocked */');

    return safe;
}
