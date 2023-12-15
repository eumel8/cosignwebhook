package test

import (
	"testing"
)

// TestPassingDeployments tests deployments that should pass signature verification
func TestPassingDeployments(t *testing.T) {

	testFuncs := map[string]func(t *testing.T){
		"OneContainerSinglePubKeyEnvRef":            testOneContainerSinglePubKeyEnvRef,
		"TwoContainersSinglePubKeyEnvRef":           testTwoContainersSinglePubKeyEnvRef,
		"OneContainerSinglePubKeySecretRef":         testOneContainerSinglePubKeySecretRef,
		"TwoContainersSinglePubKeyMixedRef":         testTwoContainersSinglePubKeyMixedRef,
		"TwoContainersMixedPubKeyMixedRef":          testTwoContainersMixedPubKeyMixedRef,
		"TwoContainersSingleWithInitPubKeyMixedRef": testTwoContainersWithInitSinglePubKeyMixedRef,
		"EventEmittedOnSignatureVerification":       testEventEmittedOnSignatureVerification,
		"EventEmittedOnNoSignatureVerification":     testEventEmittedOnNoSignatureVerification,
	}

	for name, tf := range testFuncs {
		t.Run(name, tf)
	}
}

// TestFailingDeployments tests deployments that should fail signature verification
func TestFailingDeployments(t *testing.T) {

	testFuncs := map[string]func(t *testing.T){
		"OneContainerSinglePubKeyMalformedEnvRef":  testOneContainerSinglePubKeyMalformedEnvRef,
		"TwoContainersSinglePubKeyMalformedEnvRef": testTwoContainersSinglePubKeyMalformedEnvRef,
		"OneContainerSinglePubKeyNoMatchEnvRef":    testOneContainerSinglePubKeyNoMatchEnvRef,
	}

	for name, tf := range testFuncs {
		t.Run(name, tf)
	}
}
