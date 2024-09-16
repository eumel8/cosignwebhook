package test

import (
	"testing"

	"github.com/eumel8/cosignwebhook/test/framework"
	"github.com/eumel8/cosignwebhook/webhook"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// terminationGracePeriodSeconds is the termination grace period for the test deployments
var terminationGracePeriodSeconds int64 = 3

const (
	busyboxOne    = "k3d-registry.localhost:5000/busybox:first"
	busyboxTwo    = "k3d-registry.localhost:5000/busybox:second"
	signatureRepo = "k3d-registry.localhost:5000/sigs"
)

// testOneContainerSinglePubKeyEnvRef tests that a deployment with a single signed container,
// with a public key provided via an environment variable, succeeds.
func testOneContainerSinglePubKeyEnvRef(t *testing.T) {
	fw, err := framework.New()
	if err != nil {
		t.Fatal(err)
	}

	priv, pub := fw.CreateKeys(t, "test")
	fw.SignContainer(t, framework.SignOptions{
		KeyPath: priv.Path,
		Image:   busyboxOne,
	})

	// create a deployment with a single signed container and a public key provided via an environment variable
	depl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "one-container-env-ref",
			Namespace: "test-cases",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "one-container-env-ref"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "one-container-env-ref"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{
						{
							Name:  "one-container-env-ref",
							Image: busyboxOne,
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 60; done",
							},
							Env: []corev1.EnvVar{
								{
									Name:  webhook.CosignEnvVar,
									Value: pub.Key,
								},
							},
						},
					},
				},
			},
		},
	}

	fw.CreateDeployment(t, depl)
	fw.WaitForDeployment(t, depl)
	fw.Cleanup(t)
}

// testTwoContainersSinglePubKeyEnvRef tests that a deployment with two signed containers,
// with a public key provided via an environment variable, succeeds.
func testTwoContainersSinglePubKeyEnvRef(t *testing.T) {
	fw, err := framework.New()
	if err != nil {
		t.Fatal(err)
	}

	priv, pub := fw.CreateKeys(t, "test")
	fw.SignContainer(t, framework.SignOptions{
		KeyPath: priv.Path,
		Image:   busyboxOne,
	})
	fw.SignContainer(t, framework.SignOptions{
		KeyPath: priv.Path,
		Image:   busyboxTwo,
	})

	// create a deployment with two signed containers and a public key provided via an environment variable
	depl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "two-containers-same-pub-key-env-ref",
			Namespace: "test-cases",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "two-containers-same-pub-key-env-ref"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "two-containers-same-pub-key-env-ref"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{
						{
							Name:  "two-containers-same-pub-key-env-ref-first",
							Image: busyboxOne,
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 60; done",
							},
							Env: []corev1.EnvVar{
								{
									Name:  webhook.CosignEnvVar,
									Value: pub.Key,
								},
							},
						},
						{
							Name:  "two-containers-same-pub-key-env-ref-second",
							Image: busyboxTwo,
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 60; done",
							},
							Env: []corev1.EnvVar{
								{
									Name:  webhook.CosignEnvVar,
									Value: pub.Key,
								},
							},
						},
					},
				},
			},
		},
	}

	fw.CreateDeployment(t, depl)
	fw.WaitForDeployment(t, depl)
	fw.Cleanup(t)
}

