import type { Challenge, IdentityConfig, IdentityOptions, IdentitySession } from './types';
export class IdentityAPIError extends Error {
  readonly status: number;
  readonly code?: string;
  constructor(message: string, status: number, code?: string) { super(message); this.status = status; this.code = code; }
}
async function request<T>(path: string, body?: unknown, signal?: AbortSignal): Promise<T> {
  const response = await fetch('/api' + path, {
    method: body === undefined ? 'GET' : 'POST', credentials: 'include', signal,
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const result = await response.json().catch(() => ({ error: 'Identity service unavailable. Retry.' }));
  if (!response.ok) throw new IdentityAPIError(result.error || 'Identity verification failed.', response.status, result.code);
  return result;
}
export const identityAPI = {
  config: () => request<IdentityConfig>('/identity/config'),
  me: (signal?: AbortSignal) => request<IdentitySession>('/identity/me', undefined, signal),
  challenge: (body: { address: string; publicKey?: string; network: string }) => request<Challenge>('/auth/wallet/challenge', body),
  verify: (body: { address: string; publicKey?: string; network: string; nonce: string; signature: string }) => request<IdentitySession>('/auth/wallet/verify', body),
  options: () => request<IdentityOptions>('/identity/options'),
  select: (body: { characterAssetId: string; btcName: string }) => request<Pick<IdentitySession, 'identity' | 'user' | 'dev'>>('/identity/select', body),
  logout: () => request<{ ok: boolean }>('/auth/wallet/logout', {}),
  dev: (persona: 'alice' | 'bob') => request<IdentitySession>('/auth/wallet/dev', { persona }),
};
