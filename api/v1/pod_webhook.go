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

// Package v1 contains Pod Mutating Webhook handler which modifies Pod annotations to attach Linkerd-CNI
// via Multus Network Attachment Definition.
package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	k8s "github.com/ErmakovDmitriy/linkerd-multus-attach-operator/k8s"
	"github.com/go-logr/logr"
	pkgK8s "github.com/linkerd/linkerd2/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const debugLogLevel = 1

var podCopyAnnotations = []string{
	k8s.MultusAttachAnnotation,
	k8s.LinkerdInjectAnnotation,
}

//nolint:lll
//+kubebuilder:webhook:path=/annotate-multus-v1-pod,mutating=true,failurePolicy=fail,sideEffects=None,groups="",resources=pods,verbs=create;update,versions=v1,name=multus.linkerd.io,admissionReviewVersions=v1
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;versions=v1
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;versions=v1
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;versions=v1

// PodAnnotator adds Multus annotation to a Pod to attach Linkerd CNI via Multus.
type PodAnnotator struct {
	controlPlaneNamespace string

	namespaceAllowedUIDsAnnotation string
	linkerdProxyUIDOffset          int

	Client  client.Client
	decoder *admission.Decoder
}

// Handle implements WebHook handler.
// Checks if a Pod or its Namespace have "linkerd.io/multus" annotation and then
// appends to "k8s.v1.cni.cncf.io/networks" annotations the linkerd-cni network.
func (a *PodAnnotator) Handle(ctx context.Context, req admission.Request) admission.Response {
	// log is for logging in this function.
	var podlog = logf.FromContext(ctx).WithName("pod-webhook")

	pod := &corev1.Pod{}
	err := a.decoder.Decode(req, pod)
	if err != nil {
		podlog.Error(err, "can not decode Pod")

		return admission.Errored(http.StatusBadRequest, err)
	}

	podlog = podlog.WithValues("req_namespace", req.Namespace, "pod_generate_name", pod.GenerateName)
	podlog.V(debugLogLevel).Info("Received request")

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
	var (
		needNetAttach     bool
		isControlPlanePod bool
	)

	if isMultusAnnotationRequested(pod) {
		podlog.V(debugLogLevel).Info("Pod annotations do not request Multus NetworkAttachmentDefinition", "annotations", pod.GetAnnotations())
		needNetAttach = true
	} else if isControlPlane(pod, req.Namespace, a.controlPlaneNamespace) {
		// Control plane Pods must be always processed by Linkerd CNI.
		podlog.V(debugLogLevel).Info("Pod is a Linkerd control plane")

		needNetAttach = true
		isControlPlanePod = true
	}

	if !needNetAttach {
		podlog.V(debugLogLevel).Info("Multus NetworkAttachmentDefinition is not requested, do not patch")

		return admission.Allowed("No Multus attachment requested")
	}

	// Mutate the fields in pod.
	pod = patchPod(pod)

	// Add optional Openshift UID annotation if not set and the
	// allowed range is defined by a namespace and NOT control plane
	// namespace as they are special.
	// Get the first UID and assign it as the proxy UID.
	if !isControlPlanePod {
		if containerUIDRange, ok := nsAnnotations[a.namespaceAllowedUIDsAnnotation]; ok {
			podlog.V(debugLogLevel).Info("Pod's namespace has UID range annotation",
				a.namespaceAllowedUIDsAnnotation, containerUIDRange)

			pod = addOpenshiftProxyUID(&podlog, a.namespaceAllowedUIDsAnnotation, containerUIDRange, a.linkerdProxyUIDOffset, pod)
		}
	}

	podlog.V(debugLogLevel).Info("Patches Pod annotations",
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
func SetupWebhookWithManager(mgr ctrl.Manager, controlPlaneNamespace, namespaceAllowedUIDsAnnotation string, linkerdProxyUIDOffset int) {
	mgr.GetWebhookServer().Register(
		"/annotate-multus-v1-pod",
		&webhook.Admission{
			Handler: &PodAnnotator{
				Client:                         mgr.GetClient(),
				controlPlaneNamespace:          controlPlaneNamespace,
				namespaceAllowedUIDsAnnotation: namespaceAllowedUIDsAnnotation,
				linkerdProxyUIDOffset:          linkerdProxyUIDOffset,
			},
		},
	)
}

// Add the first allowed UID for Proxy UID based on Openshift namespace
// annotation: openshift.io/sa.scc.uid-range={{ first ID }}/{{ pool size }}.
func addOpenshiftProxyUID(podlog *logr.Logger, namespaceAllowedUIDsAnnotation, namespaceUIDRange string, proxyUIDOffset int, pod *corev1.Pod) *corev1.Pod {
	// If the Pod has already configured value - leave it be.
	if val, ok := pod.GetAnnotations()[k8s.LinkerdProxyUIDAnnotation]; ok && val != "" {
		podlog.V(debugLogLevel).Info(
			"Pod already has UID range annotation, not changing it",
			"config.linkerd.io/proxy-uid", val)

		return pod
	}

	splIDRange := strings.Split(namespaceUIDRange, "/")
	if len(splIDRange) != 2 { // The correct value is like "10000000/2000".
		// Incorrect value. The application assumes that something
		// uses the same namespace annotation but for other purpose
		// and ignores it.
		podlog.Info(
			"Pod must be patched with proxy UID annotation but the namespace's range UID annotation does not conform to {{ first ID }}/{{ range }} format. Ignoring the annotation",
			namespaceAllowedUIDsAnnotation, namespaceUIDRange)

		return pod
	}

	uid, err := strconv.Atoi(splIDRange[0])
	if err != nil {
		podlog.Error(err, "Pod's namespace UID range annotation is not correct, expected to have {{ first ID }}/{{ range }}, integers. Do not change the Pod",
			namespaceAllowedUIDsAnnotation, namespaceUIDRange)

		return pod
	}

	newUIDValue := strconv.Itoa(uid + proxyUIDOffset)
	pod.Annotations[k8s.LinkerdProxyUIDAnnotation] = newUIDValue

	podlog.V(debugLogLevel).Info("Pod is patched with", k8s.LinkerdProxyUIDAnnotation, newUIDValue)

	return pod
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
func isMultusAnnotationRequested(pod *corev1.Pod) bool {
	// Injection is explicitly requested.
	podAnnotations := pod.GetAnnotations()
	ldInjectVal := podAnnotations[k8s.LinkerdInjectAnnotation]

	if podAnnotations[k8s.MultusAttachAnnotation] == k8s.MultusAttachEnabled &&
		(ldInjectVal == pkgK8s.ProxyInjectEnabled || ldInjectVal == pkgK8s.ProxyInjectIngress) {
		return true
	}

	return false
}

func isControlPlane(pod *corev1.Pod, reqNamespace, controlPlaneNamespace string) bool {
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
