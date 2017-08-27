/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package components

import (
	"fmt"
	"strings"

	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/loader"

	"github.com/blang/semver"
	"github.com/golang/glog"
)

// KubeAPIServerOptionsBuilder adds options for the apiserver to the model
type KubeAPIServerOptionsBuilder struct {
	*OptionsContext
}

var _ loader.OptionsBuilder = &KubeAPIServerOptionsBuilder{}

// BuildOptions is resposible for filling in the default settings for the kube apiserver
func (b *KubeAPIServerOptionsBuilder) BuildOptions(o interface{}) error {
	clusterSpec := o.(*kops.ClusterSpec)
	if clusterSpec.KubeAPIServer == nil {
		clusterSpec.KubeAPIServer = &kops.KubeAPIServerConfig{}
	}
	c := clusterSpec.KubeAPIServer

	if c.APIServerCount == nil {
		count := b.buildAPIServerCount(clusterSpec)
		if count == 0 {
			return fmt.Errorf("no instance groups found")
		}
		c.APIServerCount = fi.Int32(int32(count))
	}

	// @question: should the question every be able to set this?
	if c.StorageBackend == nil {
		// @note: we can use the first version as we enforce both running the same versions.
		// albeit feels a little wierd to do this
		sem, err := semver.Parse(strings.TrimPrefix(clusterSpec.EtcdClusters[0].Version, "v"))
		if err != nil {
			return err
		}
		c.StorageBackend = fi.String(fmt.Sprintf("etcd%d", sem.Major))
	}

	if c.KubeletPreferredAddressTypes == nil {
		if b.IsKubernetesGTE("1.5") {
			// We prioritize the internal IP above the hostname
			c.KubeletPreferredAddressTypes = []string{
				string(v1.NodeInternalIP),
				string(v1.NodeHostName),
				string(v1.NodeExternalIP),
			}

			if b.IsKubernetesLT("1.7") {
				// NodeLegacyHostIP was removed in 1.7; we add it to prior versions with lowest precedence
				c.KubeletPreferredAddressTypes = append(c.KubeletPreferredAddressTypes, "LegacyHostIP")
			}
		}
	}

	if clusterSpec.Authentication != nil {
		if clusterSpec.Authentication.Kopeio != nil {
			c.AuthenticationTokenWebhookConfigFile = fi.String("/etc/kubernetes/authn.config")
		}
	}

	if clusterSpec.Authorization == nil || clusterSpec.Authorization.IsEmpty() {
		// Do nothing - use the default as defined by the apiserver
		// (this won't happen anyway because of our default logic)
	} else if clusterSpec.Authorization.AlwaysAllow != nil {
		clusterSpec.KubeAPIServer.AuthorizationMode = fi.String("AlwaysAllow")
	} else if clusterSpec.Authorization.RBAC != nil {
		var modes []string

		if b.IsKubernetesGTE("1.8") {
			// Enable the Node authorizer, used for special per-node access policies
			modes = append(modes, "Node")
		}
		modes = append(modes, "RBAC")

		clusterSpec.KubeAPIServer.AuthorizationMode = fi.String(strings.Join(modes, ","))
	}

	image, err := Image("kube-apiserver", clusterSpec, b.AssetBuilder)
	if err != nil {
		return err
	}
	c.Image = image

	c.SecurePort = 443

	// We disable the insecure port from 1.6 onwards
	if b.IsKubernetesGTE("1.6") {
		c.InsecurePort = 0
		glog.V(4).Infof("Enabling apiserver insecure port, for healthchecks (issue #43784)")
		c.InsecurePort = 8080
	} else {
		c.InsecurePort = 8080
	}

	return nil
}

// buildAPIServerCount calculates the count of the api servers, essentuially the number of node marked as Master role
func (b *KubeAPIServerOptionsBuilder) buildAPIServerCount(clusterSpec *kops.ClusterSpec) int {
	// The --apiserver-count flag is (generally agreed) to be something we need to get rid of in k8s

	// We should do something like this:

	//count := 0
	//for _, ig := range b.InstanceGroups {
	//	if !ig.IsMaster() {
	//		continue
	//	}
	//	size := fi.IntValue(ig.Spec.MaxSize)
	//	if size == 0 {
	//		size = fi.IntValue(ig.Spec.MinSize)
	//	}
	//	count += size
	//}

	// But if we do, we end up with a weird dependency on InstanceGroups.  We actually could tolerate
	// that in kops, but we don't really want to.

	// So instead, we assume that the etcd cluster size is the API Server Count.
	// We can re-examine this when we allow separate etcd clusters - at which time hopefully
	// the flag won't exist

	counts := make(map[string]int)
	for _, etcdCluster := range clusterSpec.EtcdClusters {
		counts[etcdCluster.Name] = len(etcdCluster.Members)
	}

	count := counts["main"]

	return count
}
