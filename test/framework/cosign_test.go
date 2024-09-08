package framework

import (
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

			if priv == "" || pub == "" {
				t.Fatal("failed to create RSA key pair")
			}

			privStat, err := os.Stat(tt.name + ".key")
			if err != nil || privStat.Size() == 0 {
				t.Fatal("failed to create private key")
			}
			pubStat, err := os.Stat(tt.name + ".pub")
			if err != nil || pubStat.Size() == 0 {
				t.Fatal("failed to create public key")
			}

			_ = os.Remove(tt.name + ".key")
			_ = os.Remove(tt.name + ".pub")
		})
	}
}
