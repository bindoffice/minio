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
	"net"

	"github.com/bindoffice/bind-store/cmd/config"
	"github.com/bindoffice/bind-store/cmd/logger"
	"github.com/bindoffice/bind-store/pkg/auth"
	"github.com/bindoffice/bind-store/pkg/env"
	xnet "github.com/bindoffice/bind-store/pkg/net"
)

// isWildcardListenAddr reports whether serverAddr listens on every local interface
// (e.g. ":9000", "0.0.0.0:9000", "[::]:9000").
func isWildcardListenAddr(serverAddr string) bool {
	host, err := xnet.ParseHost(serverAddr)
	if err != nil {
		return false
	}
	if host.Name == "" {
		return true
	}
	return host.Name == net.IPv4zero.String() || host.Name == net.IPv6zero.String()
}

func envFlagEnabled(key string) bool {
	v := env.Get(key, "")
	if v == "" {
		return false
	}
	enabled, err := config.ParseBool(v)
	return err == nil && enabled
}

// validatePublicBind returns an error when the API would listen on all interfaces
// without an explicit operator opt-in.
func validatePublicBind(serverAddr string) error {
	if !isWildcardListenAddr(serverAddr) {
		return nil
	}
	if envFlagEnabled(config.EnvAllowPublicBind) {
		return nil
	}
	return config.ErrPublicBindNotAllowed(nil)
}

// checkPublicBind aborts startup when validatePublicBind fails.
func checkPublicBind(serverAddr string) {
	if err := validatePublicBind(serverAddr); err != nil {
		logger.Fatal(err,
			"Refusing to listen on all interfaces ("+serverAddr+"); use --address 127.0.0.1:9000 for local dev or set MINIO_ALLOW_PUBLIC_BIND=on when network access is restricted")
	}
	if isWildcardListenAddr(serverAddr) {
		logger.StartupMessage("WARNING: API is listening on all interfaces (" + serverAddr + "). Restrict network access with a firewall or bind to a private address.")
	}
}

// validateDefaultCredentials returns an error when root still uses factory defaults.
func validateDefaultCredentials() error {
	if !globalActiveCred.Equal(auth.DefaultCredentials) {
		return nil
	}
	if envFlagEnabled(config.EnvAllowDefaultCredentials) {
		return nil
	}
	return config.ErrDefaultCredentialsNotAllowed(nil)
}

// checkDefaultCredentials aborts startup when validateDefaultCredentials fails.
func checkDefaultCredentials() {
	if err := validateDefaultCredentials(); err != nil {
		logger.Fatal(err,
			"Refusing to start with default credentials '"+auth.DefaultAccessKey+"'; set MINIO_ROOT_USER and MINIO_ROOT_PASSWORD")
	}
	if globalActiveCred.Equal(auth.DefaultCredentials) {
		logger.StartupMessage("WARNING: default root credentials are in use; set MINIO_ROOT_USER and MINIO_ROOT_PASSWORD before production deployment")
	}
}
