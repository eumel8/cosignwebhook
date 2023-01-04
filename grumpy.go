package main

import (
	"context"
	"crypto"
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

	//"github.com/sigstore/sigstore/pkg/signature"
	// "github.com/sigstore/cosign/pkg/signature"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
)

const (
	cosignAnnotation = "caas.telekom.de/cosign"
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
	if pod.Name == "smooth-app" {
		return
	}

	arResponse := v1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AdmissionReview",
			APIVersion: "admission.k8s.io/v1",
		},
		Response: &v1.AdmissionResponse{
			Allowed: false,
			UID:     arRequest.Request.UID,
			Result: &metav1.Status{
				Status:  "Failure",
				Message: "Keep calm and not add more crap in the cluster!",
				Code:    403,
			},
		},
	}
	resp, err := json.Marshal(arResponse)

	if err != nil {
		glog.Errorf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}

	glog.Info("Annotation loop1: ", pod.DeepCopy())
	annotations := make(map[string]string)
	for k, v := range pod.Annotations {
		annotations[k] = v

		glog.Info("Annotation loop2: ", pod.GetAnnotations())
	}

	image := pod.Spec.Containers[0].Image
	// refImage := name.Reference{name.Tag.String(image)}
	refImage, err := name.ParseReference(image)
	/*
				imagePullSecrets := make([]string, 0, len(wp.Spec.Template.Spec.ImagePullSecrets))
			for _, s := range pod.Spec.Template.Spec.ImagePullSecrets {
				imagePullSecrets = append(imagePullSecrets, s.Name)
			}

		cosignPubKey := []byte(annotations["kubernetes.io/psp"])
	*/
	glog.Info("Annotation: ", pod.Annotations["caas.telekom.de/cosign"])
	cosignPubKey := []byte(annotations["caas.telekom.de/cosign"])
	// cosignLoadKey, err := signature.LoadPublicKey(context.Background(), cosignPubKey)
	// cosignLoadKey, err := signature.LoadVerifier(cosignPubKey, crypto.SHA256)

	unmarshalPubKey, err := cryptoutils.UnmarshalPEMToPublicKey(cosignPubKey)
	if err != nil {
		glog.Errorf("Error UnmarshalPEMToPublicKey: %v", err)
	}
	cosignLoadKey, err := signature.LoadVerifier(unmarshalPubKey, crypto.SHA256)
	if err != nil {
		glog.Errorf("Error LoadPublicKey: %v", err)
	}

	cosignVerify, bundleVerified, err := cosign.VerifyImageSignatures(context.Background(),
		refImage,
		&cosign.CheckOpts{
			SigVerifier: cosignLoadKey,
			//IgnoreSCT:      true,
			//SkipTlogVerify: true,
		})

	// SigVerifier:    signature.Verifier{signature.PublicKeyProvider: unmarshalPubKey},
	glog.Info("Resp object ", cosignVerify, bundleVerified)
	if err != nil {
		glog.Errorf("Error VerifyImageSignatures: %v", err)
	}
	glog.Infof("Ready to write reponse ...")
	if _, err := w.Write(resp); err != nil {
		glog.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}
