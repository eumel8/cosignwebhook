package test

import (
	"fmt"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli"
	"os"
	"testing"
)

// createKeys creates a signing keypair for cosing with the provided name
func createKeys(t testing.TB, name string) (string, string) {
	args := []string{fmt.Sprintf("--output-key-prefix=%s", name)}
	err := os.Setenv("COSIGN_PASSWORD", "")
	if err != nil {
		t.Fatalf("failed setting COSIGN_PASSWORD: %v", err)
	}
	cmd := cli.GenerateKeyPair()
	cmd.SetArgs(args)
	err = cmd.Execute()
	if err != nil {
		t.Fatalf("failed generating keypair: %v", err)
	}

	// read private key and public key from the current directory
	privateKey, err := os.ReadFile(fmt.Sprintf("%s.key", name))
	if err != nil {
		t.Fatalf("failed reading private key: %v", err)
	}
	pubKey, err := os.ReadFile(fmt.Sprintf("%s.pub", name))
	if err != nil {
		t.Fatalf("failed reading public key: %v", err)
	}

	return string(privateKey), string(pubKey)
}

func TestCreateKeyPair(t *testing.T) {
	priv, pub := createKeys(t, "test")
	if priv == "" {
		t.Fatal("private key is empty")
	}
	if pub == "" {
		t.Fatal("public key is empty")
	}
}
