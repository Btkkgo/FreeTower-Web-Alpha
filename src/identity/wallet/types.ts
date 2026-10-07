export interface WalletConnection { address: string; publicKey: string | null; network: string }
export interface WalletSignature { signature: string; scheme: 'bip322-simple' }
export interface WalletAdapter {
  id: string;
  name: string;
  isAvailable(): boolean;
  connect(): Promise<WalletConnection>;
  getAddress(): Promise<string>;
  getPublicKey(): Promise<string | null>;
  getNetwork(): Promise<string>;
  ensureFractalNetwork(): Promise<void>;
  signMessage(message: string): Promise<WalletSignature>;
  disconnect(): Promise<void>;
  onChange(listener: () => void): () => void;
}
