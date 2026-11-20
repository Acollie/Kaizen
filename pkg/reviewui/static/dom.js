// DOM helpers. Everything here builds nodes with textContent; nothing parses HTML,
// so untrusted PR and AI text can never become markup.

/**
 * Create an element. attrs: class, dataset (object), text, title, on<Event> handlers,
 * style vars (object of CSS custom properties) and plain attributes.
 * Children may be nodes, strings (as text) or arrays; null/false are skipped.
 */
export function h(tag, attrs = {}, ...children) {
  const element = document.createElement(tag);
  for (const [key, value] of Object.entries(attrs || {})) {
    applyAttribute(element, key, value);
  }
  appendChildren(element, children);
  return element;
}

function applyAttribute(element, key, value) {
  if (value === null || value === undefined || value === false) return;
  if (key === 'class') element.className = value;
  else if (key === 'text') element.textContent = String(value);
  else if (key === 'dataset') Object.assign(element.dataset, value);
  else if (key === 'vars') {
    for (const [name, cssValue] of Object.entries(value)) element.style.setProperty(name, cssValue);
  } else if (key.startsWith('on') && typeof value === 'function') {
    element.addEventListener(key.slice(2).toLowerCase(), value);
  } else if (value === true) element.setAttribute(key, '');
  else element.setAttribute(key, String(value));
}

export function appendChildren(element, children) {
  for (const child of children.flat(Infinity)) {
    if (child === null || child === undefined || child === false) continue;
    element.append(child instanceof Node ? child : document.createTextNode(String(child)));
  }
  return element;
}

export function clear(element) {
  while (element.firstChild) element.removeChild(element.firstChild);
  return element;
}

export const SEVERITIES = ['critical', 'warning', 'suggestion', 'praise'];

export function severityRank(severity) {
  const index = SEVERITIES.indexOf(severity);
  return index === -1 ? SEVERITIES.length : index;
}

/** Highest severity among findings, or null */
export function topSeverity(findings) {
  let best = null;
  for (const finding of findings) {
    if (best === null || severityRank(finding.severity) < severityRank(best)) best = finding.severity;
  }
  return best;
}

/** Count findings by severity in display order: [[severity, count], ...] (only non-zero) */
export function severityCounts(findings) {
  const counts = new Map(SEVERITIES.map((severity) => [severity, 0]));
  for (const finding of findings) {
    if (counts.has(finding.severity)) counts.set(finding.severity, counts.get(finding.severity) + 1);
  }
  return [...counts.entries()].filter(([, count]) => count > 0);
}

export function severityBadges(findings, { animateIds } = {}) {
  return severityCounts(findings).map(([severity, count]) => {
    const fresh = animateIds && findings.some((finding) => finding.severity === severity && animateIds.has(finding.id));
    return h('span', { class: `badge sev-${severity}${fresh ? ' arrived' : ''}`, title: `${count} ${severity}`, text: count });
  });
}

const RISK_STOPS = [
  [0, [139, 154, 135]],    // sage
  [50, [212, 165, 116]],   // amber
  [100, [184, 92, 80]],    // terracotta
];

/** Map a 0-100 risk score to the sage → amber → terracotta scale */
export function riskColor(score) {
  const value = Math.max(0, Math.min(100, Number(score) || 0));
  for (let index = 1; index < RISK_STOPS.length; index++) {
    const [upper, upperColor] = RISK_STOPS[index];
    if (value <= upper) {
      const [lower, lowerColor] = RISK_STOPS[index - 1];
      const ratio = (value - lower) / (upper - lower);
      const channel = (offset) => Math.round(lowerColor[offset] + (upperColor[offset] - lowerColor[offset]) * ratio);
      return `rgb(${channel(0)}, ${channel(1)}, ${channel(2)})`;
    }
  }
  return 'rgb(184, 92, 80)';
}

export function riskTier(score) {
  if (score >= 60) return 'high';
  if (score >= 30) return 'medium';
  return 'low';
}

export function riskBar(score) {
  return h('div', { class: 'risk-bar', title: `Risk ${Math.round(score)}` },
    h('span', { vars: { width: '0' } }));
}

/** Fill a risk bar's inner span (CSSOM writes are allowed by the CSP, style attributes are not) */
export function setRiskBar(bar, score) {
  const fill = bar.firstChild;
  fill.style.width = `${Math.max(3, Math.min(100, score))}%`;
  fill.style.background = riskColor(score);
  return bar;
}

export function makeRiskBar(score) {
  return setRiskBar(riskBar(score), score);
}

export function riskChip(score) {
  const chip = h('span', { class: 'risk-chip', text: Math.round(score), title: `Risk score ${Math.round(score)} / 100` });
  chip.style.background = riskColor(score);
  return chip;
}

const AGENT_COLORS = ['#B85C50', '#6F8BA0', '#7E9A7A', '#C08552', '#8C6FA0', '#5E8C8C'];

export function agentColor(agent) {
  if (!agent || agent === 'lead') return AGENT_COLORS[0];
  let hash = 0;
  for (const character of agent) hash = (hash * 31 + character.charCodeAt(0)) >>> 0;
  return AGENT_COLORS[1 + (hash % (AGENT_COLORS.length - 1))];
}

export function agentTag(agent) {
  const tag = h('span', { class: 'agent-tag', text: agent || 'lead' });
  tag.style.setProperty('--agent-color', agentColor(agent));
  return tag;
}

export function splitPath(path) {
  const index = path.lastIndexOf('/');
  return index === -1 ? { dir: '', base: path } : { dir: path.slice(0, index + 1), base: path.slice(index + 1) };
}

export function formatTime(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false });
}

export function truncate(text, limit) {
  const value = String(text || '');
  return value.length <= limit ? value : `${value.slice(0, limit - 1).trimEnd()}…`;
}

export function isSafeHttpUrl(value) {
  try {
    const parsed = new URL(value);
    return parsed.protocol === 'https:' || parsed.protocol === 'http:';
  } catch {
    return false;
  }
}

/** External link that never leaks the session token through the referrer */
export function externalLink(href, text, attrs = {}) {
  if (!isSafeHttpUrl(href)) return h('span', attrs, text);
  return h('a', { ...attrs, href, target: '_blank', rel: 'noopener noreferrer' }, text);
}
