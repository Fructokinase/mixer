// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package golden

import (
	"context"
	"path"
	"runtime"
	"testing"

	pbs "github.com/datacommonsorg/mixer/internal/proto/service"
	pbv1 "github.com/datacommonsorg/mixer/internal/proto/v1"
	"github.com/datacommonsorg/mixer/test"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestBulkVariableInfo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	_, filename, _, _ := runtime.Caller(0)
	goldenPath := path.Join(path.Dir(filename), "bulk_variable_info")

	testSuite := func(mixer pbs.MixerClient, latencyTest bool) {
		for _, c := range []struct {
			nodes      []string
			goldenFile string
			wantErr    bool
		}{
			{
				[]string{
					"UnemploymentRate_Person",
					"Count_Person_Female",
					"Count_Person_Female_AsianAlone",
					"FertilityRate_Person_Female",
					"IncrementalCount_MedicalConditionIncident_COVID_19_ConfirmedOrProbableCase",
				},
				"bulk_result.json",
				false,
			},
			{
				[]string{},
				"empty.json",
				true,
			},
		} {
			resp, err := mixer.BulkVariableInfo(ctx, &pbv1.BulkVariableInfoRequest{
				Nodes: c.nodes,
			})
			if c.wantErr {
				if err == nil {
					t.Errorf("Expect to get error for BulkVariableInfo() but succeed")
				}
				continue
			}
			if err != nil {
				t.Errorf("BulkVariableInfo() = %s", err)
				continue
			}

			if latencyTest {
				continue
			}

			if test.GenerateGolden {
				test.UpdateProtoGolden(resp, goldenPath, c.goldenFile)
				continue
			}

			var expected pbv1.BulkVariableInfoResponse
			if err = test.ReadJSON(goldenPath, c.goldenFile, &expected); err != nil {
				t.Errorf("Can not Unmarshal golden file")
				continue
			}

			if diff := cmp.Diff(resp, &expected, protocmp.Transform()); diff != "" {
				t.Errorf("payload got diff: %v", diff)
				continue
			}
		}
	}

	if err := test.TestDriver(
		"BulkVariableInfo", &test.TestOption{}, testSuite); err != nil {
		t.Errorf("TestDriver() = %s", err)
	}
}
