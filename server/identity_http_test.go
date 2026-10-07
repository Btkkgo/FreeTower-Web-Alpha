package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/gorilla/websocket"
)

type identityHarness struct {
	t       *testing.T
	app     *App
	server  *httptest.Server
	client  *http.Client
	key     *btcec.PrivateKey
	address string
	path    string
}

func newIdentityHarness(t *testing.T) *identityHarness {
	t.Helper()
	t.Setenv("FT_ENV", "development")
	t.Setenv("FT_ENABLE_DEV_IDENTITY", "true")
	t.Setenv("FT_ENABLE_LEGACY_AUTH", "")
	path := filepath.Join(t.TempDir(), "identity.json")
	store, e := openStore("", path)
	if e != nil {
		t.Fatal(e)
	}
	a, e := newApp(store, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	server := httptest.NewServer(a.routes())
	a.identity.config.Origin = server.URL
	a.identity.config.Origins = map[string]bool{server.URL: true}
	jar, _ := cookiejar.New(nil)
	key, _ := btcec.NewPrivateKey()
	addr, _ := btcutil.NewAddressWitnessPubKeyHash(btcutil.Hash160(key.PubKey().SerializeCompressed()), &chaincfg.MainNetParams)
	h := &identityHarness{t, a, server, &http.Client{Jar: jar, Timeout: 10 * time.Second}, key, addr.EncodeAddress(), path}
	t.Cleanup(func() {
		server.Close()
		select {
		case <-a.stop:
			<-a.tickerDone
		default:
			a.shutdown()
		}
		store.close()
	})
	return h
}
func (h *identityHarness) request(c *http.Client, method, path string, body any, origin string) (int, []byte) {
	h.t.Helper()
	b, _ := json.Marshal(body)
	r, _ := http.NewRequest(method, h.server.URL+path, bytes.NewReader(b))
	r.Header.Set("Origin", origin)
	r.Header.Set("Content-Type", "application/json")
	res, e := c.Do(r)
	if e != nil {
		h.t.Fatal(e)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, raw
}
func (h *identityHarness) call(method, path string, body any, status int, out any) {
	h.t.Helper()
	got, raw := h.request(h.client, method, path, body, h.server.URL)
	if got != status {
		h.t.Fatalf("%s: got %d want %d: %s", path, got, status, raw)
	}
	if out != nil {
		if e := json.Unmarshal(raw, out); e != nil {
			h.t.Fatal(e)
		}
	}
}
func (h *identityHarness) challenge() walletChallenge {
	h.t.Helper()
	var c walletChallenge
	h.call("POST", "/api/auth/wallet/challenge", map[string]string{"address": h.address, "network": identityNetwork}, 200, &c)
	return c
}
func (h *identityHarness) proof(c walletChallenge) map[string]string {
	h.t.Helper()
	_, tx, script, e := bip322Transactions(h.address, c.Message)
	if e != nil {
		h.t.Fatal(e)
	}
	scriptCode, _ := txscript.NewScriptBuilder().AddOp(txscript.OP_DUP).AddOp(txscript.OP_HASH160).AddData(btcutil.Hash160(h.key.PubKey().SerializeCompressed())).AddOp(txscript.OP_EQUALVERIFY).AddOp(txscript.OP_CHECKSIG).Script()
	fetch := txscript.NewCannedPrevOutputFetcher(script, 0)
	witness, e := txscript.WitnessSignature(tx, txscript.NewTxSigHashes(tx, fetch), 0, 0, scriptCode, txscript.SigHashAll, h.key, true)
	if e != nil {
		h.t.Fatal(e)
	}
	var b bytes.Buffer
	wire.WriteVarInt(&b, 0, uint64(len(witness)))
	for _, item := range witness {
		wire.WriteVarBytes(&b, 0, item)
	}
	return map[string]string{"address": h.address, "network": identityNetwork, "nonce": c.Nonce, "signature": base64.StdEncoding.EncodeToString(b.Bytes())}
}
func (h *identityHarness) login() {
	h.t.Helper()
	c := h.challenge()
	h.call("POST", "/api/auth/wallet/verify", h.proof(c), 200, nil)
}
func (h *identityHarness) fixtureProviders() *identityFixture {
	f := &identityFixture{address: h.address, assetOwned: true, nameOwned: true, assets: 2, names: 2}
	h.app.identity.assets = f
	h.app.identity.names = &fixtureNames{f}
	return f
}
func (h *identityHarness) selectIdentity(status int) CharacterIdentity {
	h.t.Helper()
	var v struct{ Identity CharacterIdentity }
	h.call("POST", "/api/identity/select", map[string]string{"characterAssetId": "asset-1", "btcName": "alice.btc"}, status, &v)
	return v.Identity
}

type identityFixture struct {
	address         string
	assetOwned      bool
	nameOwned       bool
	assets          int
	names           int
	outage          bool
	wrongOwner      bool
	reverseMismatch bool
	wait            chan struct{}
	started         chan struct{}
}

func (f *identityFixture) GetOwnedCharacterAssets(context.Context, string) ([]CharacterAsset, error) {
	if f.outage {
		return nil, errProvider
	}
	out := []CharacterAsset{}
	for _, id := range []string{"asset-1", "asset-2"}[:f.assets] {
		owner := f.address
		if f.wrongOwner {
			owner = "other"
		}
		out = append(out, CharacterAsset{AssetID: id, InscriptionID: id, Owner: owner, Status: "eligible"})
	}
	return out, nil
}
func (f *identityFixture) VerifyOwnership(context.Context, string, string) (bool, error) {
	if f.wait != nil {
		close(f.started)
		<-f.wait
	}
	if f.outage {
		return false, errProvider
	}
	return f.assetOwned && !f.wrongOwner, nil
}

type fixtureNames struct{ f *identityFixture }

func (p *fixtureNames) ReverseLookup(context.Context, string) ([]BtcNameRecord, error) {
	f := p.f
	if f.outage {
		return nil, errProvider
	}
	result := []BtcNameRecord{}
	for _, name := range []string{"alice.btc", "second.btc"}[:f.names] {
		if f.reverseMismatch {
			name = "mismatch.btc"
		}
		owner := f.address
		if f.wrongOwner {
			owner = "other"
		}
		result = append(result, BtcNameRecord{Name: name, Owner: owner, Provider: "fixture", Network: "bitcoin-mainnet"})
	}
	return result, nil
}
func (p *fixtureNames) Resolve(_ context.Context, name string) (*BtcNameRecord, error) {
	f := p.f
	if f.outage {
		return nil, errProvider
	}
	owner := f.address
	if !f.nameOwned || f.wrongOwner {
		owner = "other"
	}
	return &BtcNameRecord{Name: name, Owner: owner, Provider: "fixture", Network: "bitcoin-mainnet"}, nil
}
func (p *fixtureNames) VerifyOwnership(ctx context.Context, name, address string) (bool, error) {
	r, e := p.Resolve(ctx, name)
	return r != nil && r.Owner == address, e
}
func TestWalletChallengeAndReplay(t *testing.T) {
	h := newIdentityHarness(t)
	c := h.challenge()
	other := h.challenge()
	if c.Nonce == other.Nonce || len(c.Nonce) != 64 || c.ExpiresAt-c.IssuedAt != 300000 || !strings.Contains(c.Message, h.server.URL) {
		t.Fatal("challenge entropy/TTL/origin")
	}
	proof := h.proof(c)
	h.call("POST", "/api/auth/wallet/verify", proof, 200, nil)
	h.call("POST", "/api/auth/wallet/verify", proof, 401, nil)
	h.call("GET", "/api/rooms", nil, 401, nil)
	var me struct {
		Wallet   WalletAccount
		Identity *CharacterIdentity
		User     *User
	}
	h.call("GET", "/api/identity/me", nil, 200, &me)
	if me.Identity != nil || me.User != nil || me.Wallet.Provider != "unisat" || me.Wallet.Network != identityNetwork {
		t.Fatal("unverified identity entered world")
	}
	id := me.Wallet.ID
	h.login()
	h.call("GET", "/api/identity/me", nil, 200, &me)
	if me.Wallet.ID != id {
		t.Fatal("duplicate wallet account")
	}
}
func TestWalletChallengeFailures(t *testing.T) {
	for _, scenario := range []string{"expiry", "address", "network", "signature", "origin", "host", "binding", "missing-origin"} {
		t.Run(scenario, func(t *testing.T) {
			h := newIdentityHarness(t)
			c := h.challenge()
			proof := h.proof(c)
			origin := h.server.URL
			status := 401
			switch scenario {
			case "expiry":
				h.app.identity.mu.Lock()
				v := h.app.identity.challenges[c.Nonce]
				v.ExpiresAt = nowMS() - 1
				h.app.identity.challenges[c.Nonce] = v
				h.app.identity.mu.Unlock()
			case "address":
				proof["address"] = "bc1q9vza2e8x573nczrlzms0wvx3gsqjx7vavgkx0l"
			case "network":
				proof["network"] = "bitcoin-mainnet"
				status = 400
			case "signature":
				proof["signature"] = "invalid"
			case "origin", "host":
				origin = "https://attacker.invalid"
				status = 403
			case "missing-origin":
				origin = ""
				status = 403
			case "binding":
				h.client.Jar, _ = cookiejar.New(nil)
			}
			got, _ := h.request(h.client, "POST", "/api/auth/wallet/verify", proof, origin)
			if got != status {
				t.Fatalf("got %d want %d", got, status)
			}
		})
	}
}
func TestWalletConcurrentProofConsumption(t *testing.T) {
	h := newIdentityHarness(t)
	proof := h.proof(h.challenge())
	b, _ := json.Marshal(proof)
	codes := make(chan int, 8)
	var wg sync.WaitGroup
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, _ := http.NewRequest("POST", h.server.URL+"/api/auth/wallet/verify", bytes.NewReader(b))
			r.Header.Set("Origin", h.server.URL)
			res, e := h.client.Do(r)
			if e != nil {
				codes <- 0
				return
			}
			io.Copy(io.Discard, res.Body)
			res.Body.Close()
			codes <- res.StatusCode
		}()
	}
	wg.Wait()
	close(codes)
	success := 0
	for code := range codes {
		if code == 200 {
			success++
		} else if code != 401 {
			t.Fatalf("unexpected %d", code)
		}
	}
	if success != 1 {
		t.Fatalf("successes %d", success)
	}
}
func TestWalletSaveFailureBurnsProof(t *testing.T) {
	h := newIdentityHarness(t)
	proof := h.proof(h.challenge())
	old := h.app.store.path
	h.app.store.path = filepath.Join(h.t.TempDir(), "block", "file")
	os.WriteFile(filepath.Dir(h.app.store.path), []byte("block"), 0600)
	h.call("POST", "/api/auth/wallet/verify", proof, 503, nil)
	h.app.store.path = old
	h.call("POST", "/api/auth/wallet/verify", proof, 401, nil)
	d, e := h.app.store.identityState(context.Background(), nil)
	if e != nil || len(d.Wallets) != 0 || len(d.Sessions) != 0 {
		t.Fatal("failed save left account/session")
	}
}
func TestWalletProductionGate(t *testing.T) {
	h := newIdentityHarness(t)
	h.app.identity.config = identityConfig{Origins: map[string]bool{}}
	for _, path := range []string{"/api/auth/register", "/api/auth/login", "/api/auth/wallet/dev", "/api/auth/wallet/challenge"} {
		h.call("POST", path, map[string]string{}, 403, nil)
	}
	h.call("GET", "/api/rooms?dev=1", nil, 401, nil)
	h.call("GET", "/api/rooms/room/ws?dev=1", nil, 401, nil)
	h.app.identity.config.Development = false
	w := httptest.NewRecorder()
	h.app.identityCookie(w, walletCookie, "token", 3600)
	c := w.Result().Cookies()[0]
	if !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
		t.Fatal("insecure production cookie")
	}
}
func TestIdentityOptionsSelectionAndPrivacy(t *testing.T) {
	h := newIdentityHarness(t)
	h.fixtureProviders()
	h.login()
	var options struct {
		Assets []CharacterAsset
		Names  []BtcNameRecord
	}
	h.call("GET", "/api/identity/options", nil, 200, &options)
	if len(options.Assets) != 2 || len(options.Names) != 2 {
		t.Fatal("multiple choices lost")
	}
	id := h.selectIdentity(200)
	if id.ID == "" || id.BtcName != "alice.btc" || id.LastVerifiedAt == 0 {
		t.Fatal("identity not created")
	}
	h.call("GET", "/api/rooms", nil, 200, nil)
	h.call("POST", "/api/identity/select", map[string]string{"characterAssetId": "asset-1", "btcName": "alice.btc", "username": "custom"}, 400, nil)
	var me struct{ User User }
	h.call("GET", "/api/identity/me", nil, 200, &me)
	if me.User.ID != id.ID || me.User.Username != "alice.btc" || me.User.CharacterAssetID != "asset-1" {
		t.Fatal("display identity mismatch")
	}
	public, _ := json.Marshal(Player{ID: me.User.ID, Username: me.User.Username, CharacterIdentityID: id.ID, BtcName: id.BtcName, CharacterAssetID: id.CharacterAssetID})
	if bytes.Contains(public, []byte(h.address)) || bytes.Contains(public, []byte(id.WalletAccountID)) {
		t.Fatal("public wallet leak")
	}
	h.login()
	h.call("GET", "/api/rooms", nil, 401, nil)
	again := h.selectIdentity(200)
	if again.ID != id.ID {
		t.Fatal("character inventory key changed")
	}
	reopened, e := openStore("", h.path)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.close()
	d, e := reopened.identityState(context.Background(), nil)
	if e != nil || d.Identities[id.ID].BtcName != "alice.btc" || len(d.Wallets) != 1 {
		t.Fatal("identity persistence failed")
	}
}
func TestIdentityOwnershipDenials(t *testing.T) {
	for _, scenario := range []string{"asset", "name", "wrong-owner", "reverse-mismatch", "outage", "no-assets", "no-names", "suffix"} {
		t.Run(scenario, func(t *testing.T) {
			h := newIdentityHarness(t)
			f := h.fixtureProviders()
			h.login()
			status := 403
			switch scenario {
			case "asset":
				f.assetOwned = false
			case "name":
				f.nameOwned = false
			case "wrong-owner":
				f.wrongOwner = true
			case "reverse-mismatch":
				f.reverseMismatch = true
			case "outage":
				f.outage = true
				status = 503
			case "no-assets":
				f.assets = 0
				f.assetOwned = false
			case "no-names":
				f.names = 0
			case "suffix":
				h.call("POST", "/api/identity/select", map[string]string{"characterAssetId": "asset-1", "btcName": "alice.eth"}, 400, nil)
				return
			}
			h.selectIdentity(status)
			h.call("GET", "/api/rooms", nil, 401, nil)
		})
	}
}
func TestIdentityTransferRevalidation(t *testing.T) {
	for _, kind := range []string{"asset", "domain"} {
		t.Run(kind, func(t *testing.T) {
			h := newIdentityHarness(t)
			f := h.fixtureProviders()
			h.login()
			i := h.selectIdentity(200)
			if kind == "asset" {
				f.assetOwned = false
			} else {
				f.nameOwned = false
			}
			h.login()
			h.call("GET", "/api/rooms", nil, 401, nil)
			d, _ := h.app.store.identityState(context.Background(), nil)
			if d.Identities[i.ID].Status != kind+"-invalid" {
				t.Fatalf("transfer status %s", d.Identities[i.ID].Status)
			}
		})
	}
}
func TestIdentityLogoutDuringSelection(t *testing.T) {
	h := newIdentityHarness(t)
	f := h.fixtureProviders()
	h.login()
	f.wait = make(chan struct{})
	f.started = make(chan struct{})
	finished := make(chan int, 1)
	go func() {
		status, _ := h.request(h.client, "POST", "/api/identity/select", map[string]string{"characterAssetId": "asset-1", "btcName": "alice.btc"}, h.server.URL)
		finished <- status
	}()
	<-f.started
	h.call("POST", "/api/auth/wallet/logout", map[string]string{}, 200, nil)
	close(f.wait)
	if <-finished != 401 {
		t.Fatal("stale selection revived session")
	}
	h.call("GET", "/api/identity/me", nil, 401, nil)
}
func TestIdentitySocketLogoutAndReauth(t *testing.T) {
	for _, scenario := range []string{"logout", "reauth", "expiry"} {
		t.Run(scenario, func(t *testing.T) {
			h := newIdentityHarness(t)
			h.fixtureProviders()
			h.login()
			h.selectIdentity(200)
			var room detail
			h.call("POST", "/api/rooms", map[string]string{"name": "identity room"}, 201, &room)
			if scenario == "expiry" {
				h.app.store.identityState(context.Background(), func(d *identityData) error {
					for k, s := range d.Sessions {
						s.ExpiresAt = nowMS() + 400
						d.Sessions[k] = s
					}
					return nil
				})
			}
			u, _ := url.Parse(h.server.URL)
			headers := http.Header{"Origin": []string{h.server.URL}}
			for _, c := range h.client.Jar.Cookies(u) {
				headers.Add("Cookie", c.String())
			}
			ws, res, e := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(h.server.URL, "http")+"/api/rooms/"+room.Room.ID+"/ws", headers)
			if e != nil {
				t.Fatalf("upgrade: %v %v", e, res)
			}
			defer ws.Close()
			if scenario == "logout" {
				h.call("POST", "/api/auth/wallet/logout", map[string]string{}, 200, nil)
			} else if scenario == "reauth" {
				h.login()
			}
			ws.SetReadDeadline(time.Now().Add(2 * time.Second))
			for {
				_, _, e = ws.ReadMessage()
				if e != nil {
					break
				}
			}
			if e == nil {
				t.Fatal("socket remained alive")
			}
			if ne, ok := e.(interface{ Timeout() bool }); ok && ne.Timeout() {
				t.Fatal("revocation did not close socket")
			}
		})
	}
}

