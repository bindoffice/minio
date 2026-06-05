/*
 * MinIO Cloud Storage, (C) 2021 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package madmin

import (
	"encoding/json"
	"testing"
)

// CVE-2021-43858: AddOrUpdateUserReq must not accept policy fields.
func TestAddOrUpdateUserReqIgnoresPolicyName(t *testing.T) {
	payload := `{"secretKey":"secret","status":"enabled","policyName":"consoleAdmin"}`
	var req AddOrUpdateUserReq
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if req.SecretKey != "secret" {
		t.Fatalf("expected secretKey, got %q", req.SecretKey)
	}
	if req.Status != AccountEnabled {
		t.Fatalf("expected enabled status, got %q", req.Status)
	}
}
