package framework

import (
	"fmt"
	"os"
	"testing"
)

func TestFramework_CreateRSAKeyPair(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "success",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Framework{}
			priv, pub := f.CreateRSAKeyPair(t, tt.name)
			defer f.Cleanup(t)

			if priv == "" || pub == "" {
				t.Fatal("failed to create RSA key pair")
			}

			privStat, err := os.Stat(fmt.Sprintf("%s.key", tt.name))
			if err != nil || privStat.Size() == 0 {
				t.Fatal("failed to create private key")
			}
			pubStat, err := os.Stat(fmt.Sprintf("%s.pub", tt.name))
			if err != nil || pubStat.Size() == 0 {
				t.Fatal("failed to create public key")
			}

			coPrivStat, err := os.Stat("import-cosign.key")
			if err != nil || coPrivStat.Size() == 0 {
				t.Fatal("failed to create cosign private key")
			}
			coPubStat, err := os.Stat("import-cosign.pub")
			if err != nil || coPubStat.Size() == 0 {
				t.Fatal("failed to create cosign public key")
			}
		})
	}
}

// TestFramework_SignContainer_RSA generates an RSA keypair and signs a container image
// with the private key. The key is generated using the CreateRSAKeyPair function.
func TestFramework_SignContainer_RSA(t *testing.T) {
	if os.Getenv("COSIGN_INTEGRATION") == "" {
		t.Skip()
	}

	f := &Framework{}
	name := "testkey"
	priv, pub := f.CreateRSAKeyPair(t, name)
	defer f.Cleanup(t)
	if priv == "" || pub == "" {
		t.Fatal("failed to create RSA key pair")
	}

	privStat, err := os.Stat(fmt.Sprintf("%s.key", name))
	if err != nil || privStat.Size() == 0 {
		t.Fatal("failed to create private key")
	}
	pubStat, err := os.Stat(fmt.Sprintf("%s.pub", name))
	if err != nil || pubStat.Size() == 0 {
		t.Fatal("failed to create public key")
	}

	f.SignContainer(t, SignOptions{
		KeyName: fmt.Sprintf("%s-%s", name, ImportKeySuffix),
		Image:   "busybox",
	})
}
