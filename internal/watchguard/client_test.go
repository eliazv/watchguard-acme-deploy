package watchguard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/eliazv/watchguard-acme-deploy/internal/config"
)

func TestDocumentedFlow(t *testing.T) {
	var auths, creates, installs, deploys atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/oauth/token":
			auths.Add(1)
			id, password, ok := r.BasicAuth()
			if !ok || id != "id" || password != "password" {
				t.Error("wrong OAuth credentials")
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"access_token":"secret-token","expires_in":3600}`))
			return
		case strings.HasPrefix(r.URL.Path, prefix):
			if r.Header.Get("Authorization") != "Bearer secret-token" || r.Header.Get("WatchGuard-API-Key") != "api-secret" {
				t.Error("missing authentication headers")
			}
			switch {
			case strings.HasSuffix(r.URL.Path, "/devices"):
				w.Write([]byte(`{"count":1,"data":[{"id":"FB-123","name":"Test","cloud_managed":"yes"}]}`))
			case strings.HasSuffix(r.URL.Path, "/certificates") && r.Method == "GET":
				w.Write([]byte(`{"objects":[]}`))
			case strings.HasSuffix(r.URL.Path, "/certificates") && r.Method == "POST":
				creates.Add(1)
				var b map[string]any
				json.NewDecoder(r.Body).Decode(&b)
				if b["pvt_key"] != "private" || b["device"] != "FB-123" {
					t.Errorf("wrong create payload: %v", b)
				}
				w.Write([]byte(`{"id":"crt_1","device":"FB-123","name":"test"}`))
			case strings.HasSuffix(r.URL.Path, "/certificates/install"):
				installs.Add(1)
				w.Write([]byte(`{"id":"install_1","status":"in_progress"}`))
			case strings.HasSuffix(r.URL.Path, "/deployments"):
				deploys.Add(1)
				var b map[string]any
				json.NewDecoder(r.Body).Decode(&b)
				if b["full_config"] != true || b["staged"] != false {
					t.Errorf("wrong deployment payload: %v", b)
				}
				w.Write([]byte(`[{"id":"ddply_1","status":"pending"}]`))
			case strings.HasSuffix(r.URL.Path, "/transactions/ddply_1"):
				w.Write([]byte(`{"id":"ddply_1","status":"complete"}`))
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				http.NotFound(w, r)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c := New(config.Config{AccountID: "ACC-1", APIURL: server.URL, AuthURL: server.URL, APIKey: "api-secret", AccessID: "id", AccessPassword: "password"})
	ctx := context.Background()
	if _, err := c.Device(ctx, "FB-123"); err != nil {
		t.Fatal(err)
	}
	list, err := c.Certificates(ctx, "FB-123")
	if err != nil || len(list) != 0 {
		t.Fatalf("list: %v %v", list, err)
	}
	created, err := c.CreateCertificate(ctx, "FB-123", "test", "pem", "private")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.InstallCertificate(ctx, "FB-123", created.ID); err != nil {
		t.Fatal(err)
	}
	tx, err := c.DeployConfiguration(ctx, "FB-123")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Transaction(ctx, tx.ID); err != nil {
		t.Fatal(err)
	}
	if auths.Load() != 1 || creates.Load() != 1 || installs.Load() != 1 || deploys.Load() != 1 {
		t.Fatalf("unexpected counts: %d %d %d %d", auths.Load(), creates.Load(), installs.Load(), deploys.Load())
	}
}
func TestMutationNotRetriedAndRedacted(t *testing.T) {
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			w.Write([]byte(`{"access_token":"secret-token"}`))
			return
		}
		calls.Add(1)
		http.Error(w, "private=very-secret", 500)
	}))
	defer s.Close()
	c := New(config.Config{AccountID: "ACC-1", APIURL: s.URL, AuthURL: s.URL, APIKey: "api-secret", AccessID: "id", AccessPassword: "password"})
	_, err := c.CreateCertificate(context.Background(), "FB-1", "name", "pem", "very-secret")
	if err == nil || strings.Contains(err.Error(), "very-secret") || calls.Load() != 1 {
		t.Fatalf("mutation retry/redaction failed: calls=%d err=%v", calls.Load(), err)
	}
}

func TestReadRetriesAfterExpiredToken(t *testing.T) {
	var auths, reads atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			n := auths.Add(1)
			fmt.Fprintf(w, `{"access_token":"token-%d","expires_in":3600}`, n)
			return
		}
		if reads.Add(1) == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"data":[]}`))
	}))
	defer s.Close()
	c := New(config.Config{AccountID: "ACC-1", APIURL: s.URL, AuthURL: s.URL, APIKey: "key", AccessID: "id", AccessPassword: "password"})
	if _, err := c.Devices(context.Background()); err != nil {
		t.Fatal(err)
	}
	if auths.Load() != 2 || reads.Load() != 2 {
		t.Fatalf("expected one token refresh: auth=%d reads=%d", auths.Load(), reads.Load())
	}
}
