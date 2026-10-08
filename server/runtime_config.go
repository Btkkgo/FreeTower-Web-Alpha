package main

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// RuntimeConfig is the single validated runtime authority. GitSHA is safe for health output.
type RuntimeConfig struct {
	Environment          string
	CanonicalEnvironment bool
	PublicOrigin         string
	RequireDatabase      bool
	GitSHA               string
}

func (c RuntimeConfig) Public() bool {
	return c.Environment == "staging" || c.Environment == "production"
}
func runtimeSetting(lookup func(string) (string, bool), canonical, legacy string) string {
	if v, ok := lookup(canonical); ok {
		return v
	}
	v, _ := lookup(legacy)
	return v
}

var commitPattern = regexp.MustCompile(`^[a-fA-F0-9]{7,40}$`)

func loadRuntimeConfig(lookup func(string) (string, bool)) (RuntimeConfig, error) {
	_, canonical := lookup("FREETOWER_ENV")
	c := RuntimeConfig{Environment: runtimeSetting(lookup, "FREETOWER_ENV", "FT_ENV"), CanonicalEnvironment: canonical, PublicOrigin: runtimeSetting(lookup, "FREETOWER_PUBLIC_ORIGIN", "FT_PUBLIC_ORIGIN")}
	if c.Environment == "" && !canonical {
		c.Environment = "development"
	}
	switch c.Environment {
	case "development", "test", "staging", "production":
	default:
		return c, errors.New("invalid FREETOWER_ENV")
	}
	if value := runtimeSetting(lookup, "FREETOWER_REQUIRE_DATABASE", "FT_REQUIRE_DATABASE"); value != "" {
		v, e := strconv.ParseBool(value)
		if e != nil {
			return c, errors.New("invalid FREETOWER_REQUIRE_DATABASE")
		}
		c.RequireDatabase = v
	}
	c.RequireDatabase = c.RequireDatabase || c.Public()
	if c.Public() {
		u, e := url.Parse(c.PublicOrigin)
		if e != nil || u.Scheme != "https" || u.Host == "" || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" || c.PublicOrigin != "https://"+u.Host {
			return c, errors.New("public runtime requires an exact HTTPS FREETOWER_PUBLIC_ORIGIN")
		}
		host := strings.ToLower(u.Hostname())
		ip := net.ParseIP(host)
		if loopbackHost(host) || strings.HasSuffix(host, ".localhost") || host == "railway.internal" || strings.HasSuffix(host, ".railway.internal") || ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast()) {
			return c, errors.New("public runtime requires a public HTTPS origin")
		}
	}
	database, _ := lookup("DATABASE_URL")
	if c.RequireDatabase && database == "" {
		return c, errors.New("runtime requires DATABASE_URL")
	}
	sha := runtimeSetting(lookup, "FREETOWER_GIT_SHA", "FT_GIT_SHA")
	if commitPattern.MatchString(sha) {
		c.GitSHA = sha[:7]
	}
	return c, nil
}
func (c RuntimeConfig) originAllowed(r *http.Request) bool {
	if c.Public() {
		return c.PublicOrigin != "" && r.Header.Get("Origin") == c.PublicOrigin
	}
	return sameOrigin(r)
}
