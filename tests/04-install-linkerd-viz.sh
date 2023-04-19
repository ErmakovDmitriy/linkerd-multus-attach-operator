#!/bin/bash

# set -x
set -o pipefail

echo "Creating a namespace for linkerd-viz"
NAMESPACE="linkerd-viz"
kubectl create namespace $NAMESPACE
kubectl annotate --overwrite namespace/$NAMESPACE "linkerd.io/multus=enabled"

echo "Print extra diagnostics information"
kubectl get ns $NAMESPACE -o yaml
kubectl -n $NAMESPACE get network-attachment-definitions.k8s.cni.cncf.io linkerd-cni -o yaml

linkerd viz install | kubectl apply --wait -f -

echo "Checking rollout status of linkerd viz components"
kubectl -n $NAMESPACE rollout status deployment tap --timeout=120s
kubectl -n $NAMESPACE rollout status deployment metrics-api --timeout=120s
kubectl -n $NAMESPACE rollout status deployment tap-injector --timeout=120s
kubectl -n $NAMESPACE rollout status deployment web --timeout=120s
kubectl -n $NAMESPACE rollout status deployment prometheus --timeout=120s

# If this check is successful, it means that the multus handles linkerd-cni
# plugin as expected. As soon as the multus annotation (and as a result the Multus NetworkAttachmentDefinition)
# is removed from the linkerd-viz namespace, the check fails because the viz
# pods can not communicate to linkerd control plane.
kubectl -n $NAMESPACE get deployment
kubectl -n $NAMESPACE get pod
kubectl -n $NAMESPACE get event

echo "Checking Linkerd VIZ"
linkerd viz check