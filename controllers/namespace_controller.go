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

// Package controllers defines a namespace controller
// which watches namespace changes and, if a namespace is a
// Linkerd control plane namespace or it has linkerd.io/multus=enabled
// annotation, the controller creates a Network Attachment Definition for
// Linkerd CNI plugin.
package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	k8s "github.com/ErmakovDmitriy/linkerd-multus-attach-operator/k8s"
	netattachv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
)

const debugLogLevel = 1

// NamespaceReconciler reconciles a Namespace object with Multus
// NetworkAttachmentDefinitions.
type NamespaceReconciler struct {
	client.Client
	Scheme                       *runtime.Scheme
	LinkerdControlPlaneNamespace string
	LinkerdCNINamespace          string
	LinkerdCNIKubeconfigPath     string
}

//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=namespaces/status,verbs=get;update;patch
//+kubebuilder:rbac:groups="",resources=namespaces/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
//+kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Namespace object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *NamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("request_namespace", req.Name)

	logger.V(debugLogLevel).Info("Reconcile event")

	// Load namespace.
	var ns = &corev1.Namespace{}

	if err := r.Get(ctx, req.NamespacedName, ns); err != nil {
		if errors.IsNotFound(err) {
			logger.V(debugLogLevel).Info("Namespace was deleted, no action needed")

			return ctrl.Result{}, nil
		}

		logger.Error(err, "can not get Namespace")

		return ctrl.Result{}, fmt.Errorf("can not get Namespace: %w", err)
	}

	// Stop processing for a Namespace which is terminating.
	if ns.Status.Phase == corev1.NamespaceTerminating {
		logger.V(debugLogLevel).Info("Namespace is Terminating, not action needed")

		return ctrl.Result{}, nil
	}

	// Check if Multus NetworkAttachmentDefinition must be in the namespace.
	var isMultusRequired bool

	if req.Name == r.LinkerdControlPlaneNamespace {
		logger.V(debugLogLevel).Info("Controller namespace must always have NetworkAttachmentDefinition")

		isMultusRequired = true
	} else {
		// Check if Multus is requested in the Namespace.
		isMultusRequired = (ns.Annotations[k8s.MultusAttachAnnotation] == k8s.MultusAttachEnabled)
	}

	var (
		multusNetAttach = &netattachv1.NetworkAttachmentDefinition{}
		multusRef       = types.NamespacedName{
			Namespace: req.Name,
			Name:      k8s.MultusNetworkAttachmentDefinitionName,
		}
	)

	logger = logger.WithValues("multusRef", multusRef.String())

	logger.V(debugLogLevel).Info("Checked if Multus NetworkAttachmentDefinition is required", "is_required", isMultusRequired)

	if err := r.Get(ctx, multusRef, multusNetAttach); err != nil {
		// Errors except NotFound are treated as errors.
		if !errors.IsNotFound(err) {
			logger.Error(err, "Can not get Multus NetworkAttachmentDefinition")

			return ctrl.Result{}, fmt.Errorf("can not get Multus NetworkAttachmentDefinition: %w", err)
		}

		// Here we have a state "NetworkAttachmentDefinition is not found in the namespace".

		// No Multus in the namespace and required - create new.
		if isMultusRequired {
			logger.V(debugLogLevel).Info("Multus NetworkAttachmentDefinition is not in the Namespace and required, creating")

			if err := createMultusNetAttach(ctx, r.Client, multusRef,
				r.LinkerdCNINamespace, r.LinkerdCNIKubeconfigPath); err != nil {
				logger.Error(err, "can not create Multus NetworkAttachmentDefinition")

				return ctrl.Result{}, err
			}

			return ctrl.Result{}, nil
		}

		// Multus NetworkAttachmentDefinition is not found and not required.
		logger.V(debugLogLevel).Info("Multus NetworkAttachmentDefinition is not found in the Namespace and not required, do nothing")

		return ctrl.Result{}, nil
	}

	// Here we have the state "NetworkAttachmentDefinition is found in the namespace".

	// We have Multus in the Namespace, decide what to do with it.
	if !isMultusRequired {
		logger.V(debugLogLevel).Info("Multus NetworkAttachmentDefinition is in the Namespace and not required, deleting")

		if err := deleteMultusNetAttach(ctx, r.Client, multusNetAttach); err != nil {
			logger.Error(err, "can not delete Multus NetworkAttachmentDefinition")

			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	// Update multus if necessary.
	logger.V(debugLogLevel).Info("Multus NetworkAttachmentDefinition is in the Namespace and required, patch if changed")

	if err := updateMultusNetAttach(ctx, r.Client, logger,
		multusNetAttach, r.LinkerdCNINamespace, r.LinkerdCNIKubeconfigPath); err != nil {
		logger.Error(err, "can not update Multus NetworkAttachmentDefinition")

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Watches(
			&source.Kind{Type: &netattachv1.NetworkAttachmentDefinition{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
				return []reconcile.Request{
					{
						NamespacedName: types.NamespacedName{
							Name: o.GetNamespace(),
						},
					},
				}
			}),
			builder.WithPredicates(getEventFilter()),
		).
		Complete(r)
}
