package main

import (
	"context"
	"net/http"
	"time"
)

func (a *App) identityOptions(w http.ResponseWriter, r *http.Request) {
	_, account, _, e := a.walletCurrent(r.Context(), requestWalletHash(r))
	if e != nil {
		identityError(w, 401, "unauthenticated", "Connect and verify your wallet")
		return
	}
	if !a.limiter.allow("options:"+account.ID, 30, time.Minute) {
		identityError(w, 429, "rate_limited", "Please retry shortly")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	assets, names := a.identityProviders(account)
	available, e := assets.GetOwnedCharacterAssets(ctx, account.Address)
	if e != nil {
		identityError(w, 503, "asset_verification_unavailable", "Character ownership verification unavailable")
		return
	}
	owned, e := names.ReverseLookup(ctx, account.Address)
	if e != nil {
		identityError(w, 503, "name_verification_unavailable", "Name verification unavailable")
		return
	}
	// The service boundary independently checks provider output before exposing choices.
	filtered := []CharacterAsset{}
	for _, v := range available {
		if v.Owner == account.Address && v.Status == "eligible" && v.AssetID != "" {
			filtered = append(filtered, v)
		}
	}
	valid := []BtcNameRecord{}
	for _, n := range owned {
		if !validBTCName(n.Name) || n.Owner != account.Address {
			continue
		}
		forward, e := names.Resolve(ctx, n.Name)
		if e != nil {
			identityError(w, 503, "name_verification_unavailable", "Name verification unavailable")
			return
		}
		if forward != nil && forward.Owner == account.Address && forward.Name == n.Name {
			valid = append(valid, n)
		}
	}
	writeJSON(w, 200, map[string]any{"assets": filtered, "names": valid, "dev": account.Dev})
}
func (a *App) identitySelect(w http.ResponseWriter, r *http.Request) {
	if !a.identityOrigin(w, r) {
		return
	}
	var body struct {
		AssetID string `json:"characterAssetId"`
		Name    string `json:"btcName"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if !validBTCName(body.Name) || len(body.AssetID) > 150 || body.AssetID == "" {
		identityError(w, 400, "invalid_selection", "Select an eligible character and owned .btc name")
		return
	}
	// Provider calls happen outside the lock. Commit must re-read the exact session;
	// logout/re-auth during an in-flight request cannot resurrect that old session.
	hash := requestWalletHash(r)
	session, account, _, e := a.walletCurrent(r.Context(), hash)
	if e != nil {
		identityError(w, 401, "unauthenticated", "Connect and verify your wallet")
		return
	}
	if !a.limiter.allow("select:"+account.ID, 15, time.Minute) {
		identityError(w, 429, "rate_limited", "Please retry shortly")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	assets, names := a.identityProviders(account)
	ok, e := assets.VerifyOwnership(ctx, account.Address, body.AssetID)
	if e != nil {
		identityError(w, 503, "asset_verification_unavailable", "Character ownership verification unavailable")
		return
	}
	if !ok {
		if a.invalidateSelection(ctx, account.ID, body.AssetID, "", "asset-invalid", hash) != nil {
			identityError(w, 503, "persistence_unavailable", "Ownership invalid; session revoked but status could not be saved")
			return
		}
		identityError(w, 403, "asset_invalid", "Character is not eligible or is no longer owned by this wallet")
		return
	}
	owned, e := names.ReverseLookup(ctx, account.Address)
	if e != nil {
		identityError(w, 503, "name_verification_unavailable", "Name verification unavailable")
		return
	}
	match := false
	for _, n := range owned {
		if n.Name == body.Name && n.Owner == account.Address {
			match = true
		}
	}
	forward, e := names.Resolve(ctx, body.Name)
	if e != nil {
		identityError(w, 503, "name_verification_unavailable", "Name verification unavailable")
		return
	}
	if !match || forward == nil || forward.Name != body.Name || forward.Owner != account.Address {
		if a.invalidateSelection(ctx, account.ID, "", body.Name, "domain-invalid", hash) != nil {
			identityError(w, 503, "persistence_unavailable", "Ownership invalid; session revoked but status could not be saved")
			return
		}
		identityError(w, 403, "domain_invalid", "Select a .btc name owned by this wallet")
		return
	}
	a.identity.mu.Lock()
	defer a.identity.mu.Unlock()
	var selected CharacterIdentity
	_, e = a.store.identityState(ctx, func(d *identityData) error {
		current, _, e := readWalletSession(*d, hash)
		if e != nil || current.WalletID != session.WalletID || current.BootID != a.identity.bootID || a.identity.sessionRevoked(hash) {
			return errNotFound
		}
		for _, i := range d.Identities {
			if i.WalletAccountID == account.ID && i.CharacterAssetID == body.AssetID {
				selected = i
				break
			}
		}
		if selected.ID == "" {
			selected = CharacterIdentity{ID: newID(), WalletAccountID: account.ID, CharacterAssetID: body.AssetID, CreatedAt: nowMS(), Dev: account.Dev}
		}
		selected.BtcName = body.Name
		selected.Provider = forward.Provider
		selected.LastVerifiedAt = nowMS()
		selected.Status = "active"
		d.Identities[selected.ID] = selected
		current.IdentityID = selected.ID
		d.Sessions[hash] = current
		return nil
	})
	if e != nil {
		if isIdentityMissing(e) {
			identityError(w, 401, "session_changed", "Wallet session changed; authenticate again")
		} else {
			identityError(w, 503, "persistence_unavailable", "Identity could not be saved; retry")
		}
		return
	}
	// Existing sockets may contain a previous character/name, so force fresh upgrade.
	a.closeWalletSockets(map[string]bool{hash: true})
	writeJSON(w, 200, map[string]any{"identity": selected, "user": identityUser(selected), "dev": account.Dev})
}
func (a *App) invalidateSelection(ctx context.Context, walletID, asset, name, status, sessionHash string) error {
	a.identity.mu.Lock()
	defer a.identity.mu.Unlock()
	revoked := map[string]bool{sessionHash: true}
	_, e := a.store.identityState(ctx, func(d *identityData) error {
		for id, i := range d.Identities {
			if i.WalletAccountID == walletID && ((asset != "" && i.CharacterAssetID == asset) || (name != "" && i.BtcName == name)) {
				i.Status = status
				d.Identities[id] = i
				for hash, s := range d.Sessions {
					if s.IdentityID == id {
						s.IdentityID = ""
						d.Sessions[hash] = s
						revoked[hash] = true
					}
				}
			}
		}
		return nil
	})
	a.identity.denySessions(revoked)
	a.closeWalletSockets(revoked)
	return e
}
