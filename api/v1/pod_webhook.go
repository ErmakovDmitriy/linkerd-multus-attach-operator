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
	"fmt"
	"net/http"
	"strconv"
	"strings"

	k8s "github.com/ErmakovDmitriy/linkerd-multus-attach-operator/k8s"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const debugLogLevel = 1

//nolint:lll
//+kubebuilder:webhook:path=/annotate-multus-v1-pod,mutating=true,failurePolicy=fail,sideEffects=None,groups="",resources=pods,verbs=create;update,versions=v1,name=multus.linkerd.io,admissionReviewVersions=v1
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;versions=v1
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;versions=v1
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;versions=v1

// PodAnnotator adds Multus annotation to a Pod to attach Linkerd CNI via Multus.
type PodAnnotator struct {
	controlPlaneNamespace string

	namespaceAllowedUIDsAnnotation string
	linkerdProxyUIDOffset          int64

	Client  client.Client
	decoder *admission.Decoder
}

// Handle implements WebHook handler.
// Checks if a Pod has the Linkerd proxy sidecar and network-check container and
// appends to "k8s.v1.cni.cncf.io/networks" annotations the linkerd-cni network, if they are present.
// Changes the sidecar's and network check container's runAsUser, if UID range annotation present.
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

	// Check if the Pod has Linkerd Proxy and Network Validator => needs Multus.
	checkCntFound, checkCntInd := findLinkerdNetValidator(pod)
	proxyCntFound, proxyCntInd := findLinkerdProxy(pod)

	if !checkCntFound || !proxyCntFound {
		podlog.V(debugLogLevel).Info("Pod data", "pod", pod)

		return admission.Allowed("Pod does not have Linkerd proxy or/and Linkerd Network Validator, do not add Multus Network Attach Definition")
	}

	// Patch with Multus Network Attach Definition annotation.
	podlog.V(debugLogLevel).Info("Annotating with Multus NetworkAttachmentDefinition")
	pod = addMultusNetAttach(pod)

	// Check if the Pod should have proxy and network validator runAsUser patched.

	// Retrieve namespace annotations.
	var namespace = &corev1.Namespace{}

	if err := a.Client.Get(ctx, types.NamespacedName{Name: req.Namespace}, namespace); err != nil {
		podlog.Error(err, "Can not get namespace")

		return admission.Errored(http.StatusInternalServerError, err)
	}

	nsAnnotations := namespace.GetAnnotations()

	nsPoolRangeRaw, ok := nsAnnotations[a.namespaceAllowedUIDsAnnotation]
	if !ok {
		// No allowed UIDs range - return Multus-annotated pod without changing
		// Proxy and Network Validator UIDs.
		podlog.V(debugLogLevel).Info("Pod's namespace does not have annotation, do not change Proxy and Network Validator SecurityContexts", "absent_annotation", a.namespaceAllowedUIDsAnnotation)

		return makePatch(podlog, req, pod)
	}

	uidStart, uidEnd, err := parseUIDRange(nsPoolRangeRaw)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	podlog.V(debugLogLevel).Info("Allowed RunAsUser range", "start", uidStart, "end", uidEnd)

	// Custom proxy UID is not set - annotate and set.
	if pod.GetAnnotations()[k8s.LinkerdProxyUIDAnnotation] == "" {
		proxyUID := uidStart + a.linkerdProxyUIDOffset

		if proxyUID > uidEnd {
			return admission.Errored(http.StatusInternalServerError, fmt.Errorf("Computed proxy UID %d is not in the allowed range [%d, %d]", proxyUID, uidStart, uidEnd))
		}

		pod.GetAnnotations()[k8s.LinkerdProxyUIDAnnotation] = strconv.FormatInt(proxyUID, 10)
		if pod.Spec.Containers[proxyCntInd].SecurityContext == nil {
			podlog.V(debugLogLevel).Info("Proxy container does not have SecurityContext, defining a new one", "RunAsUser", proxyUID)
			pod.Spec.Containers[proxyCntInd].SecurityContext = &corev1.SecurityContext{RunAsUser: &proxyUID}
		} else {
			podlog.V(debugLogLevel).Info("Proxy container does has SecurityContext, setting RunAsUser", "RunAsUser", proxyUID)
			pod.Spec.Containers[proxyCntInd].SecurityContext.RunAsUser = &proxyUID
		}
	}

	// Network Validator container set runAsUser.
	if pod.Spec.InitContainers[checkCntInd].SecurityContext == nil {
		podlog.V(debugLogLevel).Info("Network Validator container does not have SecurityContext, defining a new one", "RunAsUser", uidStart)
		pod.Spec.InitContainers[checkCntInd].SecurityContext = &corev1.SecurityContext{RunAsUser: &uidStart}
	} else {
		podlog.V(debugLogLevel).Info("Network Validator container has SecurityContext,  setting RunAsUser", "RunAsUser", uidStart)
		pod.Spec.InitContainers[checkCntInd].SecurityContext.RunAsUser = &uidStart
	}

	if podlog.V(debugLogLevel).Enabled() {
		podlog.Info("Patched Pod", "pod", pod)
	}

	return makePatch(podlog, req, pod)
}