// testOneContainerPubKeySecret tests that a deployment with a single signed container,
// with a public key provided via a secret, succeeds.
func testOneContainerSinglePubKeySecretRef(t *testing.T) {
	fw, err := framework.New()
	if err != nil {
		t.Fatal(err)
	}

	priv, pub := fw.CreateKeys(t, "test")
	fw.SignContainer(t, framework.SignOptions{
		KeyPath: priv.Path,
		Image:   busyboxOne,
	})

	// create a secret with the public key
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "one-container-secret-ref",
			Namespace: "test-cases",
		},
		StringData: map[string]string{
			"cosign.pub": pub.Key,
		},
	}

	// create a deployment with a single signed container and a public key provided via a secret
	depl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "one-container-secret-ref",
			Namespace: "test-cases",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "one-container-secret-ref"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "one-container-secret-ref"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{
						{
							Name:  "one-container-secret-ref",
							Image: busyboxOne,
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 60; done",
							},
							Env: []corev1.EnvVar{
								{
									Name: webhook.CosignEnvVar,
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											Key: "cosign.pub",
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "one-container-secret-ref",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	fw.CreateSecret(t, secret)
	fw.CreateDeployment(t, depl)
	fw.WaitForDeployment(t, depl)
	fw.Cleanup(t)
}

// testTwoContainersMixedPubKeyMixedRef tests that a deployment with two signed containers with two different public keys,
// with the keys provided by a secret and an environment variable, succeeds.
func testTwoContainersMixedPubKeyMixedRef(t *testing.T) {
	fw, err := framework.New()
	if err != nil {
		t.Fatal(err)
	}

	priv1, pub1 := fw.CreateKeys(t, "test1")
	priv2, pub2 := fw.CreateKeys(t, "test2")
	fw.SignContainer(t, framework.SignOptions{
		KeyPath: priv1.Path,
		Image:   busyboxOne,
	})
	fw.SignContainer(t, framework.SignOptions{
		KeyPath: priv2.Path,
		Image:   busyboxTwo,
	})

	// create a secret with the public key
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "two-containers-mixed-pub-keyrefs",
			Namespace: "test-cases",
		},
		StringData: map[string]string{
			"cosign.pub": pub1.Key,
		},
	}

	// create a deployment with two signed containers and a public key provided via a secret
	depl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "two-containers-mixed-pub-keyrefs",
			Namespace: "test-cases",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "two-containers-mixed-pub-keyrefs"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "two-containers-mixed-pub-keyrefs"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{
						{
							Name:  "two-containers-mixed-pub-keyrefs-from-secret",
							Image: busyboxOne,
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 60; done",
							},
							Env: []corev1.EnvVar{
								{
									Name: webhook.CosignEnvVar,
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											Key: "cosign.pub",
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "two-containers-mixed-pub-keyrefs",
											},
										},
									},
								},
							},
						},
						{
							Name:  "two-containers-mixed-pub-keyrefs-second-from-env",
							Image: busyboxTwo,
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 60; done",
							},
							Env: []corev1.EnvVar{
								{
									Name:  webhook.CosignEnvVar,
									Value: pub2.Key,
								},
							},
						},
					},
				},
			},
		},
	}

	fw.CreateSecret(t, secret)
	fw.CreateDeployment(t, depl)
	fw.WaitForDeployment(t, depl)
	fw.Cleanup(t)
}

// testTwoContainersSinglePubKeyMixedRef tests that a deployment with two signed containers,
// with a public key provided via a secret and an environment variable, succeeds.
func testTwoContainersSinglePubKeyMixedRef(t *testing.T) {
	fw, err := framework.New()
	if err != nil {
		t.Fatal(err)
	}

	priv, pub := fw.CreateKeys(t, "test")
	fw.SignContainer(t, framework.SignOptions{
		KeyPath: priv.Path,
		Image:   busyboxOne,
	})
	fw.SignContainer(t, framework.SignOptions{
		KeyPath: priv.Path,
		Image:   busyboxTwo,
	})

	// create a secret with the public key
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "two-containers-onekey-mixed-ref",
			Namespace: "test-cases",
		},
		StringData: map[string]string{
			"cosign.pub": pub.Key,
		},
	}

	// create a deployment with two signed containers and a public key provided via a secret
	depl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "two-containers-onekey-mixed-ref",
			Namespace: "test-cases",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "two-containers-onekey-mixed-ref"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "two-containers-onekey-mixed-ref"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{
						{
							Name:  "two-containers-onekey-mixed-ref-first",
							Image: busyboxOne,
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 60; done",
							},
							Env: []corev1.EnvVar{
								{
									Name: webhook.CosignEnvVar,
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											Key: "cosign.pub",
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "two-containers-onekey-mixed-ref",
											},
										},
									},
								},
							},
						},
						{
							Name:  "two-containers-onekey-mixed-ref-second",
							Image: busyboxTwo,
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 60; done",
							},
							Env: []corev1.EnvVar{
								{
									Name:  webhook.CosignEnvVar,
									Value: pub.Key,
								},
							},
						},
					},
				},
			},
		},
	}

	fw.CreateSecret(t, secret)
	fw.CreateDeployment(t, depl)
	fw.WaitForDeployment(t, depl)
	fw.Cleanup(t)
}

