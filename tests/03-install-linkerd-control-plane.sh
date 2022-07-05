#!/bin/bash

# set -x
set -o pipefail

kubectl -n linkerd-multus-attach-operator-system rollout status deployment linkerd-multus-operator-controller-manager --timeout=10s
kubectl -n linkerd-cni rollout status daemonset linkerd-cni --timeout=10s

echo "Installing Linkerd"
linkerd check --pre 
# linkerd install --crds | kubectl apply -f -
linkerd install --linkerd-cni-enabled | kubectl apply --wait -f -

echo "Check linkerd"
linkerd check