func TestIdentityLegacyAndDevSessionsCannotBypassProduction(t *testing.T) {
	h := newIdentityHarness(t)
	h.call("POST", "/api/auth/wallet/dev", map[string]string{"persona": "alice"}, 200, nil)
	var choices struct {
		Assets []CharacterAsset
		Names  []BtcNameRecord
	}
	h.call("GET", "/api/identity/options", nil, 200, &choices)
	if len(choices.Assets) != 2 || choices.Names[0].Name != "alice.btc" {
		t.Fatal("DEV fixture choices")
	}
	h.call("POST", "/api/identity/select", map[string]string{"characterAssetId": choices.Assets[0].AssetID, "btcName": choices.Names[0].Name}, 200, nil)
	h.call("GET", "/api/rooms", nil, 200, nil)
	h.app.identity.config.DevEnabled = false
	h.app.identity.config.Development = false
	h.call("GET", "/api/rooms", nil, 401, nil)
	account := Account{ID: newID(), Username: "Legacy", PasswordHash: "fixture"}
	h.app.store.createAccount(context.Background(), account)
	token, _ := randomToken()
	h.app.store.putSession(context.Background(), Session{TokenHash: tokenHash(token), UserID: account.ID, ExpiresAt: nowMS() + 100000})
	h.client.Jar, _ = cookiejar.New(nil)
	u, _ := url.Parse(h.server.URL)
	h.client.Jar.SetCookies(u, []*http.Cookie{{Name: "flock_session", Value: token, Path: "/"}})
	h.call("GET", "/api/rooms", nil, 401, nil)
}

