package main

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	log "github.com/gookit/slog"
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
	admissionApi  = "admission.k8s.io/v1"
	admissionKind = "AdmissionReview"
	cosignEnvVar  = "COSIGNPUBKEY"
)

// CosignServerHandler listen to admission requests and serve responses
// build certs here: https://raw.githubusercontent.com/openshift/external-dns-operator/fb77a3c547a09cd638d4e05a7b8cb81094ff2476/hack/generate-certs.sh
// generate-certs.sh --service cosignwebhook --webhook cosignwebhook --namespace cosignwebhook --secret cosignwebhook
type CosignServerHandler struct {
	cs kubernetes.Interface
}

func NewCosignServerHandler() *CosignServerHandler {
	cs, err := restClient()
	if err != nil {
		log.Errorf("Can't init rest client: %v", err)
	}
	return &CosignServerHandler{
		cs: cs,
	}
}

// create restClient for get secrets and create events
func restClient() (*kubernetes.Clientset, error) {
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Errorf("error init in-cluster config: %v", err)
		return nil, err
	}
	k8sclientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		log.Errorf("error creating k8sclientset: %v", err)
		return nil, err
	}
	return k8sclientset, err
}

func recordEvent(pod *corev1.Pod, k8sclientset *kubernetes.Clientset) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: k8sclientset.CoreV1().Events("")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "Cosignwebhook"})
	eventRecorder.Eventf(pod, corev1.EventTypeNormal, "Cosignwebhook", "Cosign image verified")
	eventBroadcaster.Shutdown()
}

// get pod object from admission request
func getPod(byte []byte) (*corev1.Pod, *v1.AdmissionReview, error) {
	arRequest := v1.AdmissionReview{}
	if err := json.Unmarshal(byte, &arRequest); err != nil {
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
		log.Error("Error deserializing pod")
		return nil, nil, err
	}
	return &pod, &arRequest, nil
}

// getPubKeyFromEnv grabs the public key from Pod's environment
func (csh *CosignServerHandler) getPubKeyFromEnv(pod *corev1.Pod, c int) (string, error) {
	for i := 0; i < len(pod.Spec.Containers[c].Env); i++ {
		if pod.Spec.Containers[c].Env[i].Name == cosignEnvVar {

			if len(pod.Spec.Containers[c].Env[i].Value) != 0 {
				log.Debugf("Found public key in env var %s/%s", pod.Namespace, pod.Name)
				return pod.Spec.Containers[c].Env[i].Value, nil
			}

			if pod.Spec.Containers[c].Env[i].ValueFrom.SecretKeyRef != nil {
				log.Debugf("Found public key in secret of %s/%s/%s", pod.Namespace, pod.Name, pod.Spec.Containers[c].Name)
				return csh.getSecretValue(pod.Namespace,
					pod.Spec.Containers[c].Env[i].ValueFrom.SecretKeyRef.Name,
					pod.Spec.Containers[c].Env[i].ValueFrom.SecretKeyRef.Key,
				)
			}
		}
	}
	return "", fmt.Errorf("no env var found in %s/%s/%s", pod.Namespace, pod.Name, pod.Spec.Containers[c].Name)
}

// getSecretValue returns the value of passed key for the secret with passed name in passed namespace
func (csh *CosignServerHandler) getSecretValue(namespace string, name string, key string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	secret, err := csh.cs.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		log.Debugf("Can't get secret %s/%s : %v", namespace, name, err)
		return "", err
	}
	value := secret.Data[key]
	if len(value) == 0 {
		log.Errorf("Secret value of %q is empty for %s/%s", key, namespace, name)
		return "", nil
	}
	log.Debugf("Found public key in secret %s/%s, value: %s", namespace, name, value)
	return string(value), nil
}

