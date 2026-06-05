/*
 * MinIO Cloud Storage, (C) 2018,2020 MinIO, Inc.
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

package dns

import (
	"path"

	"github.com/miekg/dns"
)

// etcdPath converts a domain name to an etcd key path compatible with CoreDNS
// SkyDNS layout. For example service.staging.skydns.local. with prefix skydns
// becomes /skydns/local/skydns/staging/service.
//
// Logic mirrors CoreDNS plugin/etcd/msg.Path (Apache 2.0, CoreDNS Authors).
func etcdPath(s, prefix string) string {
	l := dns.SplitDomainName(s)
	for i, j := 0, len(l)-1; i < j; i, j = i+1, j-1 {
		l[i], l[j] = l[j], l[i]
	}
	return path.Join(append([]string{"/" + prefix + "/"}, l...)...)
}
