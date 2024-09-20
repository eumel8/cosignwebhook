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
			f := &Framework{
				t: t,
			}
			defer f.Cleanup()
			private, public := CreateRSAKeyPair(f, tt.name)

			if private.Key == "" || public.Key == "" {
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

			coPrivStat, err := os.Stat(fmt.Sprintf("%s-%s.key", tt.name, ImportKeySuffix))

			if err != nil || coPrivStat.Size() == 0 {
				t.Fatal("failed to create cosign private key")
			}
			coPubStat, err := os.Stat(fmt.Sprintf("%s-%s.pub", tt.name, ImportKeySuffix))

			if err != nil || coPubStat.Size() == 0 {
				t.Fatal("failed to create cosign public key")
			}

			// pub keys should be the same
			pubBytes, err := os.ReadFile(fmt.Sprintf("%s.pub", tt.name))
			if err != nil {
				t.Fatal(err)
			}
			coPubBytes, err := os.ReadFile(fmt.Sprintf("%s-%s.pub", tt.name, ImportKeySuffix))
			if err != nil {
				t.Fatal(err)
			}
			if string(pubBytes) != string(coPubBytes) {
				t.Fatal("public keys do not match. expected: ", string(pubBytes), " got: ", string(coPubBytes))
			}
		})
	}
}

// TestFramework_SignContainer_RSA generates an RSA keypair and signs a container image
// with the private key. The key is generated using the CreateRSAKeyPair function.
func TestFramework_SignContainer_RSA(t *testing.T) {
	if os.Getenv("COSIGN_E2E") == "" {
		t.Skip()
	}

	f := &Framework{
		t: t,
	}
	defer f.Cleanup()
	name := "testkey"
	private, public := CreateRSAKeyPair(f, name)
	if private.Key == "" || public.Key == "" {
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

	f.SignContainer(SignOptions{
		KeyPath: fmt.Sprintf("%s-%s.key", name, ImportKeySuffix),
		Image:   "k3d-registry.localhost:5000/busybox:first",
	})
}
