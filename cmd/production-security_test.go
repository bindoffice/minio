/*
 * MinIO Cloud Storage, (C) 2026 bindoffice
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

package cmd

import (
	"testing"

	"github.com/bindoffice/bind-store/cmd/config"
	"github.com/bindoffice/bind-store/pkg/auth"
)

func TestIsWildcardListenAddr(t *testing.T) {
	tests := []struct {
		addr string
		want bool
	}{
		{":9000", true},
		{"0.0.0.0:9000", true},
		{"[::]:9000", true},
		{"127.0.0.1:9000", false},
		{"[::1]:9000", false},
		{"10.0.0.5:9000", false},
		{"localhost:9000", false},
	}
	for _, tt := range tests {
		if got := isWildcardListenAddr(tt.addr); got != tt.want {
			t.Errorf("isWildcardListenAddr(%q) = %v, want %v", tt.addr, got, tt.want)
		}
	}
}

func TestValidatePublicBind(t *testing.T) {
	t.Setenv(config.EnvAllowPublicBind, "")
	if err := validatePublicBind(":9000"); err == nil {
		t.Fatal("expected error for wildcard bind without opt-in")
	}
	t.Setenv(config.EnvAllowPublicBind, config.EnableOn)
	if err := validatePublicBind(":9000"); err != nil {
		t.Fatalf("expected opt-in to allow wildcard bind, got %v", err)
	}
	if err := validatePublicBind("127.0.0.1:9000"); err != nil {
		t.Fatalf("loopback bind should be allowed, got %v", err)
	}
}

func TestValidateDefaultCredentials(t *testing.T) {
	prev := globalActiveCred
	t.Cleanup(func() { globalActiveCred = prev })

	globalActiveCred = auth.DefaultCredentials
	t.Setenv(config.EnvAllowDefaultCredentials, "")
	if err := validateDefaultCredentials(); err == nil {
		t.Fatal("expected error for default credentials without opt-in")
	}
	t.Setenv(config.EnvAllowDefaultCredentials, config.EnableOn)
	if err := validateDefaultCredentials(); err != nil {
		t.Fatalf("expected opt-in to allow default credentials, got %v", err)
	}
	globalActiveCred = auth.Credentials{AccessKey: "customuser", SecretKey: "custompass12"}
	if err := validateDefaultCredentials(); err != nil {
		t.Fatalf("custom credentials should be allowed, got %v", err)
	}
}
