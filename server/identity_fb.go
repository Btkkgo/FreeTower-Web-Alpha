package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// UniSat's official wallet SDK supplies name resolution; its Fractal Open API
// independently confirms the resolved inscription's current owner. No name
// inscription is implicitly eligible as a Character asset.
type FractalNameResolver struct {
	http *http.Client
	key  string
}
type fractalNameInscription struct {
	ID            string `json:"inscriptionId"`
	Address       string `json:"address"`
	ContentType   string `json:"contentType"`
	ContentLength int    `json:"contentLength"`
	Height        int64  `json:"utxoHeight"`
	Confirmations int64  `json:"utxoConfirmation"`
}

func validFBName(name string) bool {
	if !strings.HasSuffix(name, ".fb") {
		return false
	}
	return len(name) <= 255 && validBTCName(strings.TrimSuffix(name, ".fb")+".btc")
}
func (p *FractalNameResolver) contentName(ctx context.Context, v fractalNameInscription) (string, error) {
	if !inscriptionIDPattern.MatchString(v.ID) || v.Height <= 0 || v.Confirmations < 3 || v.ContentLength <= 0 || v.ContentLength > 4096 || strings.Split(v.ContentType, ";")[0] != "text/plain" {
		return "", nil
	}
	var body struct {
		Protocol  string `json:"p"`
		Operation string `json:"op"`
		Name      string `json:"name"`
	}
	// Never follow content/preview URLs supplied by inscriptions or providers.
	req, e := http.NewRequestWithContext(ctx, "GET", "https://fractal-static.unisat.io/content/"+v.ID, nil)
	if e != nil {
		return "", errProvider
	}
	req.Header.Set("Cache-Control", "no-cache")
	res, e := p.http.Do(req)
	if e != nil {
		return "", errProvider
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", errProvider
	}
	raw, e := io.ReadAll(io.LimitReader(res.Body, 4097))
	if e != nil || len(raw) > 4096 {
		return "", errProvider
	}
	// Other text inscriptions in the wallet are not name-provider failures.
	if json.Unmarshal(raw, &body) != nil {
		return "", nil
	}
	if body.Protocol != "sns" || body.Operation != "reg" || !validFBName(body.Name) {
		return "", nil
	}
	return body.Name, nil
}
func (p *FractalNameResolver) resolveInscription(ctx context.Context, name string) (*BtcNameRecord, string, error) {
	if !validFBName(name) {
		return nil, "", nil
	}
	if p.key == "" {
		return nil, "", errProvider
	}
	var out struct {
		Code *int                    `json:"code"`
		Data *fractalNameInscription `json:"data"`
	}
	_, e := providerJSON(ctx, p.http, "https://wallet-api-fractal.unisat.io/v5/address/search?domain="+url.QueryEscape(name), "", &out)
	if e != nil || out.Code == nil || *out.Code != 0 {
		return nil, "", errProvider
	}
	if out.Data == nil {
		return nil, "", nil
	}
	v := *out.Data
	got, e := p.contentName(ctx, v)
	if e != nil {
		return nil, "", e
	}
	if got != name {
		return nil, "", nil
	}
	owner, e := canonicalAddress(v.Address)
	if e != nil {
		return nil, "", errProvider
	}
	var indexed struct {
		Code *int              `json:"code"`
		Data inscriptionRecord `json:"data"`
	}
	_, e = providerJSON(ctx, p.http, "https://open-api-fractal.unisat.io/v1/indexer/inscription/info/"+v.ID, p.key, &indexed)
	if e != nil || indexed.Code == nil || *indexed.Code != 0 {
		return nil, "", errProvider
	}
	current, e := canonicalAddress(indexed.Data.Address)
	if e != nil {
		return nil, "", errProvider
	}
	if indexed.Data.ID != v.ID || indexed.Data.Height <= 0 || current != owner {
		return nil, "", nil
	}
	return &BtcNameRecord{Name: name, Owner: owner, Provider: "unisat-fractal-satsnames", Network: identityNetwork}, v.ID, nil
}
func (p *FractalNameResolver) Resolve(ctx context.Context, name string) (*BtcNameRecord, error) {
	r, _, e := p.resolveInscription(ctx, name)
	return r, e
}
func (p *FractalNameResolver) VerifyOwnership(ctx context.Context, name, address string) (bool, error) {
	r, e := p.Resolve(ctx, name)
	return r != nil && r.Owner == address, e
}
func (p *FractalNameResolver) ReverseLookup(ctx context.Context, address string) ([]BtcNameRecord, error) {
	if p.key == "" {
		return nil, errProvider
	}
	result := []BtcNameRecord{}
	seen := map[string]bool{}
	names := map[string]bool{}
	expected, cursor := -1, 0
	for page := 0; page < 20; page++ {
		var out struct {
			Code *int `json:"code"`
			Data struct {
				Total *int                     `json:"total"`
				List  []fractalNameInscription `json:"list"`
			} `json:"data"`
		}
		_, e := providerJSON(ctx, p.http, fmt.Sprintf("https://wallet-api-fractal.unisat.io/v5/address/inscriptions?address=%s&cursor=%d&size=100", url.QueryEscape(address), cursor), "", &out)
		if e != nil || out.Code == nil || *out.Code != 0 || out.Data.Total == nil || *out.Data.Total < 0 || *out.Data.Total > 2000 || len(out.Data.List) > 100 {
			return nil, errProvider
		}
		if expected < 0 {
			expected = *out.Data.Total
		}
		if expected != *out.Data.Total || (len(out.Data.List) == 0 && cursor < expected) {
			return nil, errProvider
		}
		for _, v := range out.Data.List {
			if !inscriptionIDPattern.MatchString(v.ID) || seen[v.ID] {
				return nil, errProvider
			}
			seen[v.ID] = true
			owner, e := canonicalAddress(v.Address)
			if e != nil {
				return nil, errProvider
			}
			if owner != address {
				continue
			}
			name, e := p.contentName(ctx, v)
			if e != nil {
				return nil, e
			}
			if name == "" || names[name] {
				continue
			}
			forward, id, e := p.resolveInscription(ctx, name)
			if e != nil {
				return nil, e
			}
			if forward != nil && forward.Owner == address && id == v.ID {
				result = append(result, *forward)
				names[name] = true
			}
		}
		cursor += len(out.Data.List)
		if cursor == expected {
			return result, nil
		}
		if cursor > expected {
			return nil, errProvider
		}
	}
	return nil, errProvider
}
