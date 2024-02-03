package framework

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
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

// Cleanup removes all resources created by the framework
// and cleans up the testing directory. If an error is passed,
// the test will fail but the cleanup will still be executed.
func (f *Framework) Cleanup(t testing.TB) {
	cleanupKeys(t)
	f.cleanupDeployments(t)
	f.cleanupSecrets(t)
}

// cleanupDeployments removes all deployments from the testing namespace
// if they exist
func (f *Framework) cleanupDeployments(t testing.TB) {
	t.Logf("cleaning up deployments")
	deployments, err := f.k8s.AppsV1().Deployments("test-cases").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		f.Cleanup(t)
		t.Fatal(err)
	}
	for _, d := range deployments.Items {
		err = f.k8s.AppsV1().Deployments("test-cases").Delete(context.Background(), d.Name, metav1.DeleteOptions{})
		if err != nil {
			f.Cleanup(t)
			t.Fatal(err)
		}
	}

	timeout := time.After(30 * time.Second)
	for {
		select {
		case <-timeout:
			f.Cleanup(t)
		default:
			pods, err := f.k8s.CoreV1().Pods("test-cases").List(context.Background(), metav1.ListOptions{})
			if err != nil {
				f.Cleanup(t)
				t.Fatal(err)
			}

			if len(pods.Items) == 0 {
				t.Logf("All pods are deleted")
				return
			}
			time.Sleep(5 * time.Second)
		}
	}
}

// cleanupSecrets removes all secrets from the testing namespace
func (f *Framework) cleanupSecrets(t testing.TB) {
	t.Logf("cleaning up secrets")
	secrets, err := f.k8s.CoreV1().Secrets("test-cases").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		f.Cleanup(t)
		t.Fatal(err)
	}
	if len(secrets.Items) == 0 {
		return
	}
	for _, s := range secrets.Items {
		err = f.k8s.CoreV1().Secrets("test-cases").Delete(context.Background(), s.Name, metav1.DeleteOptions{})
		if err != nil {
			f.Cleanup(t)
			t.Fatal(err)
		}
	}
}

// CreateDeployment creates a deployment in the testing namespace
func (f *Framework) CreateDeployment(t testing.TB, d appsv1.Deployment) {
	_, err := f.k8s.AppsV1().Deployments("test-cases").Create(context.Background(), &d, metav1.CreateOptions{})
	if err != nil {
		f.Cleanup(t)
		t.Fatal(err)
	}
}

// WaitForDeployment waits until the deployment is ready
func (f *Framework) WaitForDeployment(t *testing.T, d appsv1.Deployment) {
	t.Logf("waiting for deployment %s to be ready", d.Name)
	// wait until the deployment is ready
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	w, err := f.k8s.AppsV1().Deployments(d.Namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", d.Name),
	})
	if err != nil {
		f.Cleanup(t)
		t.Fatal(err)
	}

	for {
		select {
		case <-ctx.Done():
			f.Cleanup(t)
			t.Fatal("timeout reached while waiting for deployment to be ready")
		case event := <-w.ResultChan():
			deployment, ok := event.Object.(*appsv1.Deployment)
			if !ok {
				time.Sleep(5 * time.Second)
				continue
			}

			if deployment.Status.ReadyReplicas == 1 {
				t.Logf("deployment %s is ready", d.Name)
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
		f.Cleanup(t)
		t.Fatal(err)
	}
	t.Logf("created secret %s", s.Name)
}

// AssertDeploymentFailed asserts that the deployment cannot start
func (f *Framework) AssertDeploymentFailed(t *testing.T, d appsv1.Deployment) {
	t.Logf("waiting for deployment %s to fail", d.Name)

	// watch for replicasets of the deployment
	rsName, err := f.waitForReplicaSetCreation(t, d)
	if err != nil {
		f.Cleanup(t)
		t.Fatal(err)
	}

	// get warning events of deployment's namespace and check if the deployment failed
	w, err := f.k8s.CoreV1().Events("test-cases").Watch(context.Background(), metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s", rsName),
	})
	if err != nil {
		f.Cleanup(t)
		t.Fatal(err)
	}

	ctx, done := context.WithTimeout(context.Background(), 30*time.Second)
	defer done()

	for {
		select {
		case <-ctx.Done():
			f.Cleanup(t)
			t.Fatal("timeout reached while waiting for deployment to fail")
		case event := <-w.ResultChan():
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

// AssertEventForPod asserts that a PodVerified event is created
func (f *Framework) AssertEventForPod(t *testing.T, reason string, p corev1.Pod) {
	t.Logf("waiting for %s event to be created for pod %s", reason, p.Name)

	// watch for events of deployment's namespace and check if the podverified event is created
	w, err := f.k8s.CoreV1().Events("test-cases").Watch(context.Background(), metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s", p.Name),
	})
	if err != nil {
		f.Cleanup(t)
		t.Fatal(err)
	}

	ctx, done := context.WithTimeout(context.Background(), 30*time.Second)
	defer done()

	for {
		select {
		case <-ctx.Done():
			f.Cleanup(t)
			t.Fatal("timeout reached while waiting for podverified event")
		case event := <-w.ResultChan():
			e, ok := event.Object.(*corev1.Event)
			if !ok {
				time.Sleep(5 * time.Second)
				continue
			}
			if e.Reason == reason {
				t.Logf("%s event created for pod %s", reason, p.Name)
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
		f.Cleanup(t)
		t.Fatal(err)
	}

	ctx, done := context.WithTimeout(context.Background(), 30*time.Second)
	defer done()

	for {
		select {
		case <-ctx.Done():
			f.Cleanup(t)
			t.Fatal("timeout reached while waiting for replicaset to be created")
		case event := <-rs.ResultChan():
			rs, ok := event.Object.(*appsv1.ReplicaSet)
			if ok {
				t.Logf("replicaset %s created", rs.Name)
				return rs.Name, nil
			}
			time.Sleep(5 * time.Second)
		}
	}
}

// GetPods returns the pod(s) of the deployment. The fetch is done by label selector (app=<deployment name>)
// If the get request fails, the test will fail and the framework will be cleaned up
func (f *Framework) GetPods(t *testing.T, d appsv1.Deployment) *corev1.PodList {
	pods, err := f.k8s.CoreV1().Pods("test-cases").List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", d.Name),
	})
	if err != nil {
		f.Cleanup(t)
		t.Fatal(err)
	}
	return pods
}
