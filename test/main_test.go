package test

import (
	"os"
	"testing"
)

func TestDeployments(t *testing.T) {

	if os.Getenv("SKIP_TEST_DEPLOYMENTS") != "" {
		t.Skip("Skipping TestDeployments")
	}

	testFuncs := map[string]func(t *testing.T){
		"OneContainerPubKeyEnvVar": testOneContainerPubKeyEnvVar,
	}

	for name, tf := range testFuncs {
		t.Run(name, tf)
	}
}
