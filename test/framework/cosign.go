package framework

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/sigstore/cosign/v2/cmd/cosign/cli/importkeypair"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/options"

	"github.com/sigstore/cosign/v2/cmd/cosign/cli"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/sign"
)

const ImportKeySuffix = "imported"

// cleanupKeys removes all keypair files from the testing directory
func cleanupKeys(t testing.TB) {
	t.Logf("cleaning up keypair files")
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("failed reading directory: %v", err)
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		reKey := regexp.MustCompile(".*.key")
		rePub := regexp.MustCompile(".*.pub")
		if reKey.MatchString(f.Name()) || rePub.MatchString(f.Name()) {
			err = os.Remove(f.Name())
			if err != nil {
				t.Fatalf("failed removing file %s: %v", f.Name(), err)
			}
		}
	}
	t.Logf("cleaned up keypair files")
}

// CreateKeys creates a signing keypair for cosing with the provided name
func (f *Framework) CreateKeys(t testing.TB, name string) (private string, public string) {
	args := []string{fmt.Sprintf("--output-key-prefix=%s", name)}
	err := os.Setenv("COSIGN_PASSWORD", "")
	if err != nil {
		t.Fatalf("failed setting COSIGN_PASSWORD: %v", err)
	}
	cmd := cli.GenerateKeyPair()
	cmd.SetArgs(args)
	err = cmd.Execute()
	if err != nil {
		f.Cleanup(t)
	}

	// read private key and public key from the current directory
	privateKey, err := os.ReadFile(fmt.Sprintf("%s.key", name))
	if err != nil {
		f.Cleanup(t)
	}
	pubKey, err := os.ReadFile(fmt.Sprintf("%s.pub", name))
	if err != nil {
		f.Cleanup(t)
	}

	return string(privateKey), string(pubKey)
}

// CreateRSAKeyPair creates an RSA keypair for signing with the provided name
func (f *Framework) CreateRSAKeyPair(t *testing.T, name string) (private string, public string) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		f.Cleanup(t)
		t.Fatal(err)
	}

	privBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	})

	err = os.WriteFile(fmt.Sprintf("%s.key", name), privBytes, 0o644)
	if err != nil {
		t.Errorf("failed to write private key to file: %v", err)
		return "", ""
	}

	// Generate and save the public key to a PEM file
	pub := &priv.PublicKey

	pubASN1, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		f.Cleanup(t)
		t.Fatal(err)
	}
	pubBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubASN1,
	})
	err = os.WriteFile(fmt.Sprintf("%s.pub", name), pubBytes, 0o644)
	if err != nil {
		t.Errorf("failed to write public key to file: %v", err)
		return "", ""
	}

	t.Setenv("COSIGN_PASSWORD", "")
	// import the keypair into cosign for signing
	err = importkeypair.ImportKeyPairCmd(context.Background(), options.ImportKeyPairOptions{
		Key:             fmt.Sprintf("%s.key", name),
		OutputKeyPrefix: fmt.Sprintf("%s-%s", name, ImportKeySuffix),
	}, []string{})
	if err != nil {
		t.Errorf("failed to import keypair to cosign: %v", err)
		return "", ""
	}

	// read private key and public key from the current directory
	privBytes, err = os.ReadFile(fmt.Sprintf("%s-%s.key", name, ImportKeySuffix))
	if err != nil {
		f.Cleanup(t)
		t.Fatal(err)
	}

	pubBytes, err = os.ReadFile(fmt.Sprintf("%s-%s.pub", name, ImportKeySuffix))
	if err != nil {
		f.Cleanup(t)
		t.Fatal(err)
	}

	return string(privBytes), string(pubBytes)
}

// SignOptions is a struct to hold the options for signing a container
type SignOptions struct {
	KeyPath       string
	Image         string
	SignatureRepo string
}

// SignContainer signs the container with the provided private key
func (f *Framework) SignContainer(t *testing.T, opts SignOptions) {
	// get SHA of the container image
	t.Setenv("COSIGN_PASSWORD", "")

	// if the signature repository is different from the image, set the COSIGN_REPOSITORY environment variable
	// to push the signature to the specified repository
	if opts.SignatureRepo != opts.Image {
		t.Setenv("COSIGN_REPOSITORY", opts.SignatureRepo)
	}
	err := sign.SignCmd(
		&options.RootOptions{
			Timeout: 30 * time.Second,
		},
		options.KeyOpts{
			KeyRef: opts.KeyPath,
		},
		options.SignOptions{
			Key:        opts.KeyPath,
			TlogUpload: false,
			Upload:     true,
		},
		[]string{opts.Image},
	)
	if err != nil {
		f.Cleanup(t)
		t.Fatal(err)
	}
}