func (csh *CosignServerHandler) healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (csh *CosignServerHandler) serve(w http.ResponseWriter, r *http.Request) {
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

	for i, _ := range pod.Spec.Containers {

		// Get public key from environment var
		pubKey, err := csh.getPubKeyFromEnv(pod, i)
		if err != nil {
			log.Debugf("Could not get public key from environment variable in %s/%s/%s: %v. Trying to get public key from secret", pod.Namespace, pod.Name, pod.Spec.Containers[i].Name, err)
		}

		// If no public key get here, try to load default secret
		if len(pubKey) == 0 {
			pubKey, err = csh.getSecretValue(pod.Namespace, "cosignwebhook", cosignEnvVar)
			if err != nil {
				log.Debugf("Could not get public key from secret in %s/%s/%s: %v", pod.Namespace, pod.Name, pod.Spec.Containers[i].Name, err)
			}
		}

		// Still no public key, we don't care. Otherwise, POD won't start if we return with 403
		if len(pubKey) == 0 {
			// log.Errorf("No public key set in %s/%s", pod.Namespace, pod.Name)
			// return OK if no key is set, so user don't want a verification
			// otherwise set failurePolicy: Skip in ValidatingWebhookConfiguration
			log.Debugf("No public key found for %s/%s/%s, skipping verification", pod.Namespace, pod.Name, pod.Spec.Containers[i].Name)
			resp, err := json.Marshal(admissionResponse(200, true, "Success", "Cosign image skipped", arRequest))
			if err != nil {
				log.Errorf("Can't encode response: %v", err)
				http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
			}
			if _, err := w.Write(resp); err != nil {
				log.Errorf("Can't write response: %v", err)
				http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
			}
			return
		}

		// Lookup image name of all container
		image := pod.Spec.Containers[i].Image
		refImage, err := name.ParseReference(image)
		if err != nil {
			log.Errorf("Error ParseRef image: %v", err)
			resp, err := json.Marshal(admissionResponse(403, false, "Failure", "Cosign ParseRef image failed", arRequest))
			if err != nil {
				log.Errorf("Can't encode response %s/%s/%s: %v", pod.Namespace, pod.Name, pod.Spec.Containers[i].Name, err)
				http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
			}
			if _, err := w.Write(resp); err != nil {
				log.Errorf("Can't write response: %v", err)
				http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
			}
			return
		}

		// Lookup imagePullSecrets for reviewed POD
		imagePullSecrets := make([]string, 0, len(pod.Spec.ImagePullSecrets))
		for _, s := range pod.Spec.ImagePullSecrets {
			imagePullSecrets = append(imagePullSecrets, s.Name)
		}
		opt := k8schain.Options{
			Namespace:          pod.Namespace,
			ServiceAccountName: pod.Spec.ServiceAccountName,
			ImagePullSecrets:   imagePullSecrets,
		}

		// Encrypt public key
		publicKey, err := cryptoutils.UnmarshalPEMToPublicKey([]byte(pubKey))
		if err != nil {
			log.Errorf("Error UnmarshalPEMToPublicKey %s/%s/%s: %v", pod.Namespace, pod.Name, pod.Spec.Containers[i].Name, err)
			resp, err := json.Marshal(admissionResponse(403, false, "Failure", "Cosign UnmarshalPEMToPublicKey failed", arRequest))
			if err != nil {
				log.Errorf("Can't encode response %s/%s/%s: %v", pod.Namespace, pod.Name, pod.Spec.Containers[i].Name, err)
				http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
			}
			if _, err := w.Write(resp); err != nil {
				log.Errorf("Can't write response: %v", err)
				http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
			}
			return
		}

		// Load public key to verify
		cosignLoadKey, err := signature.LoadECDSAVerifier(publicKey.(*ecdsa.PublicKey), crypto.SHA256)
		if err != nil {
			log.Errorf("Error LoadECDSAVerifier %s/%s/%s: %v", pod.Namespace, pod.Name, pod.Spec.Containers[i].Name, err)
			resp, err := json.Marshal(admissionResponse(403, false, "Failure", "Cosign key encoding failed", arRequest))
			if err != nil {
				log.Errorf("Can't encode response %s/%s/%s: %v", pod.Namespace, pod.Name, pod.Spec.Containers[i].Name, err)
				http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
			}
			if _, err := w.Write(resp); err != nil {
				log.Errorf("Can't write response %s/%s/%s: %v", pod.Namespace, pod.Name, pod.Spec.Containers[i].Name, err)
				http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
			}
			return
		}

		// Kubernetes client to operate in cluster
		kc, err := k8schain.NewInCluster(context.Background(), opt)
		if err != nil {
			log.Errorf("Error k8schain %s/%s/%s: %v", pod.Namespace, pod.Name, pod.Spec.Containers[i].Name, err)
			resp, err := json.Marshal(admissionResponse(403, false, "Failure", "Cosign UnmarshalPEMToPublicKey failed", arRequest))
			if err != nil {
				log.Errorf("Can't encode response %s/%s/%s: %v", pod.Namespace, pod.Name, pod.Spec.Containers[i].Name, err)
				http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
			}
			if _, err := w.Write(resp); err != nil {
				log.Errorf("Can't write response: %v", err)
				http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
			}
			return
		}

		// Verify signature on remote image with the presented public key
		remoteOpts := []ociremote.Option{ociremote.WithRemoteOptions(remote.WithAuthFromKeychain(kc))}
		_, _, err = cosign.VerifyImageSignatures(
			context.Background(),
			refImage,
			&cosign.CheckOpts{
				RegistryClientOpts: remoteOpts,
				SigVerifier:        cosignLoadKey,
				IgnoreSCT:          true,
				IgnoreTlog:         true,
			})

		// this is always false,
		// log.Info("Resp bundleVerified: ", bundleVerified)

		// Verify Image failed, needs to reject pod start
		if err != nil {
			log.Errorf("Error VerifyImageSignatures %s/%s/%s: %v", pod.Namespace, pod.Name, pod.Spec.Containers[i].Name, err)
			resp, err := json.Marshal(admissionResponse(403, false, "Failure", "Cosign image verification failed", arRequest))
			if err != nil {
				log.Errorf("Can't encode response: %v", err)
				http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
			}
			if _, err := w.Write(resp); err != nil {
				log.Errorf("Can't write response: %v", err)
				http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
			}
		} else {
			// count successful verifies for prometheus metric
			verifiedProcessed.Inc()
			log.Infof("Image verified successfully: %s/%s/%s", pod.Namespace, pod.Name, pod.Spec.Containers[i].Name)

			// Just another K8S client to record events
			clientset, err := restClient()
			if err != nil {
				log.Errorf("Can't init rest client for event recorder: %v", err)
			} else {
				recordEvent(pod, clientset)
			}
		}
	}
	resp, err := json.Marshal(admissionResponse(200, true, "Success", "Cosign image verified", arRequest))
	// Verify Image successful, needs to allow pod start
	if err != nil {
		log.Errorf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}
	if _, err := w.Write(resp); err != nil {
		log.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}

// Template for AdmissionReview
func admissionResponse(admissionCode int32, admissionPermissions bool, admissionStatus string, admissionMessage string, ar *v1.AdmissionReview) v1.AdmissionReview {
	return v1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       admissionKind,
			APIVersion: admissionApi,
		},
		Response: &v1.AdmissionResponse{
			Allowed: admissionPermissions,
			UID:     ar.Request.UID,
			Result: &metav1.Status{
				Status:  admissionStatus,
				Message: admissionMessage,
				Code:    admissionCode,
			},
		},
	}
}
