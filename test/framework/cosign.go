package framework

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/sigstore/cosign/v2/cmd/cosign/cli"
)

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

// SignOptions is a struct to hold the options for signing a container
type SignOptions struct {
	KeyName       string
	Image         string
	SignatureRepo string
}

// SignContainer signs the container with the provided private key
func (f *Framework) SignContainer(t *testing.T, opts SignOptions) {
	// TODO: find a way to simplify this function - maybe use cosing CLI directly?
	// get SHA of the container image
	t.Setenv("COSIGN_PASSWORD", "")
	args := []string{
		"sign",
		opts.Image,
	}
	t.Setenv("COSIGN_PASSWORD", "")
	cmd := cli.New()
	_ = cmd.Flags().Set("timeout", "30s")
	cmd.SetArgs(args)

	// find the sign subcommand in the commands slice
	for _, c := range cmd.Commands() {
		if c.Name() == "sign" {
			cmd = c
			break
		}
	}

	// if the signature repository is different from the image, set the COSIGN_REPOSITORY environment variable
	// to push the signature to the specified repository
	if opts.SignatureRepo != opts.Image {
		t.Setenv("COSIGN_REPOSITORY", opts.SignatureRepo)
	}

	_ = cmd.Flags().Set("key", fmt.Sprintf("%s.key", opts.KeyName))
	_ = cmd.Flags().Set("tlog-upload", "false")
	_ = cmd.Flags().Set("yes", "true")
	_ = cmd.Flags().Set("allow-http-registry", "true")
	err := cmd.Execute()
	if err != nil {
		f.Cleanup(t)
		t.Fatal(err)
	}
}
