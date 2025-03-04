//go:build detectors
// +build detectors

package aws

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/trufflesecurity/trufflehog/v3/pkg/common"
	"github.com/trufflesecurity/trufflehog/v3/pkg/detectors"
	"github.com/trufflesecurity/trufflehog/v3/pkg/pb/detectorspb"
)

func TestAWS_FromChunk(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	testSecrets, err := common.GetSecret(ctx, "trufflehog-testing", "detectors4")
	if err != nil {
		t.Fatalf("could not get test secrets from GCP: %s", err)
	}
	secret := testSecrets.MustGetField("AWS")
	id := testSecrets.MustGetField("AWS_ID")
	inactiveSecret := testSecrets.MustGetField("AWS_INACTIVE")
	inactiveID := id[:len(id)-3] + "XYZ"
	hasher := sha256.New()
	hasher.Write([]byte(inactiveSecret))
	hash := string(hasher.Sum(nil))

	type args struct {
		ctx    context.Context
		data   []byte
		verify bool
	}
	tests := []struct {
		name                  string
		s                     scanner
		args                  args
		want                  []detectors.Result
		wantErr               bool
		wantVerificationError bool
	}{
		{
			name: "found, verified",
			s:    scanner{},
			args: args{
				ctx:    context.Background(),
				data:   []byte(fmt.Sprintf("You can find a aws secret %s within aws %s", secret, id)),
				verify: true,
			},
			want: []detectors.Result{
				{
					DetectorType: detectorspb.DetectorType_AWS,
					Verified:     true,
					Redacted:     "AKIASP2TPHJSQH3FJRUX",
					ExtraData: map[string]string{
						"account": "171436882533",
						"arn":     "arn:aws:iam::171436882533:user/canarytokens.com@@4dxkh0pdeop3bzu9zx5wob793",
						"user_id": "AIDASP2TPHJSUFRSTTZX4",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "found, unverified",
			s:    scanner{},
			args: args{
				ctx:    context.Background(),
				data:   []byte(fmt.Sprintf("You can find a aws secret %s within aws %s but not valid", inactiveSecret, id)), // the secret would satisfy the regex but not pass validation
				verify: true,
			},
			want: []detectors.Result{
				{
					DetectorType: detectorspb.DetectorType_AWS,
					Verified:     false,
					Redacted:     "AKIASP2TPHJSQH3FJRUX",
					ExtraData:    nil,
				},
			},
			wantErr: false,
		},
		{
			name: "not found",
			s:    scanner{},
			args: args{
				ctx:    context.Background(),
				data:   []byte("You cannot find the secret within"),
				verify: true,
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "found two, one included for every ID found",
			s:    scanner{},
			args: args{
				ctx:    context.Background(),
				data:   []byte(fmt.Sprintf("The verified ID is %s with a secret of %s, but the unverified ID is %s and this is the secret %s", id, secret, inactiveID, inactiveSecret)),
				verify: true,
			},
			want: []detectors.Result{
				{
					DetectorType: detectorspb.DetectorType_AWS,
					Verified:     false,
					Redacted:     "AKIASP2TPHJSQH3FJXYZ",
				},
				{
					DetectorType: detectorspb.DetectorType_AWS,
					Verified:     true,
					Redacted:     "AKIASP2TPHJSQH3FJRUX",
					ExtraData: map[string]string{
						"account": "171436882533",
						"arn":     "arn:aws:iam::171436882533:user/canarytokens.com@@4dxkh0pdeop3bzu9zx5wob793",
						"user_id": "AIDASP2TPHJSUFRSTTZX4",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "not found, because unverified secret was a hash",
			s:    scanner{},
			args: args{
				ctx:    context.Background(),
				data:   []byte(fmt.Sprintf("You can find a aws secret %s within aws %s but not valid", hash, id)), // The secret would satisfy the regex but be filtered out after not passing validation.
				verify: true,
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "found two, returned both because the active secret for one paired with the inactive ID, despite the hash",
			s:    scanner{},
			args: args{
				ctx:    context.Background(),
				data:   []byte(fmt.Sprintf("The verified ID is %s with a secret of %s, but the unverified ID is %s and the secret is this hash %s", id, secret, inactiveID, hash)),
				verify: true,
			},
			want: []detectors.Result{
				{
					DetectorType: detectorspb.DetectorType_AWS,
					Verified:     true,
					Redacted:     "AKIASP2TPHJSQH3FJRUX",
					ExtraData: map[string]string{
						"account": "171436882533",
						"arn":     "arn:aws:iam::171436882533:user/canarytokens.com@@4dxkh0pdeop3bzu9zx5wob793",
						"user_id": "AIDASP2TPHJSUFRSTTZX4",
					},
				},
				{
					DetectorType: detectorspb.DetectorType_AWS,
					Verified:     false,
					Redacted:     inactiveID,
				},
			},
			wantErr: false,
		},
		{
			name: "found, unverified, with leading +",
			s:    scanner{},
			args: args{
				ctx:    context.Background(),
				data:   []byte(fmt.Sprintf("You can find a aws secret %s within aws %s but not valid", "+HaNv9cTwheDKGJaws/+BMF2GgybQgBWdhcOOdfF", id)), // the secret would satisfy the regex but not pass validation
				verify: true,
			},
			want: []detectors.Result{
				{
					DetectorType: detectorspb.DetectorType_AWS,
					Verified:     false,
					Redacted:     "AKIASP2TPHJSQH3FJRUX",
				},
			},
			wantErr: false,
		},
		{
			name: "skipped",
			s: scanner{
				skipIDs: map[string]struct{}{
					"AKIASP2TPHJSQH3FJRUX": {},
				},
			},
			args: args{
				ctx:    context.Background(),
				data:   []byte(fmt.Sprintf("You can find a aws secret %s within aws %s but not valid", "+HaNv9cTwheDKGJaws/+BMF2GgybQgBWdhcOOdfF", id)), // the secret would satisfy the regex but not pass validation
				verify: true,
			},
			wantErr: false,
		},
		{
			name: "found, would be verified if not for http timeout",
			s:    scanner{},
			args: args{
				ctx:    timeoutContext(1 * time.Microsecond),
				data:   []byte(fmt.Sprintf("You can find a aws secret %s within aws %s", secret, id)),
				verify: true,
			},
			want: []detectors.Result{
				{
					DetectorType: detectorspb.DetectorType_AWS,
					Verified:     false,
					Redacted:     "AKIASP2TPHJSQH3FJRUX",
				},
			},
			wantErr:               false,
			wantVerificationError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.s
			got, err := s.FromData(tt.args.ctx, tt.args.verify, tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("AWS.FromData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for i := range got {
				if len(got[i].Raw) == 0 {
					t.Fatalf("no raw secret present: \n %+v", got[i])
				}
				if (got[i].VerificationError != nil) != tt.wantVerificationError {
					t.Fatalf("verification error = %v, wantVerificationError %v", got[i].VerificationError, tt.wantVerificationError)
				}
			}
			ignoreOpts := cmpopts.IgnoreFields(detectors.Result{}, "RawV2", "Raw", "VerificationError")
			if diff := cmp.Diff(got, tt.want, ignoreOpts); diff != "" {
				t.Errorf("AWS.FromData() %s diff: (-got +want)\n%s", tt.name, diff)
			}
		})
	}
}

func BenchmarkFromData(benchmark *testing.B) {
	ctx := context.Background()
	s := scanner{}
	for name, data := range detectors.MustGetBenchmarkData() {
		benchmark.Run(name, func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				_, err := s.FromData(ctx, false, data)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func timeoutContext(timeout time.Duration) context.Context {
	c, _ := context.WithTimeout(context.Background(), timeout)
	return c
}
