package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"
)

const walletCookie = "freetower_wallet_session"
const bindingCookie = "freetower_wallet_binding"

func randomToken() (string, error) {
	b := make([]byte, 32)
	_, e := rand.Read(b)
	return hex.EncodeToString(b), e
}
func requestWalletHash(r *http.Request) string {
	c, e := r.Cookie(walletCookie)
	if e != nil || len(c.Value) != 64 {
		return ""
	}
	return tokenHash(c.Value)
}
func (a *App) identityCookie(w http.ResponseWriter, name, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", HttpOnly: true, Secure: a.runtime.Public() || !a.identity.config.Development, SameSite: http.SameSiteLaxMode, MaxAge: maxAge})
}
func identityError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": message, "code": code})
}
func (a *App) identityRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/identity/config", func(w http.ResponseWriter, r *http.Request) {
		c := a.identity.config
		writeJSON(w, 200, map[string]any{"network": identityNetwork, "signatureScheme": signatureScheme, "development": c.Development, "devEnabled": c.DevEnabled, "legacyEnabled": c.LegacyEnabled})
	})
	m.HandleFunc("POST /api/auth/wallet/challenge", a.walletChallenge)
	m.HandleFunc("POST /api/auth/wallet/verify", a.walletVerify)
	m.HandleFunc("POST /api/auth/wallet/logout", a.walletLogout)
	m.HandleFunc("POST /api/auth/wallet/dev", a.walletDev)
	m.HandleFunc("GET /api/identity/me", a.identityMe)
	m.HandleFunc("GET /api/identity/options", a.identityOptions)
	m.HandleFunc("POST /api/identity/select", a.identitySelect)
	m.HandleFunc("POST /api/acceptance/ordinal-ownership", a.ordinalOwnershipAcceptance)
}
func (a *App) identityOrigin(w http.ResponseWriter, r *http.Request) bool {
	if !a.identity.originAllowed(r) {
		identityError(w, 403, "origin_mismatch", "Wallet authentication requires the configured FreeTower origin")
		return false
	}
	return true
}
func (a *App) walletChallenge(w http.ResponseWriter, r *http.Request) {
	if !a.identityOrigin(w, r) {
		return
	}
	if !a.limiter.allow("wallet:"+remoteKey(r), 30, 5*time.Minute) {
		identityError(w, 429, "rate_limited", "Please wait before requesting another challenge")
		return
	}
	var body struct {
		Address   string `json:"address"`
		PublicKey string `json:"publicKey"`
		Network   string `json:"network"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	address, e := canonicalAddress(body.Address)
	if e != nil || body.Network != identityNetwork {
		identityError(w, 400, "wrong_network_or_address", "Use a supported Fractal mainnet wallet address")
		return
	}
	if _, _, _, e = bip322Transactions(address, ""); e != nil {
		identityError(w, 400, "unsupported_address", "Use a native SegWit or Taproot key-path address")
		return
	}
	nonce, e := randomToken()
	if e != nil {
		identityError(w, 503, "unavailable", "Challenge generation unavailable")
		return
	}
	binding := ""
	if c, e := r.Cookie(bindingCookie); e == nil && len(c.Value) == 64 {
		binding = c.Value
	}
	if binding == "" {
		binding, e = randomToken()
		if e != nil {
			identityError(w, 503, "unavailable", "Challenge generation unavailable")
			return
		}
	}
	now := time.Now().UTC()
	c := walletChallenge{Environment: a.identity.config.Environment, Nonce: nonce, Address: address, Origin: r.Header.Get("Origin"), Binding: tokenHash(binding), IssuedAt: now.UnixMilli(), ExpiresAt: now.Add(5 * time.Minute).UnixMilli(), SignatureScheme: signatureScheme}
	c.Message = fmt.Sprintf("FreeTower Sign-In\nOrigin: %s\nAddress: %s\nNetwork: %s\nNonce: %s\nIssued At: %s\nExpiration Time: %s\n\nSign only to prove wallet ownership. No transaction or payment is requested.", c.Origin, address, identityNetwork, nonce, now.Format(time.RFC3339), now.Add(5*time.Minute).Format(time.RFC3339))
	a.identity.mu.Lock()
	defer a.identity.mu.Unlock()
	for k, v := range a.identity.challenges {
		if v.ExpiresAt <= nowMS() {
			delete(a.identity.challenges, k)
		}
	}
	if len(a.identity.challenges) >= 2000 {
		identityError(w, 429, "challenge_capacity", "Please retry shortly")
		return
	}
	a.identity.challenges[nonce] = c
	a.identityCookie(w, bindingCookie, binding, 600)
	writeJSON(w, 200, c)
}
func (a *App) walletVerify(w http.ResponseWriter, r *http.Request) {
	if !a.identityOrigin(w, r) {
		return
	}
	if !a.limiter.allow("verify:"+remoteKey(r), 60, 5*time.Minute) {
		identityError(w, 429, "rate_limited", "Please retry shortly")
		return
	}
	var body struct {
		Address   string `json:"address"`
		PublicKey string `json:"publicKey"`
		Network   string `json:"network"`
		Nonce     string `json:"nonce"`
		Signature string `json:"signature"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	address, e := canonicalAddress(body.Address)
	if e != nil || body.Network != identityNetwork {
		identityError(w, 400, "wrong_network_or_address", "Use the challenged Fractal mainnet address")
		return
	}
	a.identity.mu.Lock()
	locked := true
	defer func() {
		if locked {
			a.identity.mu.Unlock()
		}
	}()
	c, ok := a.identity.challenges[body.Nonce]
	cookie, ce := r.Cookie(bindingCookie)
	if !ok || !a.identity.config.environmentMatches(c.Environment) || c.ExpiresAt <= nowMS() || c.Address != address || c.Origin != r.Header.Get("Origin") || ce != nil || tokenHash(cookie.Value) != c.Binding {
		identityError(w, 401, "invalid_challenge", "Challenge expired, consumed, or does not match this browser; request a new one")
		return
	}
	if e = a.identity.verify(address, c.Message, body.Signature); e != nil {
		identityError(w, 401, "invalid_signature", errSignature.Error())
		return
	}
	// Burn before persistence: a failed save never leaves a reusable signed proof.
	delete(a.identity.challenges, body.Nonce)
	login, e := a.persistWalletSession(w, r, address, false)
	a.identity.mu.Unlock()
	locked = false
	if e != nil {
		identityError(w, 503, "persistence_unavailable", "Session could not be saved; request a new challenge")
		return
	}
	a.completeWalletLogin(w, r, login)
}

type walletLogin struct {
	Token    string
	Account  WalletAccount
	Snapshot identityData
}

// Caller holds identity.mu only for proof consumption, persistence and revocation.
func (a *App) persistWalletSession(w http.ResponseWriter, r *http.Request, address string, dev bool) (walletLogin, error) {
	token, e := randomToken()
	if e != nil {
		return walletLogin{}, e
	}
	binding := ""
	if c, e := r.Cookie(bindingCookie); e == nil && len(c.Value) == 64 {
		binding = c.Value
	}
	if binding == "" {
		binding, e = randomToken()
		if e != nil {
			return walletLogin{}, e
		}
		a.identityCookie(w, bindingCookie, binding, 600)
	}
	binding = tokenHash(binding)
	now := nowMS()
	hash := tokenHash(token)
	var account WalletAccount
	revoked := map[string]bool{}
	d, e := a.store.identityState(r.Context(), func(d *identityData) error {
		key := walletKey(identityNetwork, address)
		account = d.Wallets[key]
		if account.ID == "" {
			provider := "unisat"
			if dev {
				provider = "dev"
			}
			account = WalletAccount{ID: newID(), Address: address, Network: identityNetwork, Provider: provider, CreatedAt: now, Dev: dev}
		}
		account.LastAuthenticatedAt = now
		d.Wallets[key] = account
		prior := requestWalletHash(r)
		for k, s := range d.Sessions {
			if s.WalletID == account.ID || k == prior || s.Binding == binding || s.ExpiresAt <= now {
				revoked[k] = true
				delete(d.Sessions, k)
			}
		}
		d.Sessions[hash] = walletSession{Environment: a.identity.config.Environment, BootID: a.identity.bootID, TokenHash: hash, WalletID: account.ID, Binding: binding, Network: identityNetwork, ExpiresAt: now + 24*3600*1000}
		return nil
	})
	if e != nil {
		return walletLogin{}, e
	}
	a.identity.denySessions(revoked)
	a.closeWalletSockets(revoked)
	for k, c := range a.identity.challenges {
		if c.Binding == binding {
			delete(a.identity.challenges, k)
		}
	}
	return walletLogin{token, account, d}, nil
}

// External provider I/O never holds the authentication or room mutex.
func (a *App) completeWalletLogin(w http.ResponseWriter, r *http.Request, login walletLogin) {
	if e := a.revalidatePrevious(r.Context(), login.Account, login.Snapshot); e != nil {
		identityError(w, 503, "persistence_unavailable", "Ownership status could not be saved; request a new challenge")
		return
	}
	if _, _, _, e := a.walletCurrent(r.Context(), tokenHash(login.Token)); e != nil {
		identityError(w, 401, "session_changed", "Wallet session changed; authenticate again")
		return
	}
	a.identityCookie(w, walletCookie, login.Token, 24*3600)
	writeJSON(w, 200, map[string]any{"wallet": login.Account, "identity": nil, "user": nil, "dev": login.Account.Dev})
}
func (a *App) walletDev(w http.ResponseWriter, r *http.Request) {
	if !a.identity.devAllowed(r) {
		identityError(w, 403, "dev_disabled", "DEV identity is disabled")
		return
	}
	var b struct {
		Persona string `json:"persona"`
	}
	if !decodeJSON(w, r, &b) {
		return
	}
	if b.Persona != "alice" && b.Persona != "bob" {
		identityError(w, 400, "invalid_persona", "Choose a supported DEV persona")
		return
	}
	a.identity.mu.Lock()
	login, e := a.persistWalletSession(w, r, "dev:"+b.Persona, true)
	a.identity.mu.Unlock()
	if e != nil {
		identityError(w, 503, "persistence_unavailable", "DEV session could not be saved")
		return
	}
	a.completeWalletLogin(w, r, login)
}
func (a *App) walletLogout(w http.ResponseWriter, r *http.Request) {
	if !a.identityOrigin(w, r) {
		return
	}
	a.identity.mu.Lock()
	defer a.identity.mu.Unlock()
	hash := requestWalletHash(r)
	binding := ""
	if c, e := r.Cookie(bindingCookie); e == nil {
		binding = tokenHash(c.Value)
	}
	revoked := map[string]bool{hash: true}
	_, e := a.store.identityState(r.Context(), func(d *identityData) error {
		for k, s := range d.Sessions {
			if k == hash || (binding != "" && s.Binding == binding) {
				revoked[k] = true
				delete(d.Sessions, k)
			}
		}
		return nil
	})
	a.identity.denySessions(revoked)
	a.closeWalletSockets(revoked)
	if e != nil {
		identityError(w, 503, "persistence_unavailable", "Logout could not be saved; retry")
		return
	}
	a.closeWalletSockets(map[string]bool{hash: true})
	if c, e := r.Cookie(bindingCookie); e == nil {
		for k, v := range a.identity.challenges {
			if v.Binding == tokenHash(c.Value) {
				delete(a.identity.challenges, k)
			}
		}
	}
	a.identityCookie(w, walletCookie, "", -1)
	a.identityCookie(w, bindingCookie, "", -1)
	writeJSON(w, 200, map[string]bool{"ok": true})
}
func (a *App) identityMe(w http.ResponseWriter, r *http.Request) {
	_, account, i, e := a.walletCurrent(r.Context(), requestWalletHash(r))
	if e != nil {
		identityError(w, 401, "unauthenticated", "Connect and verify your wallet")
		return
	}
	var user *User
	if i != nil {
		u := identityUser(*i)
		user = &u
	}
	writeJSON(w, 200, map[string]any{"wallet": account, "identity": i, "user": user, "dev": account.Dev})
}
func (a *App) closeWalletSockets(hashes map[string]bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, rr := range a.rooms {
		rr.mu.Lock()
		for c := range rr.clients {
			if c.sessionHash != "" && hashes[c.sessionHash] {
				c.revokeIdentity()
			}
		}
		rr.mu.Unlock()
	}
}
func (a *App) revalidatePrevious(ctx context.Context, w WalletAccount, d identityData) error {
	assets, names := a.identityProviders(w)
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	for id, i := range d.Identities {
		if i.WalletAccountID != w.ID || i.Status != "active" || !a.identity.config.identityAllowed(i) {
			continue
		}
		status := ""
		ok, e := assets.VerifyOwnership(ctx, w.Address, i.CharacterAssetID)
		if e != nil {
			continue
		}
		if !ok {
			status = "asset-invalid"
		} else {
			ok, e = names.VerifyOwnership(ctx, i.BtcName, w.Address)
			if e != nil {
				continue
			}
			if !ok {
				status = "domain-invalid"
			}
		}
		if status != "" {
			_, saveError := a.store.identityState(ctx, func(current *identityData) error {
				v := current.Identities[id]
				if v.LastVerifiedAt != i.LastVerifiedAt {
					return nil
				}
				v.Status = status
				current.Identities[id] = v
				return nil
			})
			if saveError != nil {
				return saveError
			}
		}
	}

	return nil
}
