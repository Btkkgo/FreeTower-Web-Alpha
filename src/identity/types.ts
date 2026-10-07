import type { User } from '../game/types';
export interface IdentityConfig { network: string; signatureScheme: string; development: boolean; devEnabled: boolean; legacyEnabled: boolean }
export interface WalletAccount { id: string; address: string; network: string; provider: string }
export interface CharacterIdentity {
  id: string; walletAccountId: string; characterAssetId: string; btcName: string;
  createdAt: number; lastVerifiedAt: number;
  status: 'active' | 'ownership-invalid' | 'domain-invalid' | 'asset-invalid'; dev?: boolean;
}
export interface CharacterAsset { inscriptionId: string; assetId: string; owner: string; contentType: string; previewUrl?: string; status: string }
export interface BtcNameRecord { name: string; owner: string; provider: string; network?: string }
export interface IdentitySession { wallet: WalletAccount; identity: CharacterIdentity | null; user: User | null; dev: boolean }
export interface IdentityOptions { assets: CharacterAsset[]; names: BtcNameRecord[]; dev: boolean }
export interface Challenge { nonce: string; message: string; issuedAt: number; expiresAt: number; signatureScheme: string }