// testTwoContainersSinglePubKeyMixedRef tests that a deployment with two signed containers,
// with a public key provided via a secret and an environment variable, succeeds.
func testTwoContainersWithInitSinglePubKeyMixedRef(t *testing.T) {
	fw, err := framework.New()
	if err != nil {
		t.Fatal(err)
	}

	priv, pub := fw.CreateKeys(t, "test")
	fw.SignContainer(t, framework.SignOptions{
		KeyPath: priv.Path,
		Image:   busyboxOne,
	})
	fw.SignContainer(t, framework.SignOptions{
		KeyPath: priv.Path,
		Image:   busyboxTwo,
	})

	// create a secret with the public key
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "two-containers-init-singlekey-mixed-ref",
			Namespace: "test-cases",
		},
		StringData: map[string]string{
			"cosign.pub": pub.Key,
		},
	}

	// create a deployment with two signed containers and a public key provided via a secret
	depl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "two-containers-init-singlekey-mixed-ref",
			Namespace: "test-cases",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "two-containers-init-singlekey-mixed-ref"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "two-containers-init-singlekey-mixed-ref"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					InitContainers: []corev1.Container{
						{
							Name:  "two-containers-init-singlekey-mixed-ref-first",
							Image: busyboxOne,
							Command: []string{
								"sh",
								"-c",
								"echo 'hello world, i am tired and will sleep now, for a bit...';",
							},
							Env: []corev1.EnvVar{
								{
									Name: webhook.CosignEnvVar,
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											Key: "cosign.pub",
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "two-containers-init-singlekey-mixed-ref",
											},
										},
									},
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "two-containers-init-singlekey-mixed-ref-second",
							Image: busyboxTwo,
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 60; done",
							},
							Env: []corev1.EnvVar{
								{
									Name:  webhook.CosignEnvVar,
									Value: pub.Key,
								},
							},
						},
					},
				},
			},
		},
	}

	fw.CreateSecret(t, secret)
	fw.CreateDeployment(t, depl)
	fw.WaitForDeployment(t, depl)
	fw.Cleanup(t)
}

// testEventEmittedOnSignatureVerification tests
// that an event is emitted when a deployment passes signature verification
func testEventEmittedOnSignatureVerification(t *testing.T) {
	fw, err := framework.New()
	if err != nil {
		t.Fatal(err)
	}

	priv, pub := fw.CreateKeys(t, "test")
	fw.SignContainer(t, framework.SignOptions{
		KeyPath: priv.Path,
		Image:   busyboxOne,
	})

	// create a deployment with a single signed container and a public key provided via an environment variable
	depl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "event-emitted-on-verify",
			Namespace: "test-cases",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "event-emitted-on-verify"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "event-emitted-on-verify"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{
						{
							Name:  "event-emitted-on-verify",
							Image: busyboxOne,
							Command: []string{
								"sh",
								"-c",
								"echo 'hello world, i am tired and will sleep now, for a bit...'; sleep 60",
							},
							Env: []corev1.EnvVar{
								{
									Name:  webhook.CosignEnvVar,
									Value: pub.Key,
								},
							},
						},
					},
				},
			},
		},
	}

	fw.CreateDeployment(t, depl)
	fw.WaitForDeployment(t, depl)
	pod := fw.GetPods(t, depl)
	fw.AssertEventForPod(t, "PodVerified", pod.Items[0])
	fw.Cleanup(t)
}

func testEventEmittedOnNoSignatureVerification(t *testing.T) {
	fw, err := framework.New()
	if err != nil {
		t.Fatal(err)
	}

	// create a deployment with a single unsigned container
	depl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "event-emitted-on-no-verify-needed",
			Namespace: "test-cases",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "event-emitted-on-no-verify-needed"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "event-emitted-on-no-verify-needed"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{
						{
							Name:    "event-emitted-on-no-verify-needed",
							Image:   busyboxOne,
							Command: []string{"sh", "-c", "echo 'hello world, i am tired and will sleep now, for a bit...'; sleep 60"},
						},
					},
				},
			},
		},
	}

	fw.CreateDeployment(t, depl)
	fw.WaitForDeployment(t, depl)
	pl := fw.GetPods(t, depl)
	fw.AssertEventForPod(t, "NoVerification", pl.Items[0])
	fw.Cleanup(t)
}

