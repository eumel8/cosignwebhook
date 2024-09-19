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
	t   *testing.T
	err error
}

// New creates a new Framework
func New(t *testing.T) (*Framework, error) {
	if t == nil {
		return nil, fmt.Errorf("test object must not be nil")
	}

	k8s, err := createClientSet()
	if err != nil {
		return nil, err
	}

	return &Framework{
		k8s: k8s,
		t:   t,
	}, nil
}

func createClientSet() (k8sClient *kubernetes.Clientset, err error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.Getenv("HOME") + "/.kube/config"
	}

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
// and cleans up the testing directory.
func (f *Framework) Cleanup() {
	f.cleanupKeys()
	f.cleanupDeployments()
	f.cleanupSecrets()
	if f.err != nil {
		f.t.Fatal(f.err)
	}
}

// cleanupDeployments removes all deployments from the testing namespace
// if they exist
func (f *Framework) cleanupDeployments() {
	if f.k8s == nil {
		return
	}

	f.t.Logf("cleaning up deployments")
	deployments, err := f.k8s.AppsV1().Deployments("test-cases").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		f.err = err
		return
	}
	for _, d := range deployments.Items {
		err = f.k8s.AppsV1().Deployments("test-cases").Delete(context.Background(), d.Name, metav1.DeleteOptions{})
		if err != nil {
			f.err = err
			return
		}
	}

	timeout := time.After(30 * time.Second)
	for {
		select {
		case <-timeout:
			f.err = fmt.Errorf("timeout reached while waiting for deployments to be deleted")
		default:
			pods, err := f.k8s.CoreV1().Pods("test-cases").List(context.Background(), metav1.ListOptions{})
			if err != nil {
				f.err = err
				return
			}

			if len(pods.Items) == 0 {
				f.t.Logf("All pods are deleted")
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// cleanupSecrets removes all secrets from the testing namespace
func (f *Framework) cleanupSecrets() {
	if f.k8s == nil {
		return
	}

	f.t.Logf("cleaning up secrets")
	secrets, err := f.k8s.CoreV1().Secrets("test-cases").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		f.err = err
		return
	}
	if len(secrets.Items) == 0 {
		f.t.Log("no secrets to delete")
		return
	}
	for _, s := range secrets.Items {
		err = f.k8s.CoreV1().Secrets("test-cases").Delete(context.Background(), s.Name, metav1.DeleteOptions{})
		if err != nil {
			f.err = err
			return
		}
	}
	f.t.Log("all secrets are deleted")
}

// GetPods returns the pod(s) of the deployment. The fetch is done by label selector (app=<deployment name>)
// If the get request fails, the test will fail and the framework will be cleaned up
func (f *Framework) GetPods(d appsv1.Deployment) *corev1.PodList {
	if f.err != nil {
		return nil
	}

	pods, err := f.k8s.CoreV1().Pods(d.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", d.Name),
	})
	if err != nil {
		f.err = err
	}
	return pods
}

// CreateDeployment creates a deployment in the testing namespace
func (f *Framework) CreateDeployment(d appsv1.Deployment) {
	if f.err != nil {
		return
	}

	f.t.Logf("creating deployment %s", d.Name)
	_, err := f.k8s.AppsV1().Deployments(d.Namespace).Create(context.Background(), &d, metav1.CreateOptions{})
	if err != nil {
		f.err = err
		return
	}
	f.t.Logf("deployment %s created", d.Name)
}

// CreateSecret creates a secret in the testing namespace
func (f *Framework) CreateSecret(s corev1.Secret) {
	if f.err != nil {
		return
	}

	f.t.Logf("creating secret %s", s.Name)
	_, err := f.k8s.CoreV1().Secrets(s.Namespace).Create(context.Background(), &s, metav1.CreateOptions{})
	if err != nil {
		f.err = err
		return
	}
	f.t.Logf("secret %s created", s.Name)
}

// WaitForDeployment waits until the deployment is ready
func (f *Framework) WaitForDeployment(d appsv1.Deployment) {
	if f.err != nil {
		return
	}

	f.t.Logf("waiting for deployment %s to be ready", d.Name)
	// wait until the deployment is ready
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	w, err := f.k8s.AppsV1().Deployments(d.Namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", d.Name),
	})
	if err != nil {
		f.err = err
		return
	}

	for {
		select {
		case <-ctx.Done():
			f.err = fmt.Errorf("timeout reached while waiting for deployment to be ready")
		case event := <-w.ResultChan():
			deployment, ok := event.Object.(*appsv1.Deployment)
			if !ok {
				time.Sleep(500 * time.Millisecond)
				continue
			}

			if deployment.Status.ReadyReplicas == 1 {
				f.t.Logf("deployment %s is ready", d.Name)
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// waitForReplicaSetCreation waits for the replicaset of the given deployment to be created
func (f *Framework) waitForReplicaSetCreation(d appsv1.Deployment) string {
	if f.err != nil {
		return ""
	}

	rs, err := f.k8s.AppsV1().ReplicaSets(d.Namespace).Watch(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", d.Name),
	})
	if err != nil {
		f.err = err
		return ""
	}

	ctx, done := context.WithTimeout(context.Background(), 30*time.Second)
	defer done()

	for {
		select {
		case <-ctx.Done():
			f.err = fmt.Errorf("timeout reached while waiting for replicaset to be created")
		case event := <-rs.ResultChan():
			rs, ok := event.Object.(*appsv1.ReplicaSet)
			if ok {
				f.t.Logf("replicaset %s created", rs.Name)
				return rs.Name
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// AssertDeploymentFailed asserts that the deployment cannot start
func (f *Framework) AssertDeploymentFailed(d appsv1.Deployment) {
	if f.err != nil {
		return
	}

	f.t.Logf("waiting for deployment %s to fail", d.Name)

	// watch for replicasets of the deployment
	rsName := f.waitForReplicaSetCreation(d)
	if rsName == "" {
		return
	}

	// get warning events of deployment's namespace and check if the deployment failed
	w, err := f.k8s.CoreV1().Events(d.Namespace).Watch(context.Background(), metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s", rsName),
	})
	if err != nil {
		f.err = err
		return
	}

	ctx, done := context.WithTimeout(context.Background(), 30*time.Second)
	defer done()

	for {
		select {
		case <-ctx.Done():
			f.err = fmt.Errorf("timeout reached while waiting for deployment to fail")
		case event := <-w.ResultChan():
			e, ok := event.Object.(*corev1.Event)
			if !ok {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			if e.Reason == "FailedCreate" {
				f.t.Logf("deployment %s failed: %s", d.Name, e.Message)
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// AssertEventForPod asserts that a PodVerified event is created
func (f *Framework) AssertEventForPod(reason string, p corev1.Pod) {
	if f.err != nil {
		return
	}

	f.t.Logf("waiting for %s event to be created for pod %s", reason, p.Name)

	// watch for events of deployment's namespace and check if the podverified event is created
	w, err := f.k8s.CoreV1().Events(p.Namespace).Watch(context.Background(), metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s", p.Name),
	})
	if err != nil {
		f.err = err
		return
	}

	ctx, done := context.WithTimeout(context.Background(), 30*time.Second)
	defer done()

	for {
		select {
		case <-ctx.Done():
			f.err = fmt.Errorf("timeout reached while waiting for event to be created")
		case event := <-w.ResultChan():
			e, ok := event.Object.(*corev1.Event)
			if !ok {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			if e.Reason == reason {
				f.t.Logf("%s event created for pod %s", reason, p.Name)
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}
