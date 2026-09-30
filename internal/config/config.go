package config

import (
	"errors"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	AccountID, APIURL, AuthURL, APIKey, AccessID, AccessPassword string
}

func FromEnv() (Config, error) {
	c := Config{os.Getenv("WATCHGUARD_ACCOUNT_ID"), os.Getenv("WATCHGUARD_API_URL"), os.Getenv("WATCHGUARD_AUTH_URL"), os.Getenv("WATCHGUARD_API_KEY"), os.Getenv("WATCHGUARD_ACCESS_ID"), os.Getenv("WATCHGUARD_ACCESS_PASSWORD")}
	if c.AccountID == "" || c.APIURL == "" || c.AuthURL == "" || c.APIKey == "" || c.AccessID == "" || c.AccessPassword == "" {
		return c, errors.New("set WATCHGUARD_ACCOUNT_ID, WATCHGUARD_API_URL, WATCHGUARD_AUTH_URL, WATCHGUARD_API_KEY, WATCHGUARD_ACCESS_ID and WATCHGUARD_ACCESS_PASSWORD")
	}
	for _, raw := range []string{c.APIURL, c.AuthURL} {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"))) {
			return c, errors.New("API and authentication URLs must use HTTPS (HTTP is allowed only for localhost tests), without credentials, query or fragment")
		}
	}
	if strings.ContainsAny(c.AccountID, "/?#") {
		return c, errors.New("invalid WATCHGUARD_ACCOUNT_ID")
	}
	return c, nil
}
