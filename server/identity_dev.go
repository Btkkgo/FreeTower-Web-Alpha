package main

import "context"

// Synthetic fixtures are selected solely by a persisted DEV wallet session.
// They are never installed as fallback providers for a real wallet.
type devIdentityProvider struct{}

func (devIdentityProvider) GetOwnedCharacterAssets(_ context.Context, address string) ([]CharacterAsset, error) {
	return []CharacterAsset{{InscriptionID: address + ":character-1", AssetID: address + ":character-1", Owner: address, ContentType: "application/x-freetower-dev", Status: "eligible"}, {InscriptionID: address + ":character-2", AssetID: address + ":character-2", Owner: address, ContentType: "application/x-freetower-dev", Status: "eligible"}}, nil
}
func (p devIdentityProvider) VerifyOwnership(ctx context.Context, address, id string) (bool, error) {
	assets, _ := p.GetOwnedCharacterAssets(ctx, address)
	for _, a := range assets {
		if a.AssetID == id {
			return true, nil
		}
	}
	return false, nil
}

type devNameResolver struct{}

func (devNameResolver) ReverseLookup(_ context.Context, address string) ([]BtcNameRecord, error) {
	persona := "alice"
	if address == "dev:bob" {
		persona = "bob"
	}
	return []BtcNameRecord{{Name: persona + ".btc", Owner: address, Provider: "dev", Network: "synthetic"}, {Name: persona + "-two.btc", Owner: address, Provider: "dev", Network: "synthetic"}}, nil
}
func (devNameResolver) Resolve(_ context.Context, name string) (*BtcNameRecord, error) {
	for _, a := range []string{"alice", "bob"} {
		for _, n := range []string{a + ".btc", a + "-two.btc"} {
			if name == n {
				return &BtcNameRecord{Name: name, Owner: "dev:" + a, Provider: "dev", Network: "synthetic"}, nil
			}
		}
	}
	return nil, nil
}
func (p devNameResolver) VerifyOwnership(ctx context.Context, name, address string) (bool, error) {
	r, e := p.Resolve(ctx, name)
	return r != nil && r.Owner == address, e
}
func (a *App) identityProviders(w WalletAccount) (CharacterAssetProvider, BtcNameResolver) {
	if w.Dev && a.identity.config.DevEnabled {
		return devIdentityProvider{}, devNameResolver{}
	}
	return a.identity.assets, a.identity.names
}
