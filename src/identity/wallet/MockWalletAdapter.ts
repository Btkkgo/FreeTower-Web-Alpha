import type { WalletAdapter } from './types';
// Imported only by the explicitly requested development screen. Never accepted by real verify.
export class MockWalletAdapter implements WalletAdapter {
  readonly id = 'mock-dev';
  readonly name = 'DEV Mock Wallet';
  private connected = false;
  constructor(readonly persona: 'alice' | 'bob') {}
  isAvailable() { return true; }
  async connect() { this.connected = true; return { address: `dev:${this.persona}`, publicKey: null, network: 'FRACTAL_BITCOIN_MAINNET' }; }
  async getAddress() { if (!this.connected) throw new Error('DEV wallet disconnected'); return `dev:${this.persona}`; }
  async getPublicKey() { return null; }
  async getNetwork() { return 'FRACTAL_BITCOIN_MAINNET'; }
  async ensureFractalNetwork() {}
  async signMessage(): Promise<never> { throw new Error('DEV wallet cannot sign real wallet challenges.'); }
  async disconnect() { this.connected = false; }
  onChange() { return () => {}; }
}
