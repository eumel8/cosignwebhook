package main

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/golang/glog"
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	//	"k8s.io/apimachinery/pkg/types"

	"github.com/google/go-containerregistry/pkg/name"
	//"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	ociremote "github.com/sigstore/cosign/pkg/oci/remote"
	"github.com/sigstore/cosign/v2/pkg/cosign"

	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
)

const (
	admissionApi  = "admission.k8s.io/v1"
	admissionKind = "AdmissionReview"
	cosignEnvVar  = "COSIGNPUBKEY"
)

//var requestUid = v1.AdmissionReview{Request: &v1.AdmissionRequest{UID: ""}}
//var requestUid = v1.AdmissionReview.Request.UID

// CosignServerHandler listen to admission requests and serve responses
// build certs here: https://raw.githubusercontent.com/openshift/external-dns-operator/fb77a3c547a09cd638d4e05a7b8cb81094ff2476/hack/generate-certs.sh
// generate-certs.sh --service cosignwebhook --webhook cosignwebhook --namespace cosignwebhook --secret cosignwebhook
type CosignServerHandler struct {
}

func (cs *CosignServerHandler) serve(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		glog.Error("empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}
	glog.Info("Received request")

	// Url path of admission
	if r.URL.Path != "/validate" {
		glog.Error("no validate")
		http.Error(w, "no validate", http.StatusBadRequest)
		return
	}

	arRequest := v1.AdmissionReview{}
	if err := json.Unmarshal(body, &arRequest); err != nil {
		glog.Error("incorrect body")
		http.Error(w, "incorrect body", http.StatusBadRequest)
		return
	}

	raw := arRequest.Request.Object.Raw
	// requestUid := arRequest.Request.UID
	pod := corev1.Pod{}
	if err := json.Unmarshal(raw, &pod); err != nil {
		glog.Error("error deserializing pod")
		return
	}

	pubKey := ""
	for i := 0; i < len(pod.Spec.Containers[0].Env); i++ {
		value := pod.Spec.Containers[0].Env[i].Value
		if pod.Spec.Containers[0].Env[i].Name == cosignEnvVar {
			pubKey = value
		}
	}

	if len(pubKey) == 0 {
		glog.Errorf("No pubKey env in %s/%s", pod.Namespace, pod.Name)
		// return OK if no key is set, so user don't want a verification
		// otherwise set failurePolicy: Skip in ValidatingWebhookConfiguration
		resp, err := json.Marshal(admissionResponse(200, true, "Success", "Cosign image skipped", &arRequest))
		if err != nil {
			glog.Errorf("Can't encode response: %v", err)
			http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
		}
		if _, err := w.Write(resp); err != nil {
			glog.Errorf("Can't write response: %v", err)
			http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		}
		return
	}

	image := pod.Spec.Containers[0].Image
	refImage, err := name.ParseReference(image)

	if err != nil {
		glog.Errorf("Error ParseRef image: %v", err)
		resp, err := json.Marshal(admissionResponse(403, false, "Failure", "Cosign ParseRef image failed", &arRequest))
		if err != nil {
			glog.Errorf("Can't encode response %s/%s: %v", pod.Namespace, pod.Name, err)
			http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
		}
		if _, err := w.Write(resp); err != nil {
			glog.Errorf("Can't write response: %v", err)
			http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		}
		return
	}
	/*
		imagePullSecrets := make([]string, 0, len(pod.Spec.Template.Spec.ImagePullSecrets))
		for _, s := range pod.Spec.Template.Spec.ImagePullSecrets {
			imagePullSecrets = append(imagePullSecrets, s.Name)
		}
		ns := getNamespace(ctx, pod.Namespace)
		opt := k8schain.Options{
			Namespace:          ns,
			ServiceAccountName: pod.Spec.Template.Spec.ServiceAccountName,
			ImagePullSecrets:   imagePullSecrets,
		}

	*/
	/*
			imagePullSecrets := make([]string, 0, len(wp.Spec.Template.Spec.ImagePullSecrets))
		for _, s := range pod.Spec.Template.Spec.ImagePullSecrets {
			imagePullSecrets = append(imagePullSecrets, s.Name)
		}


	*/

	publicKey, err := cryptoutils.UnmarshalPEMToPublicKey([]byte(pubKey))
	if err != nil {
		glog.Errorf("Error UnmarshalPEMToPublicKey %s/%s: %v", pod.Namespace, pod.Name, err)
		resp, err := json.Marshal(admissionResponse(403, false, "Failure", "Cosign UnmarshalPEMToPublicKey failed", &arRequest))
		if err != nil {
			glog.Errorf("Can't encode response %s/%s: %v", pod.Namespace, pod.Name, err)
			http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
		}
		if _, err := w.Write(resp); err != nil {
			glog.Errorf("Can't write response: %v", err)
			http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		}
		return
	}

	cosignLoadKey, err := signature.LoadECDSAVerifier(publicKey.(*ecdsa.PublicKey), crypto.SHA256)
	if err != nil {
		glog.Errorf("Error LoadECDSAVerifier %s/%s: %v", pod.Namespace, pod.Name, err)
		resp, err := json.Marshal(admissionResponse(403, false, "Failure", "Cosign key encoding failed", &arRequest))
		if err != nil {
			glog.Errorf("Can't encode response %s/%s: %v", pod.Namespace, pod.Name, err)
			http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
		}
		if _, err := w.Write(resp); err != nil {
			glog.Errorf("Can't write response %s/%s: %v", pod.Namespace, pod.Name, err)
			http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		}
		return
	}

	kc, err := k8schain.NewInCluster(context.Background(), k8schain.Options{})
	if err != nil {
		glog.Errorf("Error k8schain %s/%s: %v", pod.Namespace, pod.Name, err)
		resp, err := json.Marshal(admissionResponse(403, false, "Failure", "Cosign UnmarshalPEMToPublicKey failed", &arRequest))
		if err != nil {
			glog.Errorf("Can't encode response %s/%s: %v", pod.Namespace, pod.Name, err)
			http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
		}
		if _, err := w.Write(resp); err != nil {
			glog.Errorf("Can't write response: %v", err)
			http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// remoteOpts := []ociremote.Option{ociremote.WithRemoteOptions()}
	remoteOpts := []ociremote.Option{ociremote.WithRemoteOptions(remote.WithAuthFromKeychain(kc))}

	_, _, err = cosign.VerifyImageSignatures(
		context.Background(),
		refImage,
		&cosign.CheckOpts{
			RegistryClientOpts: remoteOpts,
			SigVerifier:        cosignLoadKey,
			// add settings for cosign 2.0
			//IgnoreSCT:      true,
			//SkipTlogVerify: true,
		})

	// this is always false,
	// glog.Info("Resp bundleVerified: ", bundleVerified)

	// Verify Image failed, needs to reject pod start
	if err != nil {
		glog.Errorf("Error VerifyImageSignatures %s/%s: %v", pod.Namespace, pod.Name, err)
		resp, err := json.Marshal(admissionResponse(403, false, "Failure", "Cosign image verification failed", &arRequest))
		if err != nil {
			glog.Errorf("Can't encode response: %v", err)
			http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
		}
		if _, err := w.Write(resp); err != nil {
			glog.Errorf("Can't write response: %v", err)
			http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		}
	} else {
		resp, err := json.Marshal(admissionResponse(200, true, "Success", "Cosign image verified", &arRequest))
		if err != nil {
			glog.Errorf("Can't encode response: %v", err)
			http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
		}
		if _, err := w.Write(resp); err != nil {
			glog.Errorf("Can't write OK response: %v", err)
			http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		}
	}
}

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
