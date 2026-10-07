import type { WalletAdapter, WalletConnection, WalletSignature } from './types.ts';
export const FRACTAL_CHAIN = 'FRACTAL_BITCOIN_MAINNET';
export interface UniSatProvider {
  requestAccounts(): Promise<string[]>;
  getAccounts(): Promise<string[]>;
  getPublicKey(): Promise<string>;
  getChain(): Promise<{ enum: string }>;
  switchChain?(chain: string): Promise<{ enum: string }>;
  signMessage(message: string, type: 'bip322-simple'): Promise<string>;
  disconnect?(): Promise<void>;
  on?(event: string, handler: (...args: unknown[]) => void): void;
  removeListener?(event: string, handler: (...args: unknown[]) => void): void;
}
// This adapter is the only production module allowed to access the injected wallet.
const injected = (): UniSatProvider | undefined =>
  typeof window === 'undefined' ? undefined : (window as Window & { unisat?: UniSatProvider }).unisat;
function walletError(error: unknown): Error {
  const code = typeof error === 'object' && error ? (error as { code?: number }).code : undefined;
  if (code === 4001) return new Error('Wallet request rejected. You can retry. / 已取消，可重试');
  return new Error('Wallet request failed or was cancelled. Unlock UniSat and retry.');
}
export class UniSatWalletAdapter implements WalletAdapter {
  readonly id = 'unisat';
  readonly name = 'UniSat Wallet';
  private readonly getProvider: () => UniSatProvider | undefined;
  constructor(getProvider: () => UniSatProvider | undefined = injected) { this.getProvider = getProvider; }
  isAvailable() { return !!this.getProvider(); }
  private provider() {
    const p = this.getProvider();
    if (!p) throw new Error('UniSat Wallet not found. Open this page in a browser with UniSat installed.');
    return p;
  }
  async connect(): Promise<WalletConnection> {
    const p = this.provider();
    try {
      const accounts = await p.requestAccounts();
      if (!accounts[0]) throw new Error('No account');
      return { address: await this.getAddress(), publicKey: await this.getPublicKey(), network: await this.getNetwork() };
    } catch (e) { throw walletError(e); }
  }
  async getAddress() {
    const accounts = await this.provider().getAccounts();
    if (!accounts[0]) throw new Error('Wallet locked or disconnected. Connect again.');
    return accounts[0];
  }
  async getPublicKey() { return (await this.provider().getPublicKey()) || null; }
  async getNetwork() {
    const p = this.provider();
    if (!p.getChain) throw new Error('Update UniSat to a version supporting Fractal getChain.');
    return (await p.getChain()).enum;
  }
  async ensureFractalNetwork() {
    if (await this.getNetwork() === FRACTAL_CHAIN) return;
    const p = this.provider();
    if (!p.switchChain) throw new Error('Select Fractal Bitcoin Mainnet manually in UniSat, then retry.');
    try { await p.switchChain(FRACTAL_CHAIN); } catch (e) { throw walletError(e); }
    if (await this.getNetwork() !== FRACTAL_CHAIN) throw new Error('Fractal Bitcoin Mainnet is required.');
  }
  async signMessage(message: string): Promise<WalletSignature> {
    try { return { signature: await this.provider().signMessage(message, 'bip322-simple'), scheme: 'bip322-simple' }; }
    catch (e) { throw walletError(e); }
  }
  async disconnect() { await this.provider().disconnect?.(); }
  onChange(listener: () => void) {
    const p = this.getProvider();
    const handler = () => listener();
    p?.on?.('accountsChanged', handler);
    p?.on?.('chainChanged', handler);
    return () => { p?.removeListener?.('accountsChanged', handler); p?.removeListener?.('chainChanged', handler); };
  }
}
