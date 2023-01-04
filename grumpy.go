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

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sigstore/cosign/v2/pkg/cosign"

	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
)

const (
	admissionApi          = "admission.k8s.io/v1"
	admissionKind         = "AdmissionReview"
	admissionStatusNotOK  = "Failure"
	admissionMessageNotOK = "Cosign image verification failed"
	admissionCodeNotOK    = 403
	admissionStatusOK     = "Success"
	admissionMessageOK    = "Cosign image verified"
	admissionCodeOK       = 200
	cosignEnvVar          = "COSIGNPUBKEY"
)

// GrumpyServerHandler listen to admission requests and serve responses
// build certs here: https://raw.githubusercontent.com/openshift/external-dns-operator/fb77a3c547a09cd638d4e05a7b8cb81094ff2476/hack/generate-certs.sh
// generate-certs.sh --service grumpy --webhook grumpy --namespace grumpy --secret grumpy
type GrumpyServerHandler struct {
}

func (gs *GrumpyServerHandler) serve(w http.ResponseWriter, r *http.Request) {
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
	}

	raw := arRequest.Request.Object.Raw
	pod := corev1.Pod{}
	if err := json.Unmarshal(raw, &pod); err != nil {
		glog.Error("error deserializing pod")
		return
	}

	// AdmissionReview definition
	arResponseNotOK := v1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       admissionKind,
			APIVersion: admissionApi,
		},
		Response: &v1.AdmissionResponse{
			Allowed: false,
			UID:     arRequest.Request.UID,
			Result: &metav1.Status{
				Status:  admissionStatusNotOK,
				Message: admissionMessageNotOK,
				Code:    admissionCodeNotOK,
			},
		},
	}
	arResponseOK := v1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       admissionKind,
			APIVersion: admissionApi,
		},
		Response: &v1.AdmissionResponse{
			Allowed: true,
			UID:     arRequest.Request.UID,
			Result: &metav1.Status{
				Status:  admissionStatusOK,
				Message: admissionMessageOK,
				Code:    admissionCodeOK,
			},
		},
	}
	respNotOK, err := json.Marshal(arResponseNotOK)

	if err != nil {
		glog.Errorf("Can't encode NotOK response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode NotOK response: %v", err), http.StatusInternalServerError)
	}

	respOK, err := json.Marshal(arResponseOK)

	if err != nil {
		glog.Errorf("Can't encode OK response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode OK response: %v", err), http.StatusInternalServerError)
	}

	pubKey := ""
	for i := 0; i < len(pod.Spec.Containers[0].Env); i++ {
		value := pod.Spec.Containers[0].Env[i].Value
		if pod.Spec.Containers[0].Env[i].Name == cosignEnvVar {
			pubKey = value
		}
	}

	image := pod.Spec.Containers[0].Image
	refImage, err := name.ParseReference(image)

	if err != nil {
		glog.Errorf("Error ParseRef image: %v", err)
	}
	/*
			imagePullSecrets := make([]string, 0, len(wp.Spec.Template.Spec.ImagePullSecrets))
		for _, s := range pod.Spec.Template.Spec.ImagePullSecrets {
			imagePullSecrets = append(imagePullSecrets, s.Name)
		}


	*/

	publicKey, err := cryptoutils.UnmarshalPEMToPublicKey([]byte(pubKey))
	if err != nil {
		glog.Errorf("Error UnmarshalPEMToPublicKey %s/%s: %v", pod.Namespace, pod.Name, err)
	}

	cosignLoadKey, err := signature.LoadECDSAVerifier(publicKey.(*ecdsa.PublicKey), crypto.SHA256)
	if err != nil {
		glog.Errorf("Error LoadECDSAVerifier %s/%s:: %v", pod.Namespace, pod.Name, err)
	}

	_, bundleVerified, err := cosign.VerifyImageSignatures(context.Background(),
		refImage,
		&cosign.CheckOpts{
			SigVerifier: cosignLoadKey,
			// add settings for cosign 2.0
			//IgnoreSCT:      true,
			//SkipTlogVerify: true,
		})

	glog.Info("Resp bundleVerified: ", bundleVerified)

	// Verify Image failed, needs to reject pod start
	if err != nil {
		glog.Errorf("Error VerifyImageSignatures %s/%s: %v", pod.Namespace, pod.Name, err)
		if _, err := w.Write(respNotOK); err != nil {
			glog.Errorf("Can't write NotOK response: %v", err)
			http.Error(w, fmt.Sprintf("could not write NotOK response: %v", err), http.StatusInternalServerError)
		}
	} else {
		if _, err := w.Write(respOK); err != nil {
			glog.Errorf("Can't write OK response: %v", err)
			http.Error(w, fmt.Sprintf("could not write OK response: %v", err), http.StatusInternalServerError)
		}
		return
	}
}
