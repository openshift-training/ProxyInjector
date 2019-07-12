package kube

import (
	apps "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ResourceMap are resources from where changes are going to be detected
var ResourceMap = map[string]runtime.Object{
	"deployments":  &apps.Deployment{},
	"daemonsets":   &apps.DaemonSet{},
	"statefulsets": &apps.StatefulSet{},
}
