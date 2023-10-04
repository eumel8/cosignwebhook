package test

import (
	"testing"
)

func TestPassingDeployments(t *testing.T) {

	testFuncs := map[string]func(t *testing.T){
		"OneContainerSinglePubKeyEnvRef":            testOneContainerSinglePubKeyEnvRef,
		"TwoContainersSinglePubKeyEnvRef":           testTwoContainersSinglePubKeyEnvRef,
		"OneContainerSinglePubKeySecretRef":         testOneContainerSinglePubKeySecretRef,
		"TwoContainersSinglePubKeyMixedRef":         testTwoContainersSinglePubKeyMixedRef,
		"TwoContainersMixedPubKeyMixedRef":          testTwoContainersMixedPubKeyMixedRef,
		"TwoContainersSingleWithInitPubKeyMixedRef": testTwoContainersWithInitSinglePubKeyMixedRef,
	}

	for name, tf := range testFuncs {
		t.Run(name, tf)
	}
}

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
