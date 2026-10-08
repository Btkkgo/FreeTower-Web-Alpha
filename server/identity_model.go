package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
)

const identityNetwork = "fractal-mainnet"
const signatureScheme = "bip322-simple"

var errProvider = errors.New("Ownership verification unavailable; retry later")
var errIdentity = errors.New("Identity verification required")

type WalletAccount struct {
	ID                  string `json:"id"`
	Address             string `json:"address"`
	Network             string `json:"network"`
	Provider            string `json:"provider"`
	CreatedAt           int64  `json:"createdAt"`
	LastAuthenticatedAt int64  `json:"lastAuthenticatedAt"`
	Dev                 bool   `json:"dev,omitempty"`
}
type CharacterIdentity struct {
	ID               string           `json:"id"`
	WalletAccountID  string           `json:"walletAccountId"`
	CharacterAssetID string           `json:"characterAssetId"`
	BtcName          string           `json:"btcName"`
	CreatedAt        int64            `json:"createdAt"`
	LastVerifiedAt   int64            `json:"lastVerifiedAt"`
	Status           string           `json:"status"`
	Provider         string           `json:"provider"`
	Dev              bool             `json:"dev,omitempty"`
	Avatar           AvatarAppearance `json:"avatar"`
}
type CharacterAsset struct {
	Metadata      json.RawMessage `json:"metadata,omitempty"`
	Collection    json.RawMessage `json:"collection,omitempty"`
	InscriptionID string          `json:"inscriptionId"`
	Owner         string          `json:"owner"`
	ContentType   string          `json:"contentType"`
	AssetID       string          `json:"assetId"`
	Status        string          `json:"status"`
}
type BtcNameRecord struct {
	Name     string `json:"name"`
	Owner    string `json:"owner"`
	Provider string `json:"provider"`
	Network  string `json:"network"`
}
type CharacterAssetProvider interface {
	GetOwnedCharacterAssets(context.Context, string) ([]CharacterAsset, error)
	VerifyOwnership(context.Context, string, string) (bool, error)
}
type BtcNameResolver interface {
	ReverseLookup(context.Context, string) ([]BtcNameRecord, error)
	Resolve(context.Context, string) (*BtcNameRecord, error)
	VerifyOwnership(context.Context, string, string) (bool, error)
}
type walletSession struct {
	BootID     string `json:"bootId"`
	Binding    string `json:"binding,omitempty"`
	TokenHash  string `json:"tokenHash"`
	WalletID   string `json:"walletId"`
	IdentityID string `json:"identityId"`
	ExpiresAt  int64  `json:"expiresAt"`
	Network    string `json:"network"`
}
type identityData struct {
	Wallets    map[string]WalletAccount     `json:"wallets"`
	Identities map[string]CharacterIdentity `json:"identities"`
	Sessions   map[string]walletSession     `json:"sessions"`
}

func emptyIdentityData() identityData {
	return identityData{map[string]WalletAccount{}, map[string]CharacterIdentity{}, map[string]walletSession{}}
}

type walletChallenge struct {
	Nonce           string `json:"nonce"`
	Message         string `json:"message"`
	IssuedAt        int64  `json:"issuedAt"`
	ExpiresAt       int64  `json:"expiresAt"`
	SignatureScheme string `json:"signatureScheme"`
	Address         string `json:"-"`
	Origin          string `json:"-"`
	Binding         string `json:"-"`
}
type identityConfig struct {
	Environment                string
	CanonicalEnvironment       bool
	Development                bool
	DevEnabled                 bool
	LegacyEnabled              bool
	OwnershipAcceptanceEnabled bool
	Origin                     string
	Origins                    map[string]bool
	Assets                     map[string]bool
}
type identityService struct {
	// A fresh cryptographically random epoch deliberately invalidates all wallet
	// cookies after restart, including authority whose revocation failed to save.
	bootID     string
	revokedMu  sync.RWMutex
	revoked    map[string]int64
	mu         sync.Mutex
	config     identityConfig
	challenges map[string]walletChallenge
	assets     CharacterAssetProvider
	names      BtcNameResolver
	verify     func(string, string, string) error
}

func loadIdentityConfig() identityConfig {
	runtime, err := loadRuntimeConfig(os.LookupEnv)
	return identityConfigForRuntime(runtime, err)
}
func identityConfigForRuntime(runtime RuntimeConfig, err error) identityConfig {
	c := identityConfig{Environment: runtime.Environment, CanonicalEnvironment: runtime.CanonicalEnvironment, Development: err == nil && runtime.Environment == "development" && runtimeSetting(os.LookupEnv, "FREETOWER_ENV", "FT_ENV") == "development", Origin: runtime.PublicOrigin, Origins: map[string]bool{}, Assets: map[string]bool{}}
	c.DevEnabled = c.Development && os.Getenv("FT_ENABLE_DEV_IDENTITY") == "true"
	c.LegacyEnabled = c.Development && os.Getenv("FT_ENABLE_LEGACY_AUTH") == "true"
	c.OwnershipAcceptanceEnabled = c.Development && os.Getenv("FT_ENABLE_OWNERSHIP_ACCEPTANCE") == "true"
	origins := []string{c.Origin}
	if !runtime.Public() && err == nil {
		origins = append(origins, strings.Split(os.Getenv("FT_ALLOWED_ORIGINS"), ",")...)
	}
	for _, raw := range origins {
		raw = strings.TrimSpace(raw)
		u, e := url.Parse(raw)
		if e == nil && u.Host != "" && u.User == nil && u.Path == "" && u.RawQuery == "" && u.Fragment == "" && (u.Scheme == "https" || (c.Development && u.Scheme == "http" && loopbackHost(u.Hostname()))) {
			c.Origins[raw] = true
		}
	}
	for _, v := range strings.Split(os.Getenv("FT_CHARACTER_INSCRIPTION_ALLOWLIST"), ",") {
		v = strings.TrimSpace(v)
		if inscriptionIDPattern.MatchString(v) {
			c.Assets[v] = true
		}
	}
	return c
}
func loopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
func (s *identityService) originAllowed(r *http.Request) bool {
	return s.config.Origin != "" && s.config.Origins[s.config.Origin] && s.config.Origins[r.Header.Get("Origin")]
}
func (s *identityService) devAllowed(r *http.Request) bool {
	return s.config.DevEnabled && loopbackHost(remoteKey(r)) && s.originAllowed(r)
}
func newIdentityService() *identityService {
	return newIdentityServiceWithConfig(loadIdentityConfig())
}
func newIdentityServiceWithConfig(c identityConfig) *identityService {
	return &identityService{bootID: newID(), config: c, challenges: map[string]walletChallenge{}, assets: &UniSatFractalOrdinalsProvider{http: providerHTTP(), key: os.Getenv("UNISAT_FRACTAL_API_KEY"), allow: c.Assets}, names: &FractalNameResolver{http: providerHTTP(), key: os.Getenv("UNISAT_FRACTAL_API_KEY")}, verify: verifyBIP322}
}
func identityUser(i CharacterIdentity) User {
	mode := "wallet"
	if i.Dev {
		mode = "dev"
	}
	return User{ID: i.ID, Username: i.BtcName, CharacterIdentityID: i.ID, BtcName: i.BtcName, CharacterAssetID: i.CharacterAssetID, IdentityMode: mode, Avatar: i.Avatar}
}
