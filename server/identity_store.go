package main

import (
	"context"
	"encoding/json"
	"errors"
)

// Identity records are isolated from legacy users and rooms. The PostgreSQL
// singleton is transaction-locked across processes; local saves use copy/rollback.
// Challenges deliberately live only in the bounded per-process cache.
func (s *Store) identityState(ctx context.Context, mutate func(*identityData) error) (identityData, error) {
	if s.db != nil {
		tx, e := s.db.Begin(ctx)
		if e != nil {
			return identityData{}, e
		}
		defer tx.Rollback(ctx)
		var raw []byte
		query := `SELECT data FROM freetower_identity_state WHERE id=1`
		if mutate != nil {
			query += ` FOR UPDATE`
		}
		if e = tx.QueryRow(ctx, query).Scan(&raw); e != nil {
			return identityData{}, e
		}
		var d identityData
		if e = json.Unmarshal(raw, &d); e != nil {
			return d, e
		}
		if mutate != nil {
			if e = mutate(&d); e != nil {
				return d, e
			}
			raw, e = json.Marshal(d)
			if e != nil {
				return d, e
			}
			if _, e = tx.Exec(ctx, `UPDATE freetower_identity_state SET data=$1 WHERE id=1`, raw); e != nil {
				return d, e
			}
		}
		return d, tx.Commit(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d := emptyIdentityData()
	if s.data.Identity != nil {
		raw, e := json.Marshal(s.data.Identity)
		if e != nil {
			return d, e
		}
		if e = json.Unmarshal(raw, &d); e != nil {
			return d, e
		}
	}
	if mutate == nil {
		return d, nil
	}
	if e := mutate(&d); e != nil {
		return d, e
	}
	old := s.data.Identity
	s.data.Identity = &d
	if e := s.flush(); e != nil {
		s.data.Identity = old
		return identityData{}, e
	}
	return d, nil
}
func walletKey(network, address string) string { return network + ":" + address }
func walletByID(d identityData, id string) (WalletAccount, error) {
	for _, w := range d.Wallets {
		if w.ID == id {
			return w, nil
		}
	}
	return WalletAccount{}, errNotFound
}
func readWalletSession(d identityData, hash string) (walletSession, WalletAccount, error) {
	s, ok := d.Sessions[hash]
	if !ok || s.ExpiresAt <= nowMS() || s.Network != identityNetwork {
		return s, WalletAccount{}, errNotFound
	}
	w, e := walletByID(d, s.WalletID)
	return s, w, e
}
func (a *App) walletCurrent(ctx context.Context, hash string) (walletSession, WalletAccount, *CharacterIdentity, error) {
	if a.identity.sessionRevoked(hash) {
		return walletSession{}, WalletAccount{}, nil, errNotFound
	}
	d, e := a.store.identityState(ctx, nil)
	if e != nil {
		return walletSession{}, WalletAccount{}, nil, e
	}
	s, w, e := readWalletSession(d, hash)
	if e != nil {
		return s, w, nil, e
	}
	if a.identity.bootID == "" || s.BootID != a.identity.bootID {
		return s, w, nil, errNotFound
	}
	if w.Dev && !a.identity.config.DevEnabled {
		return s, w, nil, errNotFound
	}
	var i *CharacterIdentity
	if candidate, ok := d.Identities[s.IdentityID]; ok && candidate.Status == "active" && candidate.WalletAccountID == w.ID {
		i = &candidate
	}
	return s, w, i, nil
}
func isIdentityMissing(e error) bool { return errors.Is(e, errNotFound) || errors.Is(e, errIdentity) }

func (s *identityService) sessionRevoked(hash string) bool {
	s.revokedMu.RLock()
	defer s.revokedMu.RUnlock()
	return s.revoked[hash] > nowMS()
}
func (s *identityService) denySessions(hashes map[string]bool) {
	s.revokedMu.Lock()
	defer s.revokedMu.Unlock()
	if s.revoked == nil {
		s.revoked = map[string]int64{}
	}
	for k, v := range s.revoked {
		if v <= nowMS() {
			delete(s.revoked, k)
		}
	}
	for k := range hashes {
		if k != "" {
			s.revoked[k] = nowMS() + 24*3600*1000
		}
	}
}
