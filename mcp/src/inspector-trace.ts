export interface TracePreview {
  value: unknown;
  truncated: boolean;
}

export interface InspectorTrace {
  route?: { method: string; path: string };
  arguments?: TracePreview;
  request?: { method: string; url: string; path: string; query: TracePreview; headers: Record<string, string>; body: TracePreview };
  response?: { status: number; headers: Record<string, string>; body?: TracePreview };
  result?: TracePreview;
  error?: string;
}

export function isCredentialKey(key: string): boolean {
  return /authorization|cookie|password|passwd|secret|token|api[-_]?key|credential/i.test(key);
}

export function tracePreview(input: unknown, secrets: string[] = []): TracePreview {
  const redactText = (value: string) => {
    for (const secret of secrets) if (secret) value = value.split(secret).join('[OCULTO]');
    return value.replace(/Bearer\s+[^\s"<>]+/gi, 'Bearer [OCULTO]').replace(/eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+/g, '[OCULTO]');
  };
  const sanitize = (value: unknown, depth: number): unknown => {
    if (depth > 16) return '[Limite de profundidade]';
    if (typeof value === 'string') {
      try {
        const parsed = JSON.parse(value);
        if (parsed && typeof parsed === 'object') return JSON.stringify(sanitize(parsed, depth + 1));
      } catch {}
      return redactText(value);
    }
    if (Array.isArray(value)) return value.map(item => sanitize(item, depth + 1));
    if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, isCredentialKey(key) ? '[OCULTO]' : sanitize(item, depth + 1)]));
    return value ?? null;
  };
  let value = input;
  if (typeof value === 'string') {
    try { value = JSON.parse(value); } catch {}
  }
  const sanitized = sanitize(value, 0);
  const text = JSON.stringify(sanitized);
  return text.length > 24000 ? { value: text.slice(0, 24000), truncated: true } : { value: sanitized, truncated: false };
}
