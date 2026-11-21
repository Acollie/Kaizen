// Small formatting helpers shared by components.

/** "pkg/review/context.go" -> { dir: "pkg/review/", base: "context.go" } */
export function splitPath(path: string): { dir: string; base: string } {
  const index = path.lastIndexOf('/');
  return index === -1 ? { dir: '', base: path } : { dir: path.slice(0, index + 1), base: path.slice(index + 1) };
}

export function truncateMiddle(text: string, limit: number): string {
  if (text.length <= limit) return text;
  const head = Math.ceil((limit - 1) / 2);
  const tail = Math.floor((limit - 1) / 2);
  return `${text.slice(0, head)}…${text.slice(text.length - tail)}`;
}

export function truncateEnd(text: string, limit: number): string {
  return text.length <= limit ? text : `${text.slice(0, Math.max(0, limit - 1))}…`;
}

export function formatClock(value: string | number | Date): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

export function plural(count: number, singular: string, pluralForm = `${singular}s`): string {
  return `${count} ${count === 1 ? singular : pluralForm}`;
}

/** Only http(s) URLs may become links (PR and AI text is untrusted) */
export function isSafeHttpUrl(value: string): boolean {
  try {
    const url = new URL(value);
    return url.protocol === 'http:' || url.protocol === 'https:';
  } catch {
    return false;
  }
}

/** Receiver-qualified display name: "Analyzer.GetFileChurn" */
export function qualifiedName(fn: { name: string; receiver_type?: string }): string {
  const receiver = fn.receiver_type ? fn.receiver_type.replace('*', '') : '';
  return receiver ? `${receiver}.${fn.name}` : fn.name;
}
