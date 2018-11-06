package handler

import (
	"encoding/json"
	"errors"
	logger "github.com/sirupsen/logrus"
	"github.com/stakater/ProxyInjector/internal/pkg/callbacks"
	"github.com/stakater/ProxyInjector/internal/pkg/constants"
	"github.com/stakater/ProxyInjector/pkg/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

// ResourceCreatedHandler contains new objects
type ResourceCreatedHandler struct {
	Resource interface{} `json:"resource"`
}

type patch struct {
	Spec struct {
		Template struct {
			Spec struct {
				Containers []Container `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
}

type Container struct {
	Name  string   `json:"name"`
	Image string   `json:"image"`
	Args  []string `json:"args"`
}

// Handle processes the newly created resource
func (r ResourceCreatedHandler) Handle(conf []string, resourceType string) error {
	if r.Resource == nil {
		logger.Errorf("Resource creation handler received nil resource")
	} else {

		var name string
		var namespace string
		var annotations map[string]string

		if resourceType == "deployments" {
			name = callbacks.GetDeploymentName(r.Resource)
			namespace = callbacks.GetDeploymentNamespace(r.Resource)
			annotations = callbacks.GetDeploymentAnnotations(r.Resource)
		} else if resourceType == "daemonsets" {
			name = callbacks.GetDaemonsetName(r.Resource)
			namespace = callbacks.GetDaemonsetNamespace(r.Resource)
			annotations = callbacks.GetDaemonsetAnnotations(r.Resource)
		} else if resourceType == "statefulsets" {
			name = callbacks.GetStatefulsetName(r.Resource)
			namespace = callbacks.GetStatefulsetNamespace(r.Resource)
			annotations = callbacks.GetStatefulsetAnnotations(r.Resource)
		}
		logger.Infof("Resource creation handler checking resource %s of type %s in namespace %s", name, resourceType, namespace)

		if annotations[constants.EnabledAnnotation] == "true" {

			client, err := kube.GetClient()

			logger.Infof("Updating resource ... %s", name)

			containerArgs := conf

			for _, arg := range constants.KeycloakArgs {
				if annotations[constants.AnnotationPrefix+arg] != "" {
					containerArgs = append(containerArgs, "--"+arg+"="+annotations[constants.AnnotationPrefix+arg])
				}
			}

			if err == nil {
				payloadBytes, err3 := getPatch(containerArgs, annotations[constants.ImageNameAnnotation]+":"+annotations[constants.ImageTagAnnotation])

				if err3 == nil {

					var err2 error
					logger.Info("checking resource type and updating...")
					if resourceType == "deployments" {
						logger.Info("patching deployment")
						_, err2 = client.ExtensionsV1beta1().Deployments(namespace).Patch(name, types.StrategicMergePatchType, payloadBytes)
					} else if resourceType == "daemonsets" {
						logger.Info("patching daemonset")
						_, err2 = client.AppsV1beta2().DaemonSets(namespace).Patch(name, types.StrategicMergePatchType, payloadBytes)
					} else if resourceType == "statefulsets" {
						logger.Info("patching statefulset")
						_, err2 = client.AppsV1beta2().StatefulSets(namespace).Patch(name, types.StrategicMergePatchType, payloadBytes)
					} else {
						return errors.New("unexpected resource type")
					}

					if err2 == nil {
						logger.Infof("Updated resource... %s", name)
					} else {
						logger.Error(err2)
					}

					updateService(client, namespace, annotations[constants.SourceServiceNameAnnotation], annotations[constants.TargetPortAnnotation])

				} else {
					logger.Error(err3)
				}
			} else {
				logger.Error(err)
			}

		}
	}
	return nil
}

func getPatch(containerArgs []string, image string) ([]byte, error) {

	payload := &patch{}
	payload.Spec.Template.Spec.Containers = []Container{{
		Name:  "proxy",
		Image: image,
		Args:  containerArgs,
	}}

	return json.Marshal(payload)
}

func updateService(client *kubernetes.Clientset, namespace string, service string, port string) {

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// RetryOnConflict uses exponential backoff to avoid exhausting the apiserver
		result, getErr := client.CoreV1().Services(namespace).Get(service, metav1.GetOptions{})
		if getErr != nil {
			logger.Errorf("Failed to get latest version of Service: %v", getErr)
		}

		if port == "" {
			result.Spec.Ports[0].TargetPort = intstr.FromInt(80)
		} else {
			result.Spec.Ports[0].TargetPort = intstr.FromString(port)
		}
		_, updateErr := client.CoreV1().Services(namespace).Update(result)
		return updateErr
	})

	if retryErr == nil {
		logger.Infof("Updated service... %s", service)
	} else {
		logger.Errorf("Update failed: %v", retryErr)
	}
}
