export function recoverUnauthorized(error: unknown, revoke: () => void): boolean {
  if (typeof error !== 'object' || error === null || !('status' in error) || error.status !== 401) return false;
  revoke();
  return true;
}

// Browsers hide an HTTP 401 during WS upgrade behind close code 1006. Only an
// authenticated HTTP probe can distinguish expiry from ordinary connectivity.
// A pending wallet session can return 200 but cannot authorize world access.
interface ReconnectIdentity {
  identity: { id: string; status: string } | null;
  user: { id: string } | null;
}
export async function reconnectSession(probe: (signal: AbortSignal) => Promise<ReconnectIdentity>, expectedCharacterId: string, timeoutMs = 4000): Promise<'expired' | 'retry'> {
  const controller = new AbortController();
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    return await Promise.race([
      probe(controller.signal).then(session => session?.identity?.status === 'active' && session.identity.id === expectedCharacterId && session.user?.id === expectedCharacterId ? 'retry' as const : 'expired' as const).catch(error => recoverUnauthorized(error, () => {}) ? 'expired' as const : 'retry' as const),
      new Promise<'retry'>(resolve => { timer = setTimeout(() => { controller.abort(); resolve('retry'); }, timeoutMs); }),
    ]);
  } finally { clearTimeout(timer); }
}