func TestIdentityMultiplayerChatCarriesNameWithoutWallet(t *testing.T) {
	h := newIdentityHarness(t)
	h.fixtureProviders()
	h.login()
	identity := h.selectIdentity(200)
	var room detail
	h.call("POST", "/api/rooms", map[string]string{"name": "identity-chat"}, 201, &room)
	u, _ := url.Parse(h.server.URL)
	headers := http.Header{"Origin": []string{h.server.URL}}
	for _, c := range h.client.Jar.Cookies(u) {
		headers.Add("Cookie", c.String())
	}
	ws, _, e := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(h.server.URL, "http")+"/api/rooms/"+room.Room.ID+"/ws?world=freetower", headers)
	if e != nil {
		t.Fatal(e)
	}
	defer ws.Close()
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	ws.WriteJSON(map[string]string{"type": "chat", "text": "hello identity", "username": "attacker"})
	presence, chat := false, false
	for !presence || !chat {
		_, raw, e := ws.ReadMessage()
		if e != nil {
			t.Fatal(e)
		}
		if bytes.Contains(raw, []byte(h.address)) || bytes.Contains(raw, []byte(identity.WalletAccountID)) {
			t.Fatal("socket wallet privacy leak")
		}
		var msg map[string]json.RawMessage
		json.Unmarshal(raw, &msg)
		if string(msg["type"]) == `"presence"` {
			if !bytes.Contains(raw, []byte(`"btcName":"alice.btc"`)) {
				t.Fatal("presence lacks domain")
			}
			presence = true
		}
		if string(msg["type"]) == `"chat"` {
			if string(msg["btcName"]) != `"alice.btc"` || string(msg["characterIdentityId"]) != `"`+identity.ID+`"` {
				t.Fatal("chat identity spoofed")
			}
			chat = true
		}
	}
}
