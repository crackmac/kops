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

package main

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"io"
	"k8s.io/kops/cmd/kops/util"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/upup/pkg/fi/cloudup"
	"k8s.io/kops/upup/pkg/kutil"
)

type ToolboxDumpOptions struct {
	Output string

	ClusterName string
}

func (o *ToolboxDumpOptions) InitDefaults() {
	o.Output = OutputYaml
}

func NewCmdToolboxDump(f *util.Factory, out io.Writer) *cobra.Command {
	options := &ToolboxDumpOptions{}
	options.InitDefaults()

	cmd := &cobra.Command{
		Use:   "dump",
		Short: "Dump information about a cluster",
		Run: func(cmd *cobra.Command, args []string) {
			if err := rootCommand.ProcessArgs(args); err != nil {
				exitWithError(err)
			}

			options.ClusterName = rootCommand.ClusterName()

			err := RunToolboxDump(f, out, options)
			if err != nil {
				exitWithError(err)
			}
		},
	}

	// TODO: Push up to top-level command?
	cmd.Flags().StringVarP(&options.Output, "output", "o", options.Output, "output format.  One of: yaml, json")

	return cmd
}

func RunToolboxDump(f *util.Factory, out io.Writer, options *ToolboxDumpOptions) error {
	clientset, err := f.Clientset()
	if err != nil {
		return err
	}

	if options.ClusterName == "" {
		return fmt.Errorf("ClusterName is required")
	}

	cluster, err := clientset.Clusters().Get(options.ClusterName)
	if err != nil {
		return err
	}

	if cluster == nil {
		return fmt.Errorf("cluster not found %q", options.ClusterName)
	}

	cloud, err := cloudup.BuildCloud(cluster)
	if err != nil {
		return err
	}

	d := &kutil.DeleteCluster{}
	d.ClusterName = options.ClusterName
	d.Cloud = cloud

	resources, err := d.ListResources()
	if err != nil {
		return err
	}

	data := make(map[string]interface{})

	switch options.Output {
	case OutputYaml:
		dumpedResources := []interface{}{}
		for k, r := range resources {
			if r.Dumper == nil {
				glog.V(8).Infof("skipping dump of %q (no Dumper)", k)
				continue
			}

			o, err := r.Dumper(r)
			if err != nil {
				return fmt.Errorf("error dumping %q: %v", k, err)
			}
			if o != nil {
				dumpedResources = append(dumpedResources, o)
			}
		}
		data["resources"] = dumpedResources
		b, err := kops.ToRawYaml(data)
		if err != nil {
			return fmt.Errorf("error marshaling yaml: %v", err)
		}
		_, err = out.Write(b)
		if err != nil {
			return fmt.Errorf("error writing to stdout: %v", err)
		}
		return nil

	default:
		return fmt.Errorf("Unsupported output format: %q", options.Output)
	}

	return nil
}