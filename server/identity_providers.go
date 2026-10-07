package main

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/text/unicode/norm"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var inscriptionIDPattern = regexp.MustCompile(`^[0-9a-f]{64}i[0-9]+$`)

func validBTCName(n string) bool {
	if !utf8.ValidString(n) || len(n) > 255 || !strings.HasSuffix(n, ".btc") || n != strings.ToLower(n) || !norm.NFC.IsNormalString(n) {
		return false
	}
	label := strings.TrimSuffix(n, ".btc")
	if label == "" {
		return false
	}
	for _, r := range label {
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.IsMark(r) && r != '-' {
			return false
		}
	}
	return true
}
func providerHTTP() *http.Client {
	return &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
func providerJSON(ctx context.Context, c *http.Client, endpoint, key string, out any) (int, error) {
	req, e := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if e != nil {
		return 0, errProvider
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cache-Control", "no-cache")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	res, e := c.Do(req)
	if e != nil {
		return 0, errProvider
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return res.StatusCode, errProvider
	}
	b, e := io.ReadAll(io.LimitReader(res.Body, 2*1024*1024+1))
	if e != nil || len(b) > 2*1024*1024 {
		return res.StatusCode, errProvider
	}
	if json.Unmarshal(b, out) != nil {
		return res.StatusCode, errProvider
	}
	return res.StatusCode, nil
}

type UniSatFractalOrdinalsProvider struct {
	http  *http.Client
	key   string
	allow map[string]bool
}
type inscriptionRecord struct {
	Metadata    json.RawMessage `json:"metadata"`
	Collection  json.RawMessage `json:"collection"`
	ID          string          `json:"inscriptionId"`
	Address     string          `json:"address"`
	ContentType string          `json:"contentType"`
	Height      int64           `json:"height"`
}

func (p *UniSatFractalOrdinalsProvider) get(ctx context.Context, path string, out any) (int, error) {
	if p.key == "" {
		return 0, errProvider
	}
	return providerJSON(ctx, p.http, "https://open-api-fractal.unisat.io"+path, p.key, out)
}
func (p *UniSatFractalOrdinalsProvider) info(ctx context.Context, id string) (*inscriptionRecord, error) {
	if !p.allow[id] || !inscriptionIDPattern.MatchString(id) {
		return nil, nil
	}
	var out struct {
		Code *int              `json:"code"`
		Data inscriptionRecord `json:"data"`
	}
	status, e := p.get(ctx, "/v1/indexer/inscription/info/"+id, &out)
	if status == 404 {
		return nil, nil
	}
	if e != nil || out.Code == nil || *out.Code != 0 {
		return nil, errProvider
	}
	if out.Data.ID != id || out.Data.Height <= 0 {
		return nil, nil
	}
	return &out.Data, nil
}
func (p *UniSatFractalOrdinalsProvider) VerifyOwnership(ctx context.Context, address, id string) (bool, error) {
	v, e := p.info(ctx, id)
	if e != nil || v == nil {
		return false, e
	}
	owner, e := canonicalAddress(v.Address)
	if e != nil {
		return false, errProvider
	}
	return owner == address, nil
}
func (p *UniSatFractalOrdinalsProvider) GetOwnedCharacterAssets(ctx context.Context, address string) ([]CharacterAsset, error) {
	result := []CharacterAsset{}
	if len(p.allow) == 0 {
		return result, nil
	}
	seen := map[string]bool{}
	expected := -1
	cursor := 0
	for page := 0; page < 20; page++ {
		var out struct {
			Code *int `json:"code"`
			Data struct {
				Total  *int                `json:"total"`
				Cursor *int                `json:"cursor"`
				Items  []inscriptionRecord `json:"inscription"`
			} `json:"data"`
		}
		_, e := p.get(ctx, fmt.Sprintf("/v1/indexer/address/%s/inscription-data?cursor=%d&size=100", url.PathEscape(address), cursor), &out)
		if e != nil || out.Code == nil || *out.Code != 0 || out.Data.Total == nil || out.Data.Cursor == nil || *out.Data.Cursor != cursor || *out.Data.Total < 0 || *out.Data.Total > 2000 {
			return nil, errProvider
		}
		if expected < 0 {
			expected = *out.Data.Total
		}
		if expected != *out.Data.Total || len(out.Data.Items) > 100 {
			return nil, errProvider
		}
		if len(out.Data.Items) == 0 && cursor < expected {
			return nil, errProvider
		}
		for _, v := range out.Data.Items {
			if seen[v.ID] {
				return nil, errProvider
			}
			seen[v.ID] = true
			if !p.allow[v.ID] {
				continue
			}
			info, e := p.info(ctx, v.ID)
			if e != nil {
				return nil, e
			}
			if info == nil {
				continue
			}
			owner, e := canonicalAddress(info.Address)
			if e != nil {
				return nil, errProvider
			}
			if owner != address {
				continue
			}
			result = append(result, CharacterAsset{InscriptionID: v.ID, AssetID: v.ID, Owner: owner, ContentType: info.ContentType, Metadata: safeAssetMetadata(info.Metadata), Collection: safeAssetMetadata(info.Collection), Status: "eligible"})
		}
		cursor += len(out.Data.Items)
		if cursor == expected {
			return result, nil
		}
		if cursor > expected {
			return nil, errProvider
		}
	}
	return nil, errProvider
}

type BNRPResolver struct{ http *http.Client }

func (p *BNRPResolver) Resolve(ctx context.Context, name string) (*BtcNameRecord, error) {
	if !validBTCName(name) {
		return nil, nil
	}
	var out struct {
		Name          string `json:"name"`
		Owner         string `json:"owner"`
		InscriptionID string `json:"inscription_id"`
		ResolvedAt    string `json:"resolved_at"`
	}
	status, e := providerJSON(ctx, p.http, "https://api.bnrp.name/v1/resolve/"+url.PathEscape(name), "", &out)
	if status == 404 || status == 422 {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	if out.Name != name || !inscriptionIDPattern.MatchString(out.InscriptionID) {
		return nil, errProvider
	}
	owner, e := canonicalAddress(out.Owner)
	if e != nil {
		return nil, errProvider
	}
	if _, e = time.Parse(time.RFC3339, out.ResolvedAt); e != nil {
		return nil, errProvider
	}
	return &BtcNameRecord{Name: out.Name, Owner: owner, Provider: "bnrp", Network: "bitcoin-mainnet"}, nil
}
func (p *BNRPResolver) ReverseLookup(ctx context.Context, address string) ([]BtcNameRecord, error) {
	out := struct {
		Address  string  `json:"address"`
		Name     *string `json:"name"`
		Verified bool    `json:"verified"`
	}{}
	_, e := providerJSON(ctx, p.http, "https://api.bnrp.name/v1/reverse/"+url.PathEscape(address), "", &out)
	if e != nil {
		return nil, e
	}
	owner, e := canonicalAddress(out.Address)
	if e != nil || owner != address {
		return nil, errProvider
	}
	result := []BtcNameRecord{}
	if out.Name == nil || !out.Verified {
		return result, nil
	}
	record, e := p.Resolve(ctx, *out.Name)
	if e != nil {
		return nil, e
	}
	if record != nil && record.Owner == address {
		result = append(result, *record)
	}
	return result, nil
}
func (p *BNRPResolver) VerifyOwnership(ctx context.Context, name, address string) (bool, error) {
	names, e := p.ReverseLookup(ctx, address)
	if e != nil {
		return false, e
	}
	for _, n := range names {
		if n.Name == name && n.Owner == address {
			return true, nil
		}
	}
	return false, nil
}

// Retain bounded opaque provider metadata; no content URL, HTML or SVG is executed.
func safeAssetMetadata(raw json.RawMessage) json.RawMessage {
	if len(raw) > 16*1024 || len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return raw
}
