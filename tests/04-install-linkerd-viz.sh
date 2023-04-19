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
# If this check is successful, it means that the multus handles linkerd-cni
# plugin as expected. As soon as the multus annotation (and as a result the Multus NetworkAttachmentDefinition)
# is removed from the linkerd-viz namespace, the check fails because the viz
# pods can not communicate to linkerd control plane.
kubectl -n $NAMESPACE get pod
kubectl -n $NAMESPACE get event

echo "Checking Linkerd VIZ"
linkerd viz check