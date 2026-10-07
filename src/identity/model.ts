import type { WalletAdapter } from './wallet/types.ts';
import { FRACTAL_CHAIN } from './wallet/UniSatWalletAdapter.ts';
export class IdentityAttempt {
  private generation = 0;
  begin() { return ++this.generation; }
  cancel() { this.generation++; }
  assert(token: number) { if (token !== this.generation) throw new Error('Wallet changed or request cancelled. Retry authentication.'); }
}
interface AuthAPI<T> {
  challenge(body: { address: string; publicKey?: string; network: string }): Promise<{ nonce: string; message: string }>;
  verify(body: { address: string; publicKey?: string; network: string; nonce: string; signature: string }): Promise<T>;
}
export async function verifyWallet<T>(adapter: WalletAdapter, api: AuthAPI<T>, attempt: IdentityAttempt): Promise<T> {
  const token = attempt.begin();
  const address = await adapter.getAddress();
  if (await adapter.getNetwork() !== FRACTAL_CHAIN) throw new Error('Fractal Bitcoin Mainnet is required.');
  const publicKey = await adapter.getPublicKey();
  attempt.assert(token);
  const request = { address, publicKey: publicKey || undefined, network: 'fractal-mainnet' };
  const challenge = await api.challenge(request);
  attempt.assert(token);
  const signed = await adapter.signMessage(challenge.message);
  attempt.assert(token);
  if (await adapter.getAddress() !== address || await adapter.getNetwork() !== FRACTAL_CHAIN) throw new Error('Wallet or network changed during signing. Retry authentication.');
  attempt.assert(token);
  const session = await api.verify({ ...request, nonce: challenge.nonce, signature: signed.signature });
  attempt.assert(token);
  return session;
}
export function publicCharacterName(value: { btcName?: string; username?: string; characterIdentityId?: string }) {
  if (value.btcName) return value.btcName;
  return value.characterIdentityId ? 'Explorer' : value.username || 'Explorer';
}
