package framework

import (
	"context"
	"fmt"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"regexp"
	"testing"
	"time"
)

// Framework is a helper struct for testing
// the cosignwebhook in a k8s cluster
type Framework struct {
	k8s *kubernetes.Clientset
}

func New() (*Framework, error) {
	k8s, err := createClientSet()
	if err != nil {
		return nil, err
	}

	return &Framework{
		k8s: k8s,
	}, nil
}

// CreateDeployment creates a deployment in the testing namespace
func (f *Framework) CreateDeployment(t testing.TB, d appsv1.Deployment) {
	_, err := f.k8s.AppsV1().Deployments("test-cases").Create(context.Background(), &d, metav1.CreateOptions{})
	if err != nil {
		f.Cleanup(t, err)
	}
}

// WaitForDeployment waits until the deployment is ready
func (f *Framework) WaitForDeployment(t *testing.T, ns, name string) {

	t.Logf("waiting for deployment %s to be ready", name)
	// wait until the deployment is ready
	w, err := f.k8s.AppsV1().Deployments(ns).Watch(context.Background(), metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", name),
	})

	if err != nil {
		f.Cleanup(t, err)
	}
	for event := range w.ResultChan() {
		deployment, ok := event.Object.(*appsv1.Deployment)
		if !ok {
			continue
		}

		if deployment.Status.ReadyReplicas == 1 {
			t.Logf("deployment %s is ready", name)
			return
		}
	}

	t.Fatalf("deployment %s is not ready", name)
}

// Cleanup removes all resources created by the framework
// and cleans up the testing directory
func (f *Framework) Cleanup(t testing.TB, err error) {
	cleanupKeys(t)
	f.cleanupDeployments(t)
	f.cleanupSecrets(t)
	if err != nil {
		t.Fatalf("test failed: %v", err)
	}
}

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
func (f *Framework) CreateKeys(t testing.TB, name string) (string, string) {
	args := []string{fmt.Sprintf("--output-key-prefix=%s", name)}
	err := os.Setenv("COSIGN_PASSWORD", "")
	if err != nil {
		t.Fatalf("failed setting COSIGN_PASSWORD: %v", err)
	}
	cmd := cli.GenerateKeyPair()
	cmd.SetArgs(args)
	err = cmd.Execute()
	if err != nil {
		f.Cleanup(t, err)
	}

	// read private key and public key from the current directory
	privateKey, err := os.ReadFile(fmt.Sprintf("%s.key", name))
	if err != nil {
		f.Cleanup(t, err)
	}
	pubKey, err := os.ReadFile(fmt.Sprintf("%s.pub", name))
	if err != nil {
		f.Cleanup(t, err)
	}

	return string(privateKey), string(pubKey)
}

// SignContainer signs the container with the provided private key
func (f *Framework) SignContainer(t *testing.T, priv, img string) {
	// TODO: find a way to simplify this function - maybe use cosing CLI directly?
	// get SHA of the container image
	t.Setenv("COSIGN_PASSWORD", "")
	args := []string{
		"sign",
		img,
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
	_ = cmd.Flags().Set("key", fmt.Sprintf("%s.key", priv))
	_ = cmd.Flags().Set("tlog-upload", "false")
	_ = cmd.Flags().Set("yes", "true")
	_ = cmd.Flags().Set("allow-http-registry", "true")
	err := cmd.Execute()
	if err != nil {
		f.Cleanup(t, err)
	}
}

// cleanupDeployments removes all deployments from the testing namespace
// if they exist
func (f *Framework) cleanupDeployments(t testing.TB) {

	t.Logf("cleaning up deployments")
	deployments, err := f.k8s.AppsV1().Deployments("test-cases").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		f.Cleanup(t, err)
	}
	for _, d := range deployments.Items {
		err = f.k8s.AppsV1().Deployments("test-cases").Delete(context.Background(), d.Name, metav1.DeleteOptions{})
		if err != nil {
			f.Cleanup(t, err)
		}
	}

	timeout := time.After(30 * time.Second)
	for {
		select {
		case <-timeout:
			f.Cleanup(t, fmt.Errorf("timeout reached while waiting for deployments to be deleted"))
		default:
			pods, err := f.k8s.CoreV1().Pods("test-cases").List(context.Background(), metav1.ListOptions{})
			if err != nil {
				f.Cleanup(t, err)
			}

			if len(pods.Items) == 0 {
				t.Logf("All pods are deleted")
				return
			}
			time.Sleep(5 * time.Second)
		}
	}
}

// CreateSecret creates a secret in the testing namespace
func (f *Framework) CreateSecret(t *testing.T, secret corev1.Secret) {
	t.Logf("creating secret %s", secret.Name)
	s, err := f.k8s.CoreV1().Secrets("test-cases").Create(context.Background(), &secret, metav1.CreateOptions{})
	if err != nil {
		f.Cleanup(t, err)
	}
	t.Logf("created secret %s", s.Name)
}

// cleanupSecrets removes all secrets from the testing namespace
func (f *Framework) cleanupSecrets(t testing.TB) {

	t.Logf("cleaning up secrets")
	secrets, err := f.k8s.CoreV1().Secrets("test-cases").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		f.Cleanup(t, err)
	}
	if len(secrets.Items) == 0 {
		return
	}
	for _, s := range secrets.Items {
		err = f.k8s.CoreV1().Secrets("test-cases").Delete(context.Background(), s.Name, metav1.DeleteOptions{})
		if err != nil {
			f.Cleanup(t, err)
		}
	}
}

// AssertDeploymentFailed asserts that the deployment cannot start
func (f *Framework) AssertDeploymentFailed(t *testing.T, d appsv1.Deployment) {

	t.Logf("waiting for deployment %s to fail", d.Name)

	// watch for replicasets of the deployment
	rsName, err := f.waitForReplicaSetCreation(t, d)
	if err != nil {
		f.Cleanup(t, err)
	}

	// get warning events of deployment's namespace and check if the deployment failed
	w, err := f.k8s.CoreV1().Events("test-cases").Watch(context.Background(), metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s", rsName),
	})
	if err != nil {
		f.Cleanup(t, err)
	}

	timeout := time.After(30 * time.Second)
	for event := range w.ResultChan() {
		select {
		case <-timeout:
			f.Cleanup(t, fmt.Errorf("timeout reached while waiting for deployment to fail"))
		default:
			e, ok := event.Object.(*corev1.Event)
			if !ok {
				time.Sleep(5 * time.Second)
				continue
			}
			if e.Reason == "FailedCreate" {
				t.Logf("deployment %s failed: %s", d.Name, e.Message)
				return
			}
			time.Sleep(5 * time.Second)
		}
	}
}

func (f *Framework) waitForReplicaSetCreation(t *testing.T, d appsv1.Deployment) (string, error) {
	rs, err := f.k8s.AppsV1().ReplicaSets("test-cases").Watch(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", d.Name),
	})
	if err != nil {
		f.Cleanup(t, err)
	}

	timeout := time.After(30 * time.Second)
	for event := range rs.ResultChan() {
		select {
		case <-timeout:
			return "", fmt.Errorf("timeout reached while waiting for replicaset to be created")
		default:
			rs, ok := event.Object.(*appsv1.ReplicaSet)
			if ok {
				t.Logf("replicaset %s created", rs.Name)
				return rs.Name, nil
			}
			time.Sleep(5 * time.Second)
		}
	}
	return "", fmt.Errorf("failed to wait for replicaset creation")
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
