package main

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/eliazv/watchguard-acme-deploy/internal/certificate"
	"github.com/eliazv/watchguard-acme-deploy/internal/config"
	"github.com/eliazv/watchguard-acme-deploy/internal/watchguard"
)

var safeName = regexp.MustCompile(`^[A-Za-z0-9 ()*._-]{1,58}$`)

type result struct {
	Action              string    `json:"action"`
	Device              string    `json:"device,omitempty"`
	Name                string    `json:"name,omitempty"`
	Fingerprint         string    `json:"fingerprint,omitempty"`
	Subject             string    `json:"subject,omitempty"`
	SAN                 []string  `json:"san,omitempty"`
	Expires             string    `json:"expires,omitempty"`
	CertificateID       string    `json:"certificate_id,omitempty"`
	InstallID           string    `json:"install_id,omitempty"`
	InstallStatus       string    `json:"install_status,omitempty"`
	DeploymentID        string    `json:"deployment_id,omitempty"`
	DeploymentStatus    string    `json:"deployment_status,omitempty"`
	TLSFingerprint      string    `json:"tls_fingerprint,omitempty"`
	Certificates        []summary `json:"certificates,omitempty"`
	TLSMatchesInventory *bool     `json:"tls_matches_inventory,omitempty"`
	Message             string    `json:"message,omitempty"`
}

type summary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Fingerprint string `json:"fingerprint"`
	Expiration  int64  `json:"expiration"`
}

func emit(w io.Writer, format string, v any) error {
	if format == "json" {
		e := json.NewEncoder(w)
		e.SetIndent("", "  ")
		return e.Encode(v)
	}
	switch x := v.(type) {
	case result:
		fmt.Fprintf(w, "Action: %s\nDevice: %s\n", x.Action, x.Device)
		if x.Name != "" {
			fmt.Fprintf(w, "Certificate: %s\n", x.Name)
		}
		if x.Fingerprint != "" {
			fmt.Fprintf(w, "SHA256: %s\n", x.Fingerprint)
		}
		if x.Subject != "" {
			fmt.Fprintf(w, "Subject: %s\nSAN: %s\nExpires: %s\n", x.Subject, strings.Join(x.SAN, ", "), x.Expires)
		}
		if x.CertificateID != "" {
			fmt.Fprintf(w, "Certificate ID: %s\n", x.CertificateID)
		}
		if x.InstallID != "" {
			fmt.Fprintf(w, "Install command: %s (%s)\n", x.InstallID, x.InstallStatus)
		}
		if x.DeploymentID != "" {
			fmt.Fprintf(w, "Configuration deployment: %s (%s)\n", x.DeploymentID, x.DeploymentStatus)
		}
		if x.TLSFingerprint != "" {
			fmt.Fprintf(w, "Served TLS SHA256: %s\n", x.TLSFingerprint)
		}
		for _, c := range x.Certificates {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\n", c.ID, c.Name, c.Type, c.Fingerprint, c.Expiration)
		}
		if x.TLSMatchesInventory != nil {
			fmt.Fprintf(w, "Served certificate in inventory: %t\n", *x.TLSMatchesInventory)
		}
		if x.Message != "" {
			fmt.Fprintln(w, x.Message)
		}
	case []watchguard.Device:
		for _, d := range x {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", d.ID, d.Name, d.Model, d.Version, d.CloudManaged, d.State)
		}
	case []watchguard.Certificate:
		for _, c := range x {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\n", c.ID, c.Name, c.Type, c.Fingerprint, c.Expiration)
		}
	}
	return nil
}
func formatOK(s string) error {
	if s != "text" && s != "json" {
		return errors.New("--output must be text or json")
	}
	return nil
}
func certFingerprint(c watchguard.Certificate) string {
	if c.PEM != "" {
		block, _ := pem.Decode([]byte(c.PEM))
		if block != nil {
			if parsed, err := x509.ParseCertificate(block.Bytes); err == nil {
				return certificate.Fingerprint(parsed.Raw)
			}
		}
	}
	return c.Fingerprint
}

func findExisting(certs []watchguard.Certificate, fingerprint, name string) (*watchguard.Certificate, error) {
	var existing *watchguard.Certificate
	for i := range certs {
		if certificate.NormalizeFingerprint(certFingerprint(certs[i])) == certificate.NormalizeFingerprint(fingerprint) {
			if existing != nil {
				return nil, errors.New("multiple matching remote certificates; select and resolve duplicates manually")
			}
			existing = &certs[i]
		}
	}
	if existing == nil {
		for _, c := range certs {
			if c.Name == name {
				return nil, fmt.Errorf("certificate name %q already exists with a different fingerprint", name)
			}
		}
	}
	return existing, nil
}