// InjectDecoder injects provided decoder to the WebHook instance.
func (a *PodAnnotator) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d
	return nil
}

// SetupWebhookWithManager attaches PodAnnotator to a provided manager.
func SetupWebhookWithManager(mgr ctrl.Manager, controlPlaneNamespace, namespaceAllowedUIDsAnnotation string, linkerdProxyUIDOffset int64) {
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

func makePatch(podlog logr.Logger, req admission.Request, pod *corev1.Pod) admission.Response {
	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if podlog.V(debugLogLevel).Enabled() {
		podlog.V(debugLogLevel).Info("Patched Pod", "pod", string(marshaledPod))
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

// ErrIncorrectUIDRange - incorrect UID range annotation received.
type ErrIncorrectUIDRange struct {
	iErr   error
	rawVal string
	msg    string
}

func (e ErrIncorrectUIDRange) Error() string {
	if e.iErr == nil {
		return fmt.Sprintf("can not parse %q UID range: %s", e.rawVal, e.msg)
	}

	return fmt.Sprintf("can not parse %q UID range: %s: %s", e.rawVal, e.msg, e.iErr)
}

var _ error = (*ErrIncorrectUIDRange)(nil)

// parseUIDRange - parses value {{ first ID }}/{{ pool size }} to extract start and end UIDs.
func parseUIDRange(namespaceUIDRange string) (start int64, end int64, _ error) {
	splIDRange := strings.Split(namespaceUIDRange, "/")
	if len(splIDRange) != 2 { // The correct value is like "10000000/2000".
		return -1, -1, ErrIncorrectUIDRange{rawVal: namespaceUIDRange, iErr: nil, msg: "The value is not in {{ first ID }}/{{ pool size }} format"}
	}

	start, err := strconv.ParseInt(splIDRange[0], 10, 64)
	if err != nil {
		return -1, -1, ErrIncorrectUIDRange{rawVal: namespaceUIDRange, iErr: err, msg: "first UID is not a correct integer"}
	}

	poolSize, err := strconv.ParseInt(splIDRange[1], 10, 64)
	if err != nil {
		return -1, -1, ErrIncorrectUIDRange{rawVal: namespaceUIDRange, iErr: err, msg: "UID range pool size is not a correct integer"}
	}

	return start, start + poolSize, nil
}

func findLinkerdProxy(pod *corev1.Pod) (found bool, index int) {
	for cntI := range pod.Spec.Containers {
		if pod.Spec.Containers[cntI].Name == k8s.LinkerdProxyContainerName {
			return true, cntI
		}
	}

	return false, -1
}

func findLinkerdNetValidator(pod *corev1.Pod) (found bool, index int) {
	for cntI := range pod.Spec.InitContainers {
		if pod.Spec.InitContainers[cntI].Name == k8s.LinkerdNetworkValidatorContainerName {
			return true, cntI
		}
	}

	return false, -1
}

// addMultusNetAttach adds Linkerd CNI to a Pod's "k8s.v1.cni.cncf.io/networks" annotation.
func addMultusNetAttach(pod *corev1.Pod) *corev1.Pod {
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
