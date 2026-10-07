import type { WalletAdapter, WalletConnection } from './wallet/types.ts';
import { FRACTAL_CHAIN } from './wallet/UniSatWalletAdapter.ts';

// Cookie mutations must finish before cleanup, and cleanup before a later login.
// Cancellation invalidates publication immediately but never abandons a request
// whose eventual response can still set or clear the shared session cookie.
export class IdentityLifecycle {
  private generation = 0;
  private tail: Promise<unknown> = Promise.resolve();
  private needsLogout = false;
  private logout: () => Promise<unknown>;
  constructor(logout: () => Promise<unknown>) { this.logout = logout; }
  snapshot() { return this.generation; }
  begin() { return ++this.generation; }
  current(token: number) { return token === this.generation; }
  assert(token: number) {
    if (!this.current(token)) throw new Error('Wallet changed or request cancelled. Retry authentication.');
  }
  cancel() { this.generation++; }
  private enqueue<T>(work: () => Promise<T>): Promise<T> {
    const next = this.tail.then(work, work);
    this.tail = next.catch(() => {});
    return next;
  }
  private async clean() {
    if (!this.needsLogout) return;
    await this.logout();
    this.needsLogout = false;
  }
  run<T>(token: number, work: () => Promise<T>): Promise<T> {
    return this.enqueue(async () => {
      this.assert(token);
      await this.clean();
      this.assert(token);
      return work();
    });
  }
  revoke() {
    this.cancel();
    this.needsLogout = true;
    return this.enqueue(() => this.clean());
  }
}

export async function checkedWalletConnection(
  adapter: Pick<WalletAdapter, 'getAddress' | 'getNetwork' | 'getPublicKey'>,
  assertCurrent: () => void,
  expectedAddress?: string,
): Promise<WalletConnection> {
  const address = await adapter.getAddress(); assertCurrent();
  const network = await adapter.getNetwork(); assertCurrent();
  const publicKey = await adapter.getPublicKey(); assertCurrent();
  // Re-read both values to also catch a wallet lacking working event delivery.
  if (await adapter.getAddress() !== address) throw new Error('Wallet account changed. Authenticate again.');
  assertCurrent();
  if (await adapter.getNetwork() !== network) throw new Error('Wallet network changed. Authenticate again.');
  assertCurrent();
  if (expectedAddress && (address !== expectedAddress || network !== FRACTAL_CHAIN)) {
    throw new Error('Reconnect the matching Fractal wallet.');
  }
  return { address, network, publicKey };
}
