package protokube

import (
	"k8s.io/kubernetes/pkg/api/v1"
	"fmt"
	"strings"
	"k8s.io/kubernetes/pkg/util/intstr"
)

func BuildEtcdManifest(c *EtcdCluster) (*v1.Pod) {
	pod := &v1.Pod{}
	pod.APIVersion = "v1"
	pod.Kind = "Pod"
	pod.Name = c.PodName
	pod.Namespace = "kube-system"

	pod.Spec.HostNetwork = true

	{
		container := v1.Container{
			Name: "etcd-container",
			Image: "gcr.io/google_containers/etcd:2.2.1",
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceCPU: c.CPURequest,
				},
			},
			Command: []string{
				"/bin/sh",
				"-c",
				"/usr/local/bin/etcd 1>>/var/log/etcd.log 2>&1",
			},

			Env: []v1.EnvVar{
				{Name: "ETCD_NAME", Value: c.Me.Name },
				{Name: "ETCD_DATA_DIR", Value: "/var/etcd/" + c.DataDirName },
				{Name: "ETCD_LISTEN_PEER_URLS", Value: fmt.Sprintf("http://0.0.0.0:%d", c.PeerPort) },
				{Name: "ETCD_LISTEN_CLIENT_URLS", Value:  fmt.Sprintf("http://0.0.0.0:%d", c.ClientPort) },
				{Name: "ETCD_ADVERTISE_CLIENT_URLS", Value:  fmt.Sprintf("http://%s:%d", c.Me.InternalName, c.ClientPort) },
				{Name: "ETCD_INITIAL_ADVERTISE_PEER_URLS", Value:  fmt.Sprintf("http://%s:%d", c.Me.InternalName, c.PeerPort) },
				{Name: "ETCD_INITIAL_CLUSTER_STATE", Value:  "new" },
				{Name: "ETCD_INITIAL_CLUSTER_TOKEN", Value:  c.ClusterToken },
			},
		}

		var initialCluster []string
		for _, node := range c.Nodes {
			initialCluster = append(initialCluster, node.Name + "=" + fmt.Sprintf("http://%s:%d", node.InternalName, c.PeerPort))
		}
		container.Env = append(container.Env, v1.EnvVar{ Name: "ETCD_INITIAL_CLUSTER", Value: strings.Join(initialCluster, ",") })

		container.LivenessProbe = &v1.Probe{
			InitialDelaySeconds:600,
			TimeoutSeconds:15,
		}
		container.LivenessProbe.HTTPGet = &v1.HTTPGetAction{
			Host: "127.0.0.1",
			Port:  intstr.FromInt(c.ClientPort),
			Path: "/health",
		}

		container.Ports = append(container.Ports, v1.ContainerPort{
		Name:"serverport",
			ContainerPort: int32(c.PeerPort),
			HostPort: int32(c.PeerPort),
		})
		container.Ports = append(container.Ports, v1.ContainerPort{
			Name:"clientport",
			ContainerPort: int32(c.ClientPort),
			HostPort: int32(c.ClientPort),
		})

		container.VolumeMounts = append(container.VolumeMounts, v1.VolumeMount{
			Name: "varetcdata",
			MountPath: "/var/etcd/" + c.DataDirName,
			ReadOnly: false,
		})
		pod.Spec.Volumes = append(pod.Spec.Volumes, v1.Volume{
			Name: "varetcdata",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: c.VolumeMountPath + "/var/etcd/" + c.DataDirName,
				},
			},
		})

		container.VolumeMounts = append(container.VolumeMounts, v1.VolumeMount{
			Name: "varlogetcd",
			MountPath: "/var/log/etcd.log",
			ReadOnly: false,
		})
		pod.Spec.Volumes = append(pod.Spec.Volumes, v1.Volume{
			Name: "varlogetcd",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: c.LogFile,
				},
			},
		})


		pod.Spec.Containers = append(pod.Spec.Containers, container)
	}

	return pod
}


