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

package vfsclientset

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/apis/kops/registry"
	kopsinternalversion "k8s.io/kops/pkg/client/clientset_generated/clientset/typed/kops/internalversion"
	"k8s.io/kops/pkg/client/simple"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/secrets"
	"k8s.io/kops/util/pkg/vfs"
)

type VFSClientset struct {
	basePath vfs.Path
}

var _ simple.Clientset = &VFSClientset{}

func (c *VFSClientset) ClustersFor(cluster *kops.Cluster) kopsinternalversion.ClusterInterface {
	return c.clusters()
}

func (c *VFSClientset) clusters() *ClusterVFS {
	return newClusterVFS(c.basePath)
}

func (c *VFSClientset) GetCluster(name string) (*kops.Cluster, error) {
	return c.clusters().Get(name, metav1.GetOptions{})
}

func (c *VFSClientset) ListClusters(options metav1.ListOptions) (*kops.ClusterList, error) {
	return c.clusters().List(options)
}

func (c *VFSClientset) ConfigBaseFor(cluster *kops.Cluster) (vfs.Path, error) {
	if cluster.Spec.ConfigBase != "" {
		return vfs.Context.BuildVfsPath(cluster.Spec.ConfigBase)
	}
	return c.clusters().configBase(cluster.Name)
}

func (c *VFSClientset) InstanceGroupsFor(cluster *kops.Cluster) kopsinternalversion.InstanceGroupInterface {
	clusterName := cluster.Name
	return newInstanceGroupVFS(c, clusterName)
}

func (c *VFSClientset) federations() kopsinternalversion.FederationInterface {
	return newFederationVFS(c)
}

func (c *VFSClientset) FederationsFor(federation *kops.Federation) kopsinternalversion.FederationInterface {
	return c.federations()
}

func (c *VFSClientset) ListFederations(options metav1.ListOptions) (*kops.FederationList, error) {
	return c.federations().List(options)
}

func (c *VFSClientset) GetFederation(name string) (*kops.Federation, error) {
	return c.federations().Get(name, metav1.GetOptions{})
}

func (c *VFSClientset) SecretStore(cluster *kops.Cluster) (fi.SecretStore, error) {
	configBase, err := registry.ConfigBase(cluster)
	if err != nil {
		return nil, err
	}
	basedir := configBase.Join("secrets")
	return secrets.NewVFSSecretStore(basedir), nil
}

func (c *VFSClientset) KeyStore(cluster *kops.Cluster) (fi.CAStore, error) {
	configBase, err := registry.ConfigBase(cluster)
	if err != nil {
		return nil, err
	}
	basedir := configBase.Join("pki")
	return fi.NewVFSCAStore(basedir), nil
}

func DeleteAllClusterState(basePath vfs.Path) error {
	paths, err := basePath.ReadTree()
	if err != nil {
		return fmt.Errorf("error listing files in state store: %v", err)
	}

	for _, path := range paths {
		relativePath, err := vfs.RelativePath(basePath, path)
		if err != nil {
			return err
		}
		if relativePath == "config" || relativePath == "cluster.spec" {
			continue
		}
		if strings.HasPrefix(relativePath, "addons/") {
			continue
		}
		if strings.HasPrefix(relativePath, "pki/") {
			continue
		}
		if strings.HasPrefix(relativePath, "secrets/") {
			continue
		}
		if strings.HasPrefix(relativePath, "instancegroup/") {
			continue
		}

		return fmt.Errorf("refusing to delete: unknown file found: %s", path)
	}

	for _, path := range paths {
		err = path.Remove()
		if err != nil {
			return fmt.Errorf("error deleting cluster file %s: %v", path, err)
		}
	}

	return nil
}

func (c *VFSClientset) DeleteCluster(cluster *kops.Cluster) error {
	configBase, err := registry.ConfigBase(cluster)
	if err != nil {
		return err
	}

	return DeleteAllClusterState(configBase)
}

func NewVFSClientset(basePath vfs.Path) simple.Clientset {
	vfsClientset := &VFSClientset{
		basePath: basePath,
	}
	return vfsClientset
}