// testOneContainerWithCosignRepository tests that a deployment with a single signed container,
// with a public key provided via a secret succeeds.
// The signature for the container is present in the repository
// defined in the environment variables of the container.
func testOneContainerWithCosignRepository(t *testing.T) {
	fw, err := framework.New()
	if err != nil {
		t.Fatal(err)
	}

	priv, pub := fw.CreateKeys(t, "test")
	fw.SignContainer(t, framework.SignOptions{
		KeyPath:       priv.Path,
		Image:         busyboxOne,
		SignatureRepo: signatureRepo,
	})

	// create a secret with the public key
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "one-container-cosign-repo",
			Namespace: "test-cases",
		},
		StringData: map[string]string{
			"cosign.pub": pub.Key,
		},
	}

	// create a deployment with a single signed container and a public key provided via a secret
	depl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "one-container-cosign-repo",
			Namespace: "test-cases",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "one-container-cosign-repo"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "one-container-cosign-repo"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{
						{
							Name:  "one-container-cosign-repo",
							Image: busyboxOne,
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 60; done",
							},
							Env: []corev1.EnvVar{
								{
									Name: webhook.CosignEnvVar,
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											Key: "cosign.pub",
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "one-container-cosign-repo",
											},
										},
									},
								},
								{
									Name:  webhook.CosignRepositoryEnvVar,
									Value: signatureRepo,
								},
							},
						},
					},
				},
			},
		},
	}

	fw.CreateSecret(t, secret)
	fw.CreateDeployment(t, depl)
	fw.WaitForDeployment(t, depl)
	fw.Cleanup(t)
}

// testOneContainerSinglePubKeyEnvRefRSA tests that a deployment with a single signed container,
// with a public key provided via an environment variable, succeeds. The keypair used for this test is an RSA keypair.
func testOneContainerSinglePubKeyEnvRefRSA(t *testing.T) {
	fw, err := framework.New()
	if err != nil {
		t.Fatal(err)
	}

	priv, pub := fw.CreateRSAKeyPair(t, "test")
	fw.SignContainer(t, framework.SignOptions{
		KeyPath: priv.Path,
		Image:   busyboxOne,
	})

	// create a deployment with a single signed container and a public key provided via an environment variable
	depl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "one-container-env-ref-rsa",
			Namespace: "test-cases",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "one-container-env-ref-rsa"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "one-container-env-ref-rsa"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{
						{
							Name:  "one-container-env-ref-rsa",
							Image: busyboxOne,
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 60; done",
							},
							Env: []corev1.EnvVar{
								{
									Name:  webhook.CosignEnvVar,
									Value: pub.Key,
								},
							},
						},
					},
				},
			},
		},
	}

	fw.CreateDeployment(t, depl)
	fw.WaitForDeployment(t, depl)
	fw.Cleanup(t)
}

func TestTwoContainersSinglePubKeyEnvRefRSA(t *testing.T) {
	fw, err := framework.New()
	if err != nil {
		t.Fatal(err)
	}

	// Create a deployment with two containers signed by the same RSA key
	priv, pub := fw.CreateRSAKeyPair(t, "test")
	fw.SignContainer(t, framework.SignOptions{
		KeyPath: priv.Path,
		Image:   busyboxOne,
	})
	fw.SignContainer(t, framework.SignOptions{
		KeyPath: priv.Path,
		Image:   busyboxTwo,
	})

	depl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "two-containers-single-pubkey-envref",
			Namespace: "test-cases",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "two-containers-single-pubkey-envref"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "two-containers-single-pubkey-envref"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{
						{
							Name:  "two-containers-same-rsa-pub-key-env-ref-first",
							Image: busyboxOne,
							Command: []string{
								"sh", "-c",
								"echo 'hello world, i am tired and will sleep now'; sleep 60",
							},
							Env: []corev1.EnvVar{
								{
									Name:  webhook.CosignEnvVar,
									Value: pub.Key,
								},
							},
						},
						{
							Name:  "two-containers-same-rsa-pub-key-env-ref-second",
							Image: busyboxTwo,
							Command: []string{
								"sh", "-c",
								"echo 'hello world, i am tired and will sleep now'; sleep 60",
							},
							Env: []corev1.EnvVar{
								{
									Name:  webhook.CosignEnvVar,
									Value: pub.Key,
								},
							},
						},
					},
				},
			},
		},
	}

	fw.CreateDeployment(t, depl)
	fw.WaitForDeployment(t, depl)
	fw.Cleanup(t)
}

