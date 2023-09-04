package main

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

func Test_getPubKeyFromEnv(t *testing.T) {

	tests := []struct {
		name          string
		pod           *corev1.Pod
		secretPresent bool
		want          string
		wantErr       bool
	}{
		{
			name: "public key from environment variable",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Env: []corev1.EnvVar{
								{
									Name:  cosignEnvVar,
									Value: "secret",
								},
							},
						},
					},
				},
			},
			want:          "secret",
			secretPresent: false,
			wantErr:       false,
		},
		{
			name: "public key from referenced secret",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Env: []corev1.EnvVar{
								{
									Name: cosignEnvVar,
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											Key: cosignEnvVar,
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "cosign-pubkey",
											},
										},
									},
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
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Env: []corev1.EnvVar{
								{
									Name: cosignEnvVar,
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
					},
				},
			},
			secretPresent: true,
			want:          "",
			wantErr:       false,
		},
		{
			name: "public key from referenced non-existing secret",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Env: []corev1.EnvVar{
								{
									Name: cosignEnvVar,
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											Key: cosignEnvVar,
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "non-existing-secret",
											},
										},
									},
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
						cosignEnvVar: []byte(tt.want),
					},
				}
				c = fake.NewSimpleClientset(secret)
			}

			chs := &CosignServerHandler{
				cs: c,
			}

			got, err := chs.getPubKeyFromEnv(tt.pod)
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
