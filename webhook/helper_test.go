//
// Copyright 2021 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webhook

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/sigstore/cosign/v3/pkg/oci"
	"github.com/sigstore/cosign/v3/pkg/oci/static"
	"github.com/sigstore/sigstore/pkg/signature/payload"
	"github.com/stretchr/testify/assert"
)

// Ref: https://github.com/sigstore/cosign/blob/9b259ff6b690c0f0844893016cd23c2c250124f2/cmd/cosign/cli/verify/verify_test.go#L403-L452
func TestTransformOutputSuccess(t *testing.T) {
	// Build minimal in-toto statement
	stmt := `{
	  "_type": "https://in-toto.io/Statement/v0.1",
	  "subject": [
		{ "name": "artifact", "digest": { "sha256": "deadbeef" }, "annotations": { "foo": "bar" } }
	  ],
	  "predicateType": "https://slsa.dev/provenance/v0.2"
	}`
	// DSSE payloadType for in-toto
	payloadType := "application/vnd.in-toto+json"
	encodedStmt := base64.StdEncoding.EncodeToString([]byte(stmt))
	dsseEnv := fmt.Sprintf(`{
	  "payloadType": "%s",
	  "payload": "%s",
	  "signatures": [
		{ "keyid": "test", "sig": "MAo=" }
	  ]
	}`, payloadType, encodedStmt)

	sig, err := static.NewSignature([]byte(dsseEnv), "")
	if err != nil {
		t.Fatalf("creating static signature: %v", err)
	}
	fmt.Println(dsseEnv)

	name := "example.com/my/image"
	out, err := transformOutput([]oci.Signature{sig}, name)
	if err != nil {
		t.Fatalf("transformOutput returned error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 transformed signature, got %d", len(out))
	}

	payloadBytes, err := out[0].Payload()
	if err != nil {
		t.Fatalf("reading transformed payload: %v", err)
	}

	var sci payload.SimpleContainerImage
	if err := json.Unmarshal(payloadBytes, &sci); err != nil {
		t.Fatalf("unmarshal transformed payload: %v", err)
	}

	assert.Equal(t, name, sci.Critical.Identity.DockerReference, "docker reference mismatch")
	assert.Equal(t, "sha256:deadbeef", sci.Critical.Image.DockerManifestDigest, "digest mismatch")
	assert.Equal(t, "https://slsa.dev/provenance/v0.2", sci.Critical.Type, "type mismatch")
	assert.Equal(t, map[string]any{"foo": "bar"}, sci.Optional, "missing annotation")
}
