package test

import (
	"github.com/eumel8/cosignwebhook/test/framework"
	"testing"

	"github.com/eumel8/cosignwebhook/webhook"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// terminationGracePeriodSeconds is the termination grace period for the test deployments
var terminationGracePeriodSeconds int64 = 3

// testOneContainerSinglePubKeyEnvRef tests that a deployment with a single signed container,
// with a public key provided via an environment variable, succeeds.
func testOneContainerSinglePubKeyEnvRef(t *testing.T) {
	fw, err := framework.New()
	if err != nil {
		t.Fatal(err)
	}

	_, pub := fw.CreateKeys(t, "test")
	fw.SignContainer(t, "test", "k3d-registry.localhost:5000/busybox:first")

	// create a deployment with a single signed container and a public key provided via an environment variable
	depl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-case-1",
			Namespace: "test-cases",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-case-1"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-case-1"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{
						{
							Name:  "test-case-1",
							Image: "k3d-registry.localhost:5000/busybox:latest",
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 10; done",
							},
							Env: []corev1.EnvVar{
								{
									Name:  webhook.CosignEnvVar,
									Value: pub,
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
	fw.Cleanup(t, nil)
}

// testTwoContainersSinglePubKeyEnvRef tests that a deployment with two signed containers,
// with a public key provided via an environment variable, succeeds.
func testTwoContainersSinglePubKeyEnvRef(t *testing.T) {

	fw, err := framework.New()
	if err != nil {
		t.Fatal(err)
	}

	_, pub := fw.CreateKeys(t, "test")
	fw.SignContainer(t, "test", "k3d-registry.localhost:5000/busybox:first")
	fw.SignContainer(t, "test", "k3d-registry.localhost:5000/busybox:second")

	// create a deployment with two signed containers and a public key provided via an environment variable
	depl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-case-2",
			Namespace: "test-cases",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-case-2"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-case-2"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{
						{
							Name:  "test-case-2-first",
							Image: "k3d-registry.localhost:5000/busybox:first",
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 10; done",
							},
							Env: []corev1.EnvVar{
								{
									Name:  webhook.CosignEnvVar,
									Value: pub,
								},
							},
						},
						{
							Name:  "test-case-2-second",
							Image: "k3d-registry.localhost:5000/busybox:second",
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 10; done",
							},
							Env: []corev1.EnvVar{
								{
									Name:  webhook.CosignEnvVar,
									Value: pub,
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
	fw.Cleanup(t, nil)
}

// testOneContainerPubKeySecret tests that a deployment with a single signed container,
// with a public key provided via a secret, succeeds.
func testOneContainerSinglePubKeySecretRef(t *testing.T) {

	fw, err := framework.New()
	if err != nil {
		t.Fatal(err)
	}

	_, pub := fw.CreateKeys(t, "test")
	fw.SignContainer(t, "test", "k3d-registry.localhost:5000/busybox:first")

	// create a secret with the public key
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-case-3",
			Namespace: "test-cases",
		},
		StringData: map[string]string{
			"cosign.pub": pub,
		},
	}

	// create a deployment with a single signed container and a public key provided via a secret
	depl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-case-3",
			Namespace: "test-cases",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-case-3"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-case-3"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{
						{
							Name:  "test-case-3",
							Image: "k3d-registry.localhost:5000/busybox:latest",
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 10; done",
							},
							Env: []corev1.EnvVar{
								{
									Name: webhook.CosignEnvVar,
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											Key: "cosign.pub",
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "test-case-3",
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
	fw.Cleanup(t, nil)
}

// testTwoContainersMixedPubKeyMixedRef tests that a deployment with two signed containers with two different public keys,
// with the keys provided by a secret and an environment variable, succeeds.
func testTwoContainersMixedPubKeyMixedRef(t *testing.T) {
	fw, err := framework.New()
	if err != nil {
		t.Fatal(err)
	}

	_, pub1 := fw.CreateKeys(t, "test1")
	_, pub2 := fw.CreateKeys(t, "test2")
	fw.SignContainer(t, "test1", "k3d-registry.localhost:5000/busybox:first")
	fw.SignContainer(t, "test2", "k3d-registry.localhost:5000/busybox:second")

	// create a secret with the public key
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-case-4",
			Namespace: "test-cases",
		},
		StringData: map[string]string{
			"cosign.pub": pub1,
		},
	}

	// create a deployment with two signed containers and a public key provided via a secret
	depl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-case-4",
			Namespace: "test-cases",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-case-4"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-case-4"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{
						{
							Name:  "test-case-4-first",
							Image: "k3d-registry.localhost:5000/busybox:first",
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 10; done",
							},
							Env: []corev1.EnvVar{
								{
									Name: webhook.CosignEnvVar,
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											Key: "cosign.pub",
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "test-case-4",
											},
										},
									},
								},
							},
						},
						{
							Name:  "test-case-4-second",
							Image: "k3d-registry.localhost:5000/busybox:second",
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 10; done",
							},
							Env: []corev1.EnvVar{
								{
									Name:  webhook.CosignEnvVar,
									Value: pub2,
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
	fw.Cleanup(t, nil)

}

// testTwoContainersSinglePubKeyMixedRef tests that a deployment with two signed containers,
// with a public key provided via a secret and an environment variable, succeeds.
func testTwoContainersSinglePubKeyMixedRef(t *testing.T) {

	fw, err := framework.New()
	if err != nil {
		t.Fatal(err)
	}

	_, pub := fw.CreateKeys(t, "test")
	fw.SignContainer(t, "test", "k3d-registry.localhost:5000/busybox:first")
	fw.SignContainer(t, "test", "k3d-registry.localhost:5000/busybox:second")

	// create a secret with the public key
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-case-5",
			Namespace: "test-cases",
		},
		StringData: map[string]string{
			"cosign.pub": pub,
		},
	}

	// create a deployment with two signed containers and a public key provided via a secret
	depl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-case-5",
			Namespace: "test-cases",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-case-5"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-case-5"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{
						{
							Name:  "test-case-5-first",
							Image: "k3d-registry.localhost:5000/busybox:first",
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 10; done",
							},
							Env: []corev1.EnvVar{
								{
									Name: webhook.CosignEnvVar,
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											Key: "cosign.pub",
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "test-case-5",
											},
										},
									},
								},
							},
						},
						{
							Name:  "test-case-5-second",
							Image: "k3d-registry.localhost:5000/busybox:second",
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 10; done",
							},
							Env: []corev1.EnvVar{
								{
									Name:  webhook.CosignEnvVar,
									Value: pub,
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
	fw.Cleanup(t, nil)

}

// testTwoContainersSinglePubKeyMixedRef tests that a deployment with two signed containers,
// with a public key provided via a secret and an environment variable, succeeds.
func testTwoContainersWithInitSinglePubKeyMixedRef(t *testing.T) {

	fw, err := framework.New()
	if err != nil {
		t.Fatal(err)
	}

	_, pub := fw.CreateKeys(t, "test")
	fw.SignContainer(t, "test", "k3d-registry.localhost:5000/busybox:first")
	fw.SignContainer(t, "test", "k3d-registry.localhost:5000/busybox:second")

	// create a secret with the public key
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-case-6",
			Namespace: "test-cases",
		},
		StringData: map[string]string{
			"cosign.pub": pub,
		},
	}

	// create a deployment with two signed containers and a public key provided via a secret
	depl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-case-6",
			Namespace: "test-cases",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-case-6"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-case-6"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					InitContainers: []corev1.Container{
						{
							Name:  "test-case-6-first",
							Image: "k3d-registry.localhost:5000/busybox:first",
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
												Name: "test-case-6",
											},
										},
									},
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "test-case-6-second",
							Image: "k3d-registry.localhost:5000/busybox:second",
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 10; done",
							},
							Env: []corev1.EnvVar{
								{
									Name:  webhook.CosignEnvVar,
									Value: pub,
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
	fw.Cleanup(t, nil)
}

// testOneContainerSinglePubKeyNoMatchEnvRef tests that a deployment with a single signed container,
// with a public key provided via an environment variable, fails if the public key does not match the signature.
func testOneContainerSinglePubKeyNoMatchEnvRef(t *testing.T) {

	fw, err := framework.New()
	if err != nil {
		t.Fatal(err)
	}

	_, _ = fw.CreateKeys(t, "test")
	_, other := fw.CreateKeys(t, "other")
	fw.SignContainer(t, "test", "k3d-registry.localhost:5000/busybox:first")

	// create a deployment with a single signed container and a public key provided via an environment variable
	depl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fail-case-1",
			Namespace: "test-cases",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "fail-case-1"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "fail-case-1"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{
						{
							Name:  "fail-case-1",
							Image: "k3d-registry.localhost:5000/busybox:first",
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 10; done",
							},
							Env: []corev1.EnvVar{
								{
									Name:  webhook.CosignEnvVar,
									Value: other,
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
	fw.Cleanup(t, nil)

}

// testTwoContainersSinglePubKeyNoMatchEnvRef tests that a deployment with two signed containers,
// with a public key provided via an environment variable, fails if one of the container's pub key is malformed.
func testTwoContainersSinglePubKeyMalformedEnvRef(t *testing.T) {

	fw, err := framework.New()
	if err != nil {
		t.Fatal(err)
	}

	_, pub := fw.CreateKeys(t, "test")
	fw.SignContainer(t, "test", "k3d-registry.localhost:5000/busybox:first")

	// create a deployment with two signed containers and a public key provided via an environment variable
	depl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fail-case-2",
			Namespace: "test-cases",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "fail-case-2"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "fail-case-2"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{
						{
							Name:  "fail-case-2-first",
							Image: "k3d-registry.localhost:5000/busybox:first",
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 10; done",
							},
							Env: []corev1.EnvVar{
								{
									Name:  webhook.CosignEnvVar,
									Value: pub,
								},
							},
						},
						{
							Name:  "fail-case-2-second",
							Image: "k3d-registry.localhost:5000/busybox:second",
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 10; done",
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
	fw.Cleanup(t, nil)

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
			Name:      "fail-case-3",
			Namespace: "test-cases",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "fail-case-3"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "fail-case-3"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{
						{
							Name:  "fail-case-3",
							Image: "k3d-registry.localhost:5000/busybox:latest",
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello world, i am tired and will sleep now'; sleep 10; done",
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
	fw.Cleanup(t, nil)

}
