package test

import (
	"fmt"
	"testing"

	"github.com/eumel8/cosignwebhook/test/framework"
)

// TestPassECDSA tests deployments that should pass signature verification
func TestPassECDSA(t *testing.T) {
	testFuncs := map[string]func(fw *framework.Framework, kf framework.KeyFunc, key string) func(t *testing.T){
		"OneContainerSinglePubKeyEnvRef":            oneContainerSinglePubKeyEnvRef,
		"TwoContainersSinglePubKeyEnvRef":           testTwoContainersSinglePubKeyEnvRef,
		"OneContainerSinglePubKeySecretRef":         testOneContainerSinglePubKeySecretRef,
		"TwoContainersSinglePubKeyMixedRef":         testTwoContainersSinglePubKeyMixedRef,
		"TwoContainersMixedPubKeyMixedRef":          testTwoContainersMixedPubKeyMixedRef,
		"TwoContainersSingleWithInitPubKeyMixedRef": testTwoContainersWithInitSinglePubKeyMixedRef,
		"EventEmittedOnSignatureVerification":       testEventEmittedOnSignatureVerification,
		"EventEmittedOnNoSignatureVerification":     testEventEmittedOnNoSignatureVerification,
		"OneContainerWithCosignRepository":          testOneContainerWithCosignRepository,
		"OneContainerLegacySigDigest":               testOneContainerLegacySigDigest,
		"OneContainerBothSignatureFormats":          testOneContainerBothSignatureFormats,
	}

	for name, tf := range testFuncs {
		name := name
		tf := tf
		t.Run(fmt.Sprintf("[%s] %s", "ECDSA", name), func(t *testing.T) {
			fw, err := framework.New(t)
			if err != nil {
				t.Fatal(err)
			}
			tf(fw, framework.CreateECDSAKeyPair, name)(t)
		})
		t.Run(fmt.Sprintf("[%s] %s", "RSA", name), func(t *testing.T) {
			fw, err := framework.New(t)
			if err != nil {
				t.Fatal(err)
			}
			tf(fw, framework.CreateRSAKeyPair, name)(t)
		})
	}
}

// TestFailingDeployments tests deployments that should fail signature verification
func TestFailingDeployments(t *testing.T) {
	testFuncs := map[string]func(fw *framework.Framework, kf framework.KeyFunc, key string) func(t *testing.T){
		"OneContainerSinglePubKeyMalformedEnvRef":   testOneContainerSinglePubKeyMalformedEnvRef,
		"TwoContainersSinglePubKeyMalformedEnvRef":  testTwoContainersSinglePubKeyMalformedEnvRef,
		"OneContainerSinglePubKeyNoMatchEnvRef":     testOneContainerSinglePubKeyNoMatchEnvRef,
		"OneContainerWithCosignRepoVariableMissing": testOneContainerWithCosignRepoVariableMissing,
		"OneContainerMalformedDockerconfigjson":     testOneContainerMalformedDockerconfigjson,
	}

	for name, tf := range testFuncs {
		name := name
		tf := tf
		t.Run(fmt.Sprintf("[%s] %s", "ECDSA", name), func(t *testing.T) {
			fw, err := framework.New(t)
			if err != nil {
				t.Fatal(err)
			}
			tf(fw, framework.CreateECDSAKeyPair, name)(t)
		})
		t.Run(fmt.Sprintf("[%s] %s", "RSA", name), func(t *testing.T) {
			fw, err := framework.New(t)
			if err != nil {
				t.Fatal(err)
			}
			tf(fw, framework.CreateRSAKeyPair, name)(t)
		})
	}
}
