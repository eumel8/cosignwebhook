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
		"OneContainerWIthCosignRepository":          testOneContainerWithCosignRepository,
	}

	fw, err := framework.New(t)
	if err != nil {
		t.Fatal(err)
	}

	for name, tf := range testFuncs {
		t.Run(fmt.Sprintf("[%s] %s", "ECDSA", name), tf(fw, framework.CreateECDSAKeyPair, name))
		t.Run(fmt.Sprintf("[%s] %s", "RSA", name), tf(fw, framework.CreateRSAKeyPair, name))
	}
}

// TestFailingDeployments tests deployments that should fail signature verification
func TestFailingDeployments(t *testing.T) {
	testFuncs := map[string]func(fw *framework.Framework, kf framework.KeyFunc, key string) func(t *testing.T){
		"OneContainerSinglePubKeyMalformedEnvRef":   testOneContainerSinglePubKeyMalformedEnvRef,
		"TwoContainersSinglePubKeyMalformedEnvRef":  testTwoContainersSinglePubKeyMalformedEnvRef,
		"OneContainerSinglePubKeyNoMatchEnvRef":     testOneContainerSinglePubKeyNoMatchEnvRef,
		"OneContainerWithCosingRepoVariableMissing": testOneContainerWithCosingRepoVariableMissing,
	}

	fw, err := framework.New(t)
	if err != nil {
		t.Fatal(err)
	}

	for name, tf := range testFuncs {
		t.Run(name, tf(fw, framework.CreateECDSAKeyPair, name))
		t.Run(name, tf(fw, framework.CreateRSAKeyPair, name))
	}
}