func run(ctx context.Context, args []string, w io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: wgcert devices|check|deploy [flags]")
	}
	if args[0] == "help" || args[0] == "--help" {
		_, err := fmt.Fprintln(w, "usage: wgcert devices|check|deploy [flags]\nRun wgcert <command> --help for command flags.")
		return err
	}
	cmd := args[0]
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(w)
	device := fs.String("device", "", "Firebox ID")
	certPath := fs.String("cert", "", "certificate fullchain PEM")
	keyPath := fs.String("key", "", "private key PEM")
	name := fs.String("name", "", "certificate name base")
	dry := fs.Bool("dry-run", false, "show plan without mutations")
	output := fs.String("output", "text", "text or json")
	verifyHost := fs.String("verify-host", "", "TLS host:port")
	deployConfig := fs.Bool("deploy-config", false, "deploy all pending device configuration changes")
	wait := fs.Duration("wait", 0, "wait for deployment transaction, e.g. 2m")
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if len(fs.Args()) > 0 {
		return errors.New("unexpected positional arguments")
	}
	if err := formatOK(*output); err != nil {
		return err
	}
	if cmd != "devices" && cmd != "check" && cmd != "deploy" {
		return fmt.Errorf("unknown command %q", cmd)
	}
	if cmd != "devices" && *device == "" {
		return errors.New("--device is required")
	}
	if cmd == "deploy" && (*certPath == "" || *keyPath == "") {
		return errors.New("--cert and --key are required")
	}
	if *wait < 0 {
		return errors.New("--wait must be non-negative")
	}
	if *wait > 0 && !(*deployConfig) {
		return errors.New("--wait requires --deploy-config")
	}
	var bundle certificate.Bundle
	var err error
	if cmd == "deploy" {
		bundle, err = certificate.Load(*certPath, *keyPath)
		if err != nil {
			return err
		}
	}
	if cmd == "deploy" && *name == "" {
		*name = "wgcert-" + strings.ToLower(strings.ReplaceAll(bundle.Fingerprint, ":", ""))[:16]
	}
	if cmd == "deploy" && !safeName.MatchString(*name) {
		return errors.New("--name must be 1-58 allowed characters: letters, digits, spaces, (), *, ., - and _")
	}
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}
	client := watchguard.New(cfg)
	if cmd == "devices" {
		devices, err := client.Devices(ctx)
		if err != nil {
			return err
		}
		return emit(w, *output, devices)
	}
	_, err = client.Device(ctx, *device)
	if err != nil {
		return err
	}
	certs, err := client.Certificates(ctx, *device)
	if err != nil {
		return err
	}
	if cmd == "check" {
		r := result{Action: "check", Device: *device, Message: "API inventory describes cloud objects; verify the intended Firebox service to determine what is active."}
		for _, c := range certs {
			r.Certificates = append(r.Certificates, summary{c.ID, c.Name, c.Type, certFingerprint(c), c.Expiration})
		}
		if *verifyHost != "" {
			fp, err := certificate.ServedFingerprint(*verifyHost)
			if err != nil {
				return err
			}
			r.TLSFingerprint = fp
			matched := false
			for _, c := range r.Certificates {
				if certificate.NormalizeFingerprint(c.Fingerprint) == certificate.NormalizeFingerprint(fp) {
					matched = true
					break
				}
			}
			r.TLSMatchesInventory = &matched
		}
		return emit(w, *output, r)
	}
	san := append([]string{}, bundle.Leaf.DNSNames...)
	for _, ip := range bundle.Leaf.IPAddresses {
		san = append(san, ip.String())
	}
	r := result{Action: "plan", Device: *device, Name: *name, Fingerprint: bundle.Fingerprint, Subject: bundle.Leaf.Subject.String(), SAN: san, Expires: bundle.Leaf.NotAfter.UTC().Format(time.RFC3339)}
	existing, err := findExisting(certs, bundle.Fingerprint, *name)
	if err != nil {
		return err
	}
	if *dry {
		r.Message = "DRY RUN: no changes made. Would "
		if existing == nil {
			r.Message += "create, "
		}
		r.Message += "install certificate"
		if *deployConfig {
			r.Message += " and deploy all pending configuration changes"
		}
		return emit(w, *output, r)
	}
	if existing == nil {
		created, err := client.CreateCertificate(ctx, *device, *name, bundle.CertPEM, bundle.KeyPEM)
		if err != nil {
			return err
		}
		r.CertificateID = created.ID
	} else {
		r.CertificateID = existing.ID
		r.Name = existing.Name
	}
	installed, err := client.InstallCertificate(ctx, *device, r.CertificateID)
	if err != nil {
		return fmt.Errorf("certificate %s exists; install failed: %w", r.CertificateID, err)
	}
	r.Action = "install_requested"
	r.InstallID = installed.ID
	r.InstallStatus = installed.Status
	if *deployConfig {
		deployed, err := client.DeployConfiguration(ctx, *device)
		if err != nil {
			return fmt.Errorf("install command %s accepted; configuration deployment failed: %w", installed.ID, err)
		}
		r.DeploymentID = deployed.ID
		r.DeploymentStatus = deployed.Status
		r.Action = "deployment_requested"
		if *wait > 0 {
			deadline, cancel := context.WithTimeout(ctx, *wait)
			defer cancel()
			for {
				tx, err := client.Transaction(deadline, deployed.ID)
				if err != nil {
					return err
				}
				r.DeploymentStatus = tx.Status
				if tx.Status == "complete" {
					break
				}
				if tx.Status == "failed" || tx.Status == "imaged" || tx.Status == "timed_out" || tx.Status == "canceled" {
					return fmt.Errorf("configuration deployment %s ended with status %s", tx.ID, tx.Status)
				}
				select {
				case <-deadline.Done():
					return fmt.Errorf("configuration deployment %s did not complete: %w", tx.ID, deadline.Err())
				case <-time.After(3 * time.Second):
				}
			}
		}
	}
	if *verifyHost != "" {
		fp, err := certificate.ServedFingerprint(*verifyHost)
		if err != nil {
			return err
		}
		r.TLSFingerprint = fp
		if certificate.NormalizeFingerprint(fp) != certificate.NormalizeFingerprint(bundle.Fingerprint) {
			return fmt.Errorf("served TLS fingerprint does not match local certificate (install command %s)", installed.ID)
		}
	}
	r.Message = "WatchGuard accepted the command. Confirm it became active on the intended Firebox service; API inventory alone cannot prove this."
	return emit(w, *output, r)
}
func main() {
	ctx := context.Background()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
