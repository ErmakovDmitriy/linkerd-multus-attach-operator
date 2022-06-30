/*
Copyright 2022 ErmakovDmitriy.

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

package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	k8s "github.com/ErmakovDmitriy/linkerd-multus-attach-operator/k8s"
	pkgK8s "github.com/linkerd/linkerd2/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var podCopyAnnotations = []string{
	k8s.MultusAttachAnnotation,
	k8s.LinkerdInjectAnnotation,
}

// log is for logging in this package.
var podlog = logf.Log.WithName("pod-resource")

//nolint:lll
//+kubebuilder:webhook:path=/annotate-multus-v1-pod,mutating=true,failurePolicy=ignore,sideEffects=None,groups="",resources=pods,verbs=create;update,versions=v1,name=multus.linkerd.io,admissionReviewVersions=v1
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;versions=v1
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;versions=v1
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;versions=v1

// PodAnnotator adds Multus annotation to a Pod to attach Linkerd CNI via Multus.
type PodAnnotator struct {
	controlPlaneNamespace string
	Client                client.Client
	decoder               *admission.Decoder
}

// Handle implements WebHook handler.
// Checks if a Pod or its Namespace have "linkerd.io/multus" annotation and then
// appends to "k8s.v1.cni.cncf.io/networks" annotations the linkerd-cni network.
func (a *PodAnnotator) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	err := a.decoder.Decode(req, pod)
	if err != nil {
		podlog.Error(err, "can not decode Pod")

		return admission.Errored(http.StatusBadRequest, err)
	}

	podlog = podlog.WithValues("req_namespace", req.Namespace, "pod_generate_name", pod.GenerateName)
	podlog.Info("Received request")

	// Retrieve namespace annotations.
	var namespace = &corev1.Namespace{}

	if err := a.Client.Get(ctx, types.NamespacedName{Name: req.Namespace}, namespace); err != nil {
		podlog.Error(err, "Can not get namespace")

		return admission.Errored(http.StatusInternalServerError, err)
	}

	nsAnnotations := namespace.GetAnnotations()

	// Annotate Pod with Namespace annotations.
	pod = copyAnnotations(pod, nsAnnotations)

	// Do nothing, if Linkerd CNI is not requested.
	if !isMultusAnnotationRequested(pod, req.Namespace, a.controlPlaneNamespace) {
		podlog.Info("Multus NetworkAttachmentDefinition is not requested, do not patch")

		return admission.Allowed("No Multus attachment requested")
	}

	// Mutate the fields in pod.
	pod = patchPod(pod)

	podlog.Info("Patches Pod annotations",
		k8s.MultusNetworkAttachAnnotation, pod.GetAnnotations()[k8s.MultusNetworkAttachAnnotation])

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

// InjectDecoder injects provided decoder to the WebHook instance.
func (a *PodAnnotator) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d
	return nil
}

// SetupWebhookWithManager attaches PodAnnotator to a provided manager.
func SetupWebhookWithManager(mgr ctrl.Manager, controlPlaneNamespace string) {
	mgr.GetWebhookServer().Register(
		"/annotate-multus-v1-pod",
		&webhook.Admission{
			Handler: &PodAnnotator{
				Client:                mgr.GetClient(),
				controlPlaneNamespace: controlPlaneNamespace,
			},
		},
	)
}

// copyAnnotations copies podCopyAnnotations from a Pod's Namespace to the Pod.
// Does not change the Pod's annotations, if they are defined.
func copyAnnotations(pod *corev1.Pod, nsAnnotations map[string]string) *corev1.Pod {
	if nsAnnotations == nil {
		return pod
	}

	podAnnotations := pod.GetAnnotations()
	if podAnnotations == nil {
		podAnnotations = make(map[string]string)
	}

	for _, key := range podCopyAnnotations {
		if val := podAnnotations[key]; val == "" {
			podAnnotations[key] = nsAnnotations[key]
		}
	}

	pod.Annotations = podAnnotations

	return pod
}

// isMultusAnnotationRequested checks if a Pod requires Linkerd CNI via Multus.
func isMultusAnnotationRequested(pod *corev1.Pod, reqNamespace, controlPlaneNamespace string) bool {
	// Injection is explicitly requested.
	podAnnotations := pod.GetAnnotations()
	ldInjectVal := podAnnotations[k8s.LinkerdInjectAnnotation]

	if podAnnotations[k8s.MultusAttachAnnotation] == k8s.MultusAttachEnabled &&
		(ldInjectVal == pkgK8s.ProxyInjectEnabled || ldInjectVal == pkgK8s.ProxyInjectIngress) {
		return true
	}

	// Control plane Pods must be always processed by Linkerd CNI.
	podLabels := pod.GetLabels()

	// Does not have "control-plane" labels.
	if podLabels == nil {
		return false
	}

	if reqNamespace == controlPlaneNamespace &&
		podLabels[pkgK8s.ControllerComponentLabel] != "" {
		return true
	}

	return false
}

// patchPod adds Linkerd CNI to a Pod's "k8s.v1.cni.cncf.io/networks" annotation.
func patchPod(pod *corev1.Pod) *corev1.Pod {
	podAnnotations := pod.GetAnnotations()

	val, ok := podAnnotations[k8s.MultusNetworkAttachAnnotation]
	if !ok || val == "" {
		pod.Annotations[k8s.MultusNetworkAttachAnnotation] = k8s.MultusNetworkAttachmentDefinitionName
	} else {
		// Check that the linkerd-cni is not in the annotation's value yet.
		nets := strings.Split(val, ",")

		for _, n := range nets {
			if n == k8s.MultusNetworkAttachmentDefinitionName {
				return pod
			}
		}

		pod.Annotations[k8s.MultusNetworkAttachAnnotation] = val + "," + k8s.MultusNetworkAttachmentDefinitionName
	}

	return pod
}
