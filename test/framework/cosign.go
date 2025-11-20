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
	"time"

	"github.com/sigstore/cosign/v3/cmd/cosign/cli/importkeypair"
	"github.com/sigstore/cosign/v3/cmd/cosign/cli/options"

	"github.com/sigstore/cosign/v3/cmd/cosign/cli"
	"github.com/sigstore/cosign/v3/cmd/cosign/cli/sign"
)

const ImportKeySuffix = "imported"

// Pub contains the public key and its path
type Pub struct {
	Key  string
	Path string
}

// Priv contains the private key and its path
type Priv struct {
	Key  string
	Path string
}

// SignOptions is a struct to hold the options for signing a container
type SignOptions struct {
	KeyPath       string
	Image         string
	SignatureRepo string
}

// KeyFunc is a function that generates a keypair by using the testing framework
type KeyFunc func(f *Framework, name string) (Priv, Pub)

// cleanupKeys removes all keypair files from the testing directory
func (f *Framework) cleanupKeys() {
	f.t.Logf("cleaning up keypair files")
	files, err := os.ReadDir(".")
	if err != nil {
		f.err = fmt.Errorf("failed reading directory: %v", err)
		return
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		reKey := regexp.MustCompile(".*.key")
		rePub := regexp.MustCompile(".*.pub")
		if reKey.MatchString(file.Name()) || rePub.MatchString(file.Name()) {
			err = os.Remove(file.Name())
			if err != nil {
				f.err = fmt.Errorf("failed to remove file: %v", err)
				return
			}
		}
	}
	f.t.Logf("cleaned up keypair files")
}

// CreateECDSAKeyPair generates an ECDSA keypair and saves the keys to the current directory
func CreateECDSAKeyPair(f *Framework, name string) (Priv, Pub) {
	if f.err != nil {
		return Priv{}, Pub{}
	}

	args := []string{fmt.Sprintf("--output-key-prefix=%s", name)}
	err := os.Setenv("COSIGN_PASSWORD", "")
	if err != nil {
		f.err = err
		return Priv{}, Pub{}
	}
	cmd := cli.GenerateKeyPair()
	cmd.SetArgs(args)
	err = cmd.Execute()
	if err != nil {
		f.err = err
		return Priv{}, Pub{}
	}

	// read private key and public key from the current directory
	privateKey, err := os.ReadFile(fmt.Sprintf("%s.key", name))
	if err != nil {
		f.err = err
		return Priv{}, Pub{}
	}
	pubKey, err := os.ReadFile(fmt.Sprintf("%s.pub", name))
	if err != nil {
		f.err = err
		return Priv{}, Pub{}
	}

	return Priv{
			Key:  string(privateKey),
			Path: fmt.Sprintf("%s.key", name),
		}, Pub{
			Key:  string(pubKey),
			Path: fmt.Sprintf("%s.pub", name),
		}
}

// CreateRSAKeyPair generates an RSA keypair and saves the keys to the current directory
func CreateRSAKeyPair(f *Framework, name string) (Priv, Pub) {
	if f.err != nil {
		return Priv{}, Pub{}
	}

	pkey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		f.err = fmt.Errorf("failed to generate RSA key: %v", err)
		return Priv{}, Pub{}
	}
	privBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(pkey),
	})

	err = os.WriteFile(fmt.Sprintf("%s.key", name), privBytes, 0o644)
	if err != nil {
		f.err = fmt.Errorf("failed to write private key to file: %v", err)
		return Priv{}, Pub{}
	}

	// Generate and save the public key to a PEM file
	pubKey := &pkey.PublicKey

	pubASN1, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		f.err = fmt.Errorf("failed to marshal public key: %v", err)
		return Priv{}, Pub{}
	}
	pubBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubASN1,
	})
	err = os.WriteFile(fmt.Sprintf("%s.pub", name), pubBytes, 0o644)
	if err != nil {
		f.err = fmt.Errorf("failed to write public key to file: %v", err)
		return Priv{}, Pub{}
	}

	f.t.Setenv("COSIGN_PASSWORD", "")
	// import the keypair into cosign for signing
	err = importkeypair.ImportKeyPairCmd(context.Background(), options.ImportKeyPairOptions{
		Key:             fmt.Sprintf("%s.key", name),
		OutputKeyPrefix: fmt.Sprintf("%s-%s", name, ImportKeySuffix),
	}, []string{})
	if err != nil {
		f.err = fmt.Errorf("failed to import keypair: %v", err)
		return Priv{}, Pub{}
	}

	// read private key and public key from the current directory
	privBytes, err = os.ReadFile(fmt.Sprintf("%s-%s.key", name, ImportKeySuffix))
	if err != nil {
		f.err = fmt.Errorf("failed reading private key: %v", err)
		return Priv{}, Pub{}
	}

	pubBytes, err = os.ReadFile(fmt.Sprintf("%s-%s.pub", name, ImportKeySuffix))
	if err != nil {
		f.err = fmt.Errorf("failed reading public key: %v", err)
		return Priv{}, Pub{}
	}

	return Priv{
			Key:  string(privBytes),
			Path: fmt.Sprintf("%s-%s.key", name, ImportKeySuffix),
		}, Pub{
			Key:  string(pubBytes),
			Path: fmt.Sprintf("%s-%s.pub", name, ImportKeySuffix),
		}
}

// SignContainer signs the container using the provided SignOptions
func (f *Framework) SignContainer(opts SignOptions) {
	if f.err != nil {
		return
	}

	// get SHA of the container image
	f.t.Setenv("COSIGN_PASSWORD", "")

	// if the signature repository is different from the image, set the COSIGN_REPOSITORY environment variable
	// to push the signature to the specified repository
	if opts.SignatureRepo != opts.Image {
		f.t.Setenv("COSIGN_REPOSITORY", opts.SignatureRepo)
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
		f.err = fmt.Errorf("failed to sign container: %v", err)
	}
}
