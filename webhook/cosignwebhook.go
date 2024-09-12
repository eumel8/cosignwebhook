package webhook

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	log "github.com/gookit/slog"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"k8s.io/apimachinery/pkg/types"

	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"

	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	ociremote "github.com/sigstore/cosign/v2/pkg/oci/remote"

	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
)

const (
	admissionApi           = "admission.k8s.io/v1"
	admissionKind          = "AdmissionReview"
	CosignEnvVar           = "COSIGNPUBKEY"
	CosignRepositoryEnvVar = "COSIGN_REPOSITORY"
	k8sTimeout             = 10 * time.Second
)

var (
	opsProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cosign_processed_ops_total",
		Help: "The total number of processed events",
	})
	verifiedProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cosign_processed_verified_total",
		Help: "The number of verfified events",
	})
)

// CosignServerHandler listen to admission requests and serve responses
// build certs here: https://raw.githubusercontent.com/openshift/external-dns-operator/fb77a3c547a09cd638d4e05a7b8cb81094ff2476/hack/generate-certs.sh
// generate-certs.sh --service cosignwebhook --webhook cosignwebhook --namespace cosignwebhook --secret cosignwebhook
type CosignServerHandler struct {
	cs kubernetes.Interface
	kc authn.Keychain
	eb record.EventBroadcaster
}

func NewCosignServerHandler() *CosignServerHandler {
	cs, err := restClient()
	if err != nil {
		log.Errorf("Can't init rest client: %v", err)
	}
	eb := record.NewBroadcaster()
	eb.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: cs.CoreV1().Events("")})
	return &CosignServerHandler{
		cs: cs,
		eb: eb,
	}
}

// create restClient for get secrets and create events
func restClient() (*kubernetes.Clientset, error) {
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Errorf("error init in-cluster config: %v", err)
		return nil, err
	}
	cs, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		log.Errorf("error creating k8sclientset: %v", err)
		return nil, err
	}
	return cs, err
}

// recordPodVerified emits a PodVerified event for the container
func (csh *CosignServerHandler) recordPodVerified(p *corev1.Pod) {
	er := csh.eb.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "Cosignwebhook", Host: os.Getenv("HOSTNAME")})
	er.Event(p, corev1.EventTypeNormal, "PodVerified", "Signature of pod's images(s) verified successfully")
}

// recordNoVerification emits a NoVerification event for the container
func (csh *CosignServerHandler) recordNoVerification(p *corev1.Pod) {
	er := csh.eb.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "Cosignwebhook", Host: os.Getenv("HOSTNAME")})
	er.Event(p, corev1.EventTypeNormal, "NoVerification", "No signature verification performed")
}

// getPod returns the pod object from admission review request
func getPod(b []byte) (*corev1.Pod, *v1.AdmissionReview, error) {
	arRequest := v1.AdmissionReview{}
	if err := json.Unmarshal(b, &arRequest); err != nil {
		log.Error("Incorrect body")
		return nil, nil, err
	}
	if arRequest.Request == nil {
		log.Error("AdmissionReview request not found")
		return nil, nil, fmt.Errorf("admissionreview request not found")
	}
	raw := arRequest.Request.Object.Raw
	pod := corev1.Pod{}
	if err := json.Unmarshal(raw, &pod); err != nil {
		log.Error("Error deserializing container")
		return nil, nil, err
	}
	return &pod, &arRequest, nil
}

// getPubKeyFromEnv procures the public key from the container's environment section, if present.
// Else it returns an empty string and an error.
func (csh *CosignServerHandler) getPubKeyFromEnv(c *corev1.Container, ns string) (string, error) {
	for _, envVar := range c.Env {
		if envVar.Name == CosignEnvVar {
			if envVar.Value != "" {
				log.Debugf("Found public key in env var for container %q", c.Name)
				return envVar.Value, nil
			}

			if envVar.ValueFrom != nil && envVar.ValueFrom.SecretKeyRef != nil {
				log.Debugf("Found reference to public key in secret %q for container %q", envVar.ValueFrom.SecretKeyRef.Name, c.Name)
				return csh.getSecretValue(ns,
					envVar.ValueFrom.SecretKeyRef.Name,
					envVar.ValueFrom.SecretKeyRef.Key,
				)
			}
		}
	}
	return "", fmt.Errorf("no env var found in container %q in namespace %q", c.Name, ns)
}

