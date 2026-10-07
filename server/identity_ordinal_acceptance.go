package main

import (
	"context"
	"net/http"
	"time"
)

// Ownership evidence is not a CharacterAsset and cannot be passed to selection
// as authority. This helper deliberately does not consult or modify p.allow.
type ordinalOwnershipEvidence struct {
	InscriptionID               string `json:"inscriptionId"`
	Network                     string `json:"network"`
	Exists                      bool   `json:"exists"`
	OwnerResolved               bool   `json:"ownerResolved"`
	OwnerMasked                 string `json:"ownerMasked,omitempty"`
	Confirmed                   bool   `json:"confirmed"`
	ConnectedWalletMatchesOwner bool   `json:"connectedWalletMatchesOwner"`
	OwnershipVerified           bool   `json:"ownershipVerified"`
	GrantsCharacterEligibility  bool   `json:"grantsCharacterEligibility"`
}

func (p *UniSatFractalOrdinalsProvider) inspectOrdinalOwnership(ctx context.Context, address, id string) (ordinalOwnershipEvidence, error) {
	result := ordinalOwnershipEvidence{InscriptionID: id, Network: identityNetwork}
	if !inscriptionIDPattern.MatchString(id) {
		return result, errProvider
	}
	var out struct {
		Code *int              `json:"code"`
		Data inscriptionRecord `json:"data"`
	}
	status, e := p.get(ctx, "/v1/indexer/inscription/info/"+id, &out)
	if status == http.StatusNotFound {
		return result, nil
	}
	if e != nil || out.Code == nil || *out.Code != 0 || out.Data.ID != id {
		return result, errProvider
	}
	owner, e := canonicalAddress(out.Data.Address)
	if e != nil {
		return result, errProvider
	}
	result.Exists = true
	result.OwnerResolved = true
	result.OwnerMasked = owner[:6] + "…" + owner[len(owner)-6:]
	result.Confirmed = out.Data.Height > 0
	result.ConnectedWalletMatchesOwner = owner == address
	result.OwnershipVerified = result.Confirmed && result.ConnectedWalletMatchesOwner
	return result, nil
}
func (a *App) ordinalOwnershipAcceptance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	c := a.identity.config
	if !c.Development || !c.OwnershipAcceptanceEnabled || !loopbackHost(remoteKey(r)) {
		identityError(w, 403, "acceptance_disabled", "Local ownership acceptance is disabled")
		return
	}
	if !a.identityOrigin(w, r) {
		return
	}
	hash := requestWalletHash(r)
	session, account, _, e := a.walletCurrent(r.Context(), hash)
	if e != nil {
		identityError(w, 401, "unauthenticated", "A real wallet session is required")
		return
	}
	if account.Dev || account.Network != identityNetwork {
		identityError(w, 403, "real_wallet_required", "Use a real Fractal wallet session")
		return
	}
	if !a.limiter.allow("ordinal-acceptance:"+account.ID, 10, time.Minute) {
		identityError(w, 429, "rate_limited", "Please retry shortly")
		return
	}
	var body struct {
		InscriptionID string `json:"inscriptionId"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if !inscriptionIDPattern.MatchString(body.InscriptionID) {
		identityError(w, 400, "invalid_inscription", "Use an exact inscription ID")
		return
	}
	// Acceptance never selects a DEV provider, even when development fixtures are enabled.
	provider, ok := a.identity.assets.(*UniSatFractalOrdinalsProvider)
	if !ok {
		identityError(w, 503, "ownership_unavailable", "Real ownership provider unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	result, e := provider.inspectOrdinalOwnership(ctx, account.Address, body.InscriptionID)
	if e != nil {
		identityError(w, 503, "ownership_unavailable", "Real ownership verification unavailable")
		return
	}
	// Logout or session replacement during the request must not yield authenticated evidence.
	current, wallet, _, e := a.walletCurrent(ctx, hash)
	if e != nil || current.WalletID != session.WalletID || wallet.Address != account.Address {
		identityError(w, 401, "session_changed", "Authenticate again")
		return
	}
	if !result.Exists {
		identityError(w, 404, "inscription_not_found", "Inscription not found in the Fractal mainnet query environment")
		return
	}
	writeJSON(w, 200, result)
}