// testOneContainerSinglePubKeyNoMatchEnvRef tests that a deployment with a single signed container,
// with a public key provided via an environment variable, fails if the public key does not match the signature.
func testOneContainerSinglePubKeyNoMatchEnvRef(t *testing.T) {
	fw, err := framework.New()
	if err != nil {
		t.Fatal(err)
	}

	priv, _ := fw.CreateKeys(t, "test")
	_, otherPub := fw.CreateKeys(t, "other")
	fw.SignContainer(t, framework.SignOptions{
		KeyPath: priv.Path,
		Image:   busyboxOne,
	})

	// create a deployment with a single signed container and a public key provided via an environment variable
	depl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "no-match-env-ref",
			Namespace: "test-cases",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "no-match-env-ref"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "no-match-env-ref"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{
						{
							Name:  "no-match-env-ref",
							Image: busyboxOne,
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 60; done",
							},
							Env: []corev1.EnvVar{
								{
									Name:  webhook.CosignEnvVar,
									Value: otherPub.Key,
								},
							},
						},
					},
				},
			},
		},
	}

	fw.CreateDeployment(t, depl)
	fw.AssertDeploymentFailed(t, depl)
	fw.Cleanup(t)
}

// testTwoContainersSinglePubKeyNoMatchEnvRef tests that a deployment with two signed containers,
// with a public key provided via an environment variable, fails if one of the containers public key is malformed.
func testTwoContainersSinglePubKeyMalformedEnvRef(t *testing.T) {
	fw, err := framework.New()
	if err != nil {
		t.Fatal(err)
	}

	priv, pub := fw.CreateKeys(t, "test")
	fw.SignContainer(t, framework.SignOptions{
		KeyPath: priv.Path,
		Image:   busyboxOne,
	})

	// create a deployment with two signed containers and a public key provided via an environment variable
	depl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "malformed-env-ref",
			Namespace: "test-cases",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "malformed-env-ref"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "malformed-env-ref"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{
						{
							Name:  "malformed-env-ref-first",
							Image: busyboxOne,
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 60; done",
							},
							Env: []corev1.EnvVar{
								{
									Name:  webhook.CosignEnvVar,
									Value: pub.Key,
								},
							},
						},
						{
							Name:  "malformed-env-ref-second",
							Image: busyboxTwo,
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 60; done",
							},
							Env: []corev1.EnvVar{
								{
									Name:  webhook.CosignEnvVar,
									Value: "not-a-public-key",
								},
							},
						},
					},
				},
			},
		},
	}

	fw.CreateDeployment(t, depl)
	fw.AssertDeploymentFailed(t, depl)
	fw.Cleanup(t)
}

// testOneContainerSinglePubKeyMalformedEnvRef tests that a deployment with a single signed container,
// // with a public key provided via an environment variable, fails if the public key has an incorrect format.
func testOneContainerSinglePubKeyMalformedEnvRef(t *testing.T) {
	fw, err := framework.New()
	if err != nil {
		t.Fatal(err)
	}

	depl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "single-malformed-env-ref",
			Namespace: "test-cases",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "single-malformed-env-ref"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "single-malformed-env-ref"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{
						{
							Name:  "single-malformed-env-ref",
							Image: busyboxOne,
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 60; done",
							},
							Env: []corev1.EnvVar{
								{
									Name:  webhook.CosignEnvVar,
									Value: "not-a-public-key",
								},
							},
						},
					},
				},
			},
		},
	}

	fw.CreateDeployment(t, depl)
	fw.AssertDeploymentFailed(t, depl)
	fw.Cleanup(t)
}

// testOneContainerSinglePubKeyNoMatchSecretRef tests that a deployment with a single signed container,
// with a public key provided via a secret, fails if the public key does not match the signature, which
// is uploaded in a different repository as the image itself
func testOneContainerWithCosingRepoVariableMissing(t *testing.T) {
	fw, err := framework.New()
	if err != nil {
		t.Fatal(err)
	}

	priv, pub := fw.CreateKeys(t, "test")
	fw.SignContainer(t, framework.SignOptions{
		KeyPath:       priv.Path,
		Image:         busyboxOne,
		SignatureRepo: signatureRepo,
	})

	depl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "one-container-with-cosign-repo-missing",
			Namespace: "test-cases",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "one-container-with-cosign-repo-missing"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "one-container-with-cosign-repo-missing"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{
						{
							Name:  "one-container-with-cosign-repo-missing",
							Image: busyboxOne,
							Command: []string{
								"sh", "-c",
								"echo 'hello world, i can't start because I'm missing an env var...'; sleep 60",
							},
							Env: []corev1.EnvVar{
								{
									Name:  webhook.CosignEnvVar,
									Value: pub.Key,
								},
							},
						},
					},
				},
			},
		},
	}

	fw.CreateDeployment(t, depl)
	fw.AssertDeploymentFailed(t, depl)
	fw.Cleanup(t)
}