// getSecretValue returns the value of passed key for the secret with passed name in passed namespace
func (csh *CosignServerHandler) getSecretValue(namespace, secret, key string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), k8sTimeout)
	defer cancel()
	s, err := csh.cs.CoreV1().Secrets(namespace).Get(ctx, secret, metav1.GetOptions{})
	if err != nil {
		log.Debugf("Can't get secret %s/%s : %v", namespace, secret, err)
		return "", err
	}
	value := s.Data[key]
	if len(value) == 0 {
		log.Errorf("Secret value of %q is empty for %s/%s", key, namespace, secret)
		return "", nil
	}
	log.Debugf("Found public key in secret %s/%s, value: %s", namespace, secret, value)
	return string(value), nil
}

// Healthz is called by /healthz for health checks and returns 'ok' if http connection is ready
func (csh *CosignServerHandler) Healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("ok"))
	if err != nil {
		log.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError) //nolint:gocritic // function returns after call
	}
}

// Serve the main function for /validate to validate the webhook request or /metrics to get Prometheus data
func (csh *CosignServerHandler) Serve(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	// Url path of metrics
	if r.URL.Path == "/metrics" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Url path of admission
	if r.URL.Path != "/validate" {
		log.Error("No validate URI")
		http.Error(w, "no validate", http.StatusBadRequest)
		return
	}

	if len(body) == 0 {
		log.Error("Empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// count each request for prometheus metric
	opsProcessed.Inc()

	pod, arRequest, err := getPod(body)
	if err != nil {
		log.Errorf("Error getPod in %s/%s: %v", pod.Namespace, pod.Name, err)
		http.Error(w, "incorrect body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	kc, err := newKeychainForPod(ctx, pod)
	if err != nil {
		http.Error(w, "Failed initializing k8schain", http.StatusInternalServerError)
		return
	}
	csh.kc = kc

	signatureChecked := false
	for i := range pod.Spec.InitContainers {
		pubKey := csh.getPubKeyFor(pod.Spec.InitContainers[i], pod.Namespace)
		if pubKey == "" {
			continue
		}

		err = csh.verifyContainer(pod.Spec.InitContainers[i], pubKey)
		if err != nil {
			log.Errorf("Error verifying init container %s/%s/%s: %v", pod.Namespace, pod.Name, pod.Spec.InitContainers[0].Name, err)
			deny(w, err.Error(), arRequest.Request.UID)
			return
		}
		signatureChecked = true
	}

	for i := range pod.Spec.Containers {
		pubKey := csh.getPubKeyFor(pod.Spec.Containers[i], pod.Namespace)
		if pubKey == "" {
			continue
		}
		err = csh.verifyContainer(pod.Spec.Containers[i], pubKey)
		if err != nil {
			log.Errorf("Error verifying container %s/%s/%s: %v", pod.Namespace, pod.Name, pod.Spec.Containers[i].Name, err)
			deny(w, err.Error(), arRequest.Request.UID)
			return
		}
		signatureChecked = true
	}

	accept(w, "Cosign verification passed", arRequest.Request.UID)
	if signatureChecked {
		csh.recordPodVerified(pod)
		return
	}
	csh.recordNoVerification(pod)
}

// newKeychainForPod builds a new Keychain for the pod
func newKeychainForPod(ctx context.Context, pod *corev1.Pod) (authn.Keychain, error) {
	imagePullSecrets := make([]string, 0, len(pod.Spec.ImagePullSecrets))
	for _, s := range pod.Spec.ImagePullSecrets {
		imagePullSecrets = append(imagePullSecrets, s.Name)
	}
	opt := k8schain.Options{
		Namespace:          pod.Namespace,
		ServiceAccountName: pod.Spec.ServiceAccountName,
		ImagePullSecrets:   imagePullSecrets,
		UseMountSecrets:    false,
	}

	kc, err := k8schain.NewInCluster(ctx, opt)
	if err != nil {
		log.Errorf("Error intializing k8schain %s/%s: %v", pod.Namespace, pod.Name, err)
		return nil, err
	}
	return kc, nil
}

// getPubKeyFor searches for the public key to verify the container's signature.
// If no public key is found, it returns an empty string.
func (csh *CosignServerHandler) getPubKeyFor(c corev1.Container, ns string) string { //nolint:gocritic // better for garbage collection
	if c.Image == "" {
		log.Debugf("Container %q has no image, skipping verification", c.Name)
		return ""
	}
	if len(c.Env) == 0 {
		log.Debugf("Container %q has no env vars, skipping verification", c.Name)
		return ""
	}
	pubKey, err := csh.getPubKeyFromEnv(&c, ns)
	if err != nil {
		log.Debugf("Could not find pub key in container's %q environment: %v", c.Name, err)
	}

	// If no public key get here, try to load default secret
	// Should be deprecated in future versions
	if pubKey == "" {
		pubKey, err = csh.getSecretValue(ns, "cosignwebhook", CosignEnvVar)
		if err != nil {
			log.Debugf("Could not find pub key from default secret: %v", err)
		}
	}

	// Still no public key, we don't care. Otherwise, POD won't start if we return with 403
	// In future versions this should block the start of the container
	if pubKey == "" {
		log.Debugf("No public key found, returning")
		return ""
	}

	log.Debugf("Found public key for container %q", c.Name)
	return pubKey
}

// verifyContainer verifies the signature of the container image
func (csh *CosignServerHandler) verifyContainer(c corev1.Container, pubKey string) error { //nolint:gocritic // better for garbage collection
	log.Debugf("Verifying container %s", c.Name)

	// Lookup image name of current container
	image := c.Image
	refImage, err := name.ParseReference(image)
	if err != nil {
		log.Errorf("Error parsing image reference: %v", err)
		return fmt.Errorf("could parse image reference for image %q", image)
	}

	// Encrypt public key
	publicKey, err := cryptoutils.UnmarshalPEMToPublicKey([]byte(pubKey))
	if err != nil {
		log.Errorf("Error unmarshalling public key: %v", err)
		return fmt.Errorf("public key for image %q malformed", image)
	}

	// depending on key algorithm, we need to load the key differently
	// currently only ECDSA and RSA are supported
	// Load public key to verify
	verifier, err := csh.newVerifierForKey(publicKey)
	if err != nil {
		return err
	}

	// Verify signature on remote image with the presented public key
	remoteOpts := []ociremote.Option{
		ociremote.WithRemoteOptions(remote.WithAuthFromKeychain(csh.kc)),
	}
	if r := getCosignRepository(c.Env); r != "" {
		repository, repErr := name.NewRepository(r)
		if repErr != nil {
			log.Errorf("Error parsing remote signature repository: %v", repErr)
			return fmt.Errorf("could not parse signature repository %q", r)
		}
		log.Debugf("Remote signature repository overridden with: %v", repository)
		remoteOpts = append(remoteOpts, ociremote.WithTargetRepository(repository))
	}

	log.Debugf("Verifying image %q with public key %q", image, pubKey)
	_, _, err = cosign.VerifyImageSignatures(
		context.Background(),
		refImage,
		&cosign.CheckOpts{
			RegistryClientOpts: remoteOpts,
			SigVerifier:        verifier,
			IgnoreSCT:          true,
			IgnoreTlog:         true,
		})
	if err != nil {
		log.Errorf("Error verifying signature: %v", err)
		return fmt.Errorf("signature for %q couldn't be verified", image)
	}

	verifiedProcessed.Inc()
	log.Infof("Image %q verified successfully", image)
	return nil
}

// newVerifierForKey creates a new signature verifier for the given public key.
func (*CosignServerHandler) newVerifierForKey(publicKey crypto.PublicKey) (signature.Verifier, error) {
	switch pub := publicKey.(type) {
	case *ecdsa.PublicKey:
		return signature.LoadECDSAVerifier(pub, crypto.SHA256)
	case *rsa.PublicKey:
		return signature.LoadRSAPKCS1v15Verifier(pub, crypto.SHA256)
	default:
		log.Errorf("Unsupported public key type: %t", publicKey)
		return nil, fmt.Errorf("unsupported public key type: %t", publicKey)
	}
}

// getCosignRepository returns the repository specified by the COSIGN_REPOSITORY environment variable
// of the container, or nil if not set.
func getCosignRepository(env []corev1.EnvVar) string {
	for _, e := range env {
		if e.Name == CosignRepositoryEnvVar {
			return e.Value
		}
	}
	return ""
}

// deny prevents the container from starting
func deny(w http.ResponseWriter, msg string, uid types.UID) {
	resp, err := json.Marshal(admissionReview(http.StatusForbidden, false, "Failure", msg, uid))
	if err != nil {
		log.Errorf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(resp); err != nil {
		log.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}

// accept allows the container to start
func accept(w http.ResponseWriter, msg string, uid types.UID) {
	resp, err := json.Marshal(admissionReview(http.StatusOK, true, "Success", msg, uid))
	if err != nil {
		log.Errorf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(resp); err != nil {
		log.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}

// admissionReview returns a AdmissionReview object with the passed parameters
func admissionReview(admissionCode int32, admissionPermissions bool, admissionStatus, admissionMessage string, requestUID types.UID) v1.AdmissionReview {
	return v1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       admissionKind,
			APIVersion: admissionApi,
		},
		Response: &v1.AdmissionResponse{
			Allowed: admissionPermissions,
			UID:     requestUID,
			Result: &metav1.Status{
				Status:  admissionStatus,
				Message: admissionMessage,
				Code:    admissionCode,
			},
		},
	}
}
