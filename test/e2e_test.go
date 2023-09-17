package test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/eumel8/cosignwebhook/webhook"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
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

// TestOneContainerPubKeyEnvVar tests that deployment with a single signed container,
// with a public key provided via an environment variable, succeeds.
func TestOneContainerPubKeyEnvVar(t *testing.T) {
	// create a keypair to sign the container
	_, pub := createKeys(t, "test")
	os.Setenv("COSIGN_PASSWORD", "")
	// sign the container
	err := signContainer(t, "test")
	if err != nil {
		t.Fatalf("failed signing container: %v", err)
	}

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
					Containers: []corev1.Container{
						{
							Name:  "test-case-1",
							Image: "image_name:tag",
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

	// create clientset
	k8sClient, err := createClientSet()
	if err != nil {
		t.Fatalf("failed creating clientset: %v", err)
	}

	// create the deployment
	_, err = k8sClient.AppsV1().Deployments("test-cases").Create(context.Background(), &depl, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed creating deployment: %v", err)
	}

	// wait for the deployment to be ready
	err = waitForDeploymentReady(t, k8sClient, "test-cases", "test-case-1")
	if err != nil {
		t.Fatalf("failed waiting for deployment to be ready: %v", err)
	}
}

// waitForDeploymentReady waits for the deployment to be ready
func waitForDeploymentReady(t *testing.T, k8sClient *kubernetes.Clientset, ns, name string) error {

	// wait until the deployment is ready
	w, err := k8sClient.AppsV1().Deployments(ns).Watch(context.Background(), metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", name),
	})

	if err != nil {
		return err
	}

	for event := range w.ResultChan() {
		deployment, ok := event.Object.(*appsv1.Deployment)
		if !ok {
			continue
		}

		if deployment.Status.ReadyReplicas == 1 {
			return nil
		}
	}

	return nil
}

func signContainer(t *testing.T, priv string) error {
	args := []string{
		"sign",
		"--key", fmt.Sprintf("%s.key", priv),
		"image_name:tag",
	}
	cmd := cli.Sign()
	cmd.SetArgs(args)
	return cmd.Execute()

}

func createClientSet() (k8sClient *kubernetes.Clientset, err error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.Getenv("HOME") + "/.kube/config"
	}

	// create restconfig from kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return cs, nil
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
