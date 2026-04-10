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
	"encoding/json"
	"fmt"

	log "github.com/gookit/slog"
	"github.com/in-toto/in-toto-golang/in_toto"
	"github.com/sigstore/cosign/v3/pkg/cosign/attestation"
	"github.com/sigstore/cosign/v3/pkg/oci"
	"github.com/sigstore/cosign/v3/pkg/oci/static"
	"github.com/sigstore/protobuf-specs/gen/pb-go/dsse"
	"github.com/sigstore/sigstore/pkg/signature/payload"
)

// transformOutput transforms the output of cosign verification to a format that can be used by in-toto
// Ref: https://github.com/sigstore/cosign/blob/9b259ff6b690c0f0844893016cd23c2c250124f2/cmd/cosign/cli/verify/verify.go#L263
func transformOutput(verified []oci.Signature, name string) (verifiedOutput []oci.Signature, err error) {
	for _, v := range verified {
		dssePayload, err := v.Payload()
		if err != nil {
			return nil, err
		}
		var dsseEnvelope dsse.Envelope
		err = json.Unmarshal(dssePayload, &dsseEnvelope)
		if err != nil {
			return nil, err
		}
		if dsseEnvelope.PayloadType != in_toto.PayloadType {
			return nil, fmt.Errorf("unable to understand payload type %s", dsseEnvelope.PayloadType)
		}
		intotoStatement := &attestation.Statement{}
		err = intotoStatement.UnmarshalJSON(dsseEnvelope.Payload)
		if err != nil {
			return nil, err
		}
		if len(intotoStatement.Subject) < 1 || len(intotoStatement.Subject[0].Digest) < 1 {
			return nil, fmt.Errorf("no intoto subject or digest found")
		}

		var digest string
		for k, v := range intotoStatement.Subject[0].Digest {
			digest = k + ":" + v
		}
		annotations := intotoStatement.Subject[0].Annotations.AsMap()

		sci := payload.SimpleContainerImage{
			Critical: payload.Critical{
				Identity: payload.Identity{
					DockerReference: name,
				},
				Image: payload.Image{
					DockerManifestDigest: digest,
				},
				Type: intotoStatement.PredicateType,
			},
			Optional: annotations,
		}
		p, err := json.Marshal(sci)
		if err != nil {
			return nil, err
		}
		att, err := static.NewAttestation(p)
		if err != nil {
			return nil, err
		}
		verifiedOutput = append(verifiedOutput, att)
	}

	return verifiedOutput, nil
}

// logVerifiedPayloads logs the verified payloads for debugging purposes.
func logVerifiedPayloads(verified []oci.Signature, refName, image string) {
	verifiedOutput, err := transformOutput(verified, refName)
	if err != nil {
		log.Debugf("Error transforming output: %v", err)
	}
	if err == nil {
		verified = verifiedOutput
	}
	for _, sig := range verified {
		p, err := sig.Payload()
		if err != nil {
			log.Debugf("Error fetching payload for image %q: %v", image, err)
			continue
		}
		log.Debugf("Verified payload for image %q: %s", image, string(p))
	}
}
