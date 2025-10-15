package webhook

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_getPubKeyFromEnv(t *testing.T) {
	tests := []struct {
		name          string
		container     *corev1.Container
		secretPresent bool
		want          string
		wantErr       bool
	}{
		{
			name: "public key from environment variable",
			container: &corev1.Container{
				Env: []corev1.EnvVar{
					{
						Name:  CosignEnvVar,
						Value: "secret",
					},
				},
			},
			want:          "secret",
			secretPresent: false,
			wantErr:       false,
		},
		{
			name: "public key from referenced secret",
			container: &corev1.Container{
				Env: []corev1.EnvVar{
					{
						Name: CosignEnvVar,
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								Key: CosignEnvVar,
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "cosign-pubkey",
								},
							},
						},
					},
				},
			},
			secretPresent: true,
			want:          "secret",
			wantErr:       false,
		},
		{
			name: "public key from referenced secret with wrong key",
			container: &corev1.Container{
				Env: []corev1.EnvVar{
					{
						Name: CosignEnvVar,
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								Key: "wrong-key",
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "cosign-pubkey",
								},
							},
						},
					},
				},
			},
			secretPresent: true,
			want:          "",
			wantErr:       false,
		},
		{
			name: "public key from referenced non-existing secret",
			container: &corev1.Container{
				Env: []corev1.EnvVar{
					{
						Name: CosignEnvVar,
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								Key: CosignEnvVar,
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "non-existing-secret",
								},
							},
						},
					},
				},
			},
			secretPresent: false,
			want:          "",
			wantErr:       true,
		},
		{
			name: "cosign key without value",
			container: &corev1.Container{
				Env: []corev1.EnvVar{
					{
						Name: CosignEnvVar,
					},
				},
			},
			secretPresent: false,
			want:          "",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewSimpleClientset()
			if tt.secretPresent {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cosign-pubkey",
						Namespace: "test",
					},
					Data: map[string][]byte{
						CosignEnvVar: []byte(tt.want),
					},
				}
				c = fake.NewSimpleClientset(secret)
			}

			chs := &CosignServerHandler{
				cs: c,
			}

			got, err := chs.getPubKeyFromEnv(tt.container, "test")
			if (err != nil) != tt.wantErr {
				t.Errorf("getPubKeyFromEnv() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getPubKeyFromEnv() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCosignServerHandler_newVerifierForKey(t *testing.T) {
	tests := []struct {
		name    string
		pubkey  crypto.PublicKey
		wantErr bool
	}{
		{
			name:   "success RSA",
			pubkey: testRSAPubKey(t),
		},
		{
			name:   "success ECDSA",
			pubkey: testECDSAPubKey(t),
		},
		{
			name:    "fail empty public key",
			pubkey:  "",
			wantErr: true,
		},
		{
			name:    "fail: malformed key",
			pubkey:  "i'm not a key!",
			wantErr: true,
		},
	}

	for _, tt := range tests {

		csh := &CosignServerHandler{}
		t.Run(tt.name, func(t *testing.T) {
			got, err := csh.newVerifierForKey(tt.pubkey)

			if (err != nil) != tt.wantErr {
				t.Fatalf("verifySignature() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && got == nil {
				t.Fatal("expected key to produce verifier")
			}
		})
	}
}

// testECDSAPubKey creates an ECDSA keypair and returns the public key
func testECDSAPubKey(t testing.TB) crypto.PublicKey {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Errorf("failed generating ECDSA key: %v", err)
		return nil
	}
	return &key.PublicKey
}

// testRSAPubKey creates an RSA keypair and returns the public key
func testRSAPubKey(t testing.TB) crypto.PublicKey {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Errorf("failed generating RSA key: %v", err)
		return nil
	}

	return &key.PublicKey
}
