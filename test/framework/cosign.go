package framework

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	optionsV2 "github.com/sigstore/cosign/v2/cmd/cosign/cli/options"
	signV2 "github.com/sigstore/cosign/v2/cmd/cosign/cli/sign"
	"github.com/sigstore/cosign/v3/cmd/cosign/cli"
	"github.com/sigstore/cosign/v3/cmd/cosign/cli/importkeypair"
	"github.com/sigstore/cosign/v3/cmd/cosign/cli/options"
	"github.com/sigstore/cosign/v3/cmd/cosign/cli/sign"
	"github.com/sigstore/sigstore-go/pkg/root"
)

const ImportKeySuffix = "imported"

// TestImage holds both the host and cluster image references for a test image
type TestImage struct {
	Host    string // Image reference for signing (host registry port)
	Cluster string // Image reference for deployment (cluster registry port)
}

// CreateTestImage creates a unique test image for a test case by copying the base image
// and adding a unique label. This ensures each test case has its own image with no signature conflicts.
func (f *Framework) CreateTestImage(baseImage, testName string) TestImage {
	if f.err != nil {
		return TestImage{}
	}

	// Sanitize test name for use as tag (lowercase, replace spaces/special chars)
	tag := strings.ToLower(testName)
	tag = strings.ReplaceAll(tag, " ", "-")
	tag = strings.ReplaceAll(tag, "_", "-")

	// Extract registry from base image
	parts := strings.Split(baseImage, "/")
	registry := parts[0]

	// Create new image reference with test-specific tag
	newImage := fmt.Sprintf("%s/busybox:%s", registry, tag)

	// Pull the source image
	srcRef, err := name.ParseReference(baseImage, name.Insecure)
	if err != nil {
		f.err = fmt.Errorf("failed to parse source image reference: %v", err)
		return TestImage{}
	}

	img, err := crane.Pull(baseImage, crane.Insecure)
	if err != nil {
		f.err = fmt.Errorf("failed to pull base image: %v", err)
		return TestImage{}
	}

	// Get current config and add label
	cfg, err := img.ConfigFile()
	if err != nil {
		f.err = fmt.Errorf("failed to get image config: %v", err)
		return TestImage{}
	}

	if cfg.Config.Labels == nil {
		cfg.Config.Labels = make(map[string]string)
	}
	cfg.Config.Labels["testcase"] = testName

	// Mutate image with new config
	img, err = mutate.ConfigFile(img, cfg)
	if err != nil {
		f.err = fmt.Errorf("failed to mutate image config: %v", err)
		return TestImage{}
	}

	// Push the new image
	dstRef, err := name.ParseReference(newImage, name.Insecure)
	if err != nil {
		f.err = fmt.Errorf("failed to parse destination image reference: %v", err)
		return TestImage{}
	}

	err = crane.Push(img, dstRef.String(), crane.Insecure)
	if err != nil {
		f.err = fmt.Errorf("failed to push test image: %v", err)
		return TestImage{}
	}

	f.t.Logf("Created test image %s from %s for test %s", newImage, srcRef, testName)

	// Build cluster image reference (always port 5000 inside cluster)
	clusterImage := fmt.Sprintf("k3d-registry.localhost:5000/busybox:%s", tag)

	return TestImage{
		Host:    newImage,
		Cluster: clusterImage,
	}
}

func signingConfigPath() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "signing-config.json")
}

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
	LegacyFormat  bool
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

	if opts.LegacyFormat {
		err := signV2.SignCmd(
			&optionsV2.RootOptions{
				Timeout: 30 * time.Second,
			},
			optionsV2.KeyOpts{
				KeyRef: opts.KeyPath,
			},
			optionsV2.SignOptions{
				Key:        opts.KeyPath,
				TlogUpload: false,
				Upload:     true,
				Registry: optionsV2.RegistryOptions{
					AllowHTTPRegistry: true,
				},
			},
			[]string{opts.Image},
		)
		if err != nil {
			f.err = fmt.Errorf("failed to sign container: %v", err)
		}
		return
	}

	sc, err := root.NewSigningConfigFromPath(signingConfigPath())
	if err != nil {
		f.err = fmt.Errorf("failed to load signing config: %v", err)
		return
	}

	err = sign.SignCmd(
		context.Background(),
		&options.RootOptions{
			Timeout: 30 * time.Second,
		},
		options.KeyOpts{
			SigningConfig: sc,
			KeyRef:        opts.KeyPath,
		},
		options.SignOptions{
			Key:             opts.KeyPath,
			NewBundleFormat: true,
			Upload:          true,
			Registry: options.RegistryOptions{
				AllowHTTPRegistry: true,
			},
		},
		[]string{opts.Image},
	)
	if err != nil {
		f.err = fmt.Errorf("failed to sign container: %v", err)
	}
}
