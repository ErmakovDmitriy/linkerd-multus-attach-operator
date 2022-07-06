#!/bin/bash

# set -x
set -o pipefail

NAMESPACE="emojivoto"
SEPARATOR="##########"

echo $SEPARATOR
echo "Testing emojivoto application"

echo $SEPARATOR
echo "Delete $NAMESPACE if present"
kubectl delete namespace $NAMESPACE || echo "Namespace $NAMESPACE is not found - nothing to delete"
echo "Creating $NAMESPACE namespace"
kubectl apply -f tests/emojivoto/namespace.yml

echo $SEPARATOR
echo "Starting emojivoto test application without Multus annotation"
echo "Expect to have not-meshed pods"
linkerd inject tests/emojivoto/emojivoto.yml | kubectl apply -f -

echo $SEPARATOR
echo "Wait until all the deployments are rolled out"
kubectl --namespace $NAMESPACE rollout status deployment voting --timeout=60s
kubectl --namespace $NAMESPACE rollout status deployment emoji --timeout=60s
kubectl --namespace $NAMESPACE rollout status deployment web --timeout=60s
kubectl --namespace $NAMESPACE rollout status deployment vote-bot --timeout=60s

echo "Wait for 30 seconds to allow linkerd to collect statistics"
sleep 30

echo "There should not be any linkerd edges between pods in the namespace"
EDGES=$(linkerd viz edges deployment --namespace emojivoto -o json)
echo "Edges report:"
echo $EDGES | jq
echo $EDGES | python tests/count_edges.py expect-not-meshed-only


echo $SEPARATOR
echo "Delete $NAMESPACE if present"
kubectl delete namespace $NAMESPACE || echo "Namespace $NAMESPACE is not found - nothing to delete"
echo "Creating $NAMESPACE namespace"
kubectl apply -f tests/emojivoto/namespace.yml
echo "Annotate the $NAMESPACE namespace with linkerd.io/multus=enabled to use Multus"
echo "Expect to have meshed pods"
kubectl annotate --overwrite namespace/$NAMESPACE "linkerd.io/multus=enabled"

echo $SEPARATOR
echo "Starting emojivoto test application with Multus annotation"
echo "Expect to have meshed pods"
linkerd inject tests/emojivoto/emojivoto.yml | kubectl apply -f -

echo "Wait until all the deployments are rolled out"
kubectl --namespace $NAMESPACE rollout status deployment voting --timeout=60s
kubectl --namespace $NAMESPACE rollout status deployment emoji --timeout=60s
kubectl --namespace $NAMESPACE rollout status deployment web --timeout=60s
kubectl --namespace $NAMESPACE rollout status deployment vote-bot --timeout=60s

echo "Wait for 30 seconds to allow linkerd to collect statistics"
sleep 30

echo "There should be 3 linkerd edges between pods in the namespace $NAMESPACE"
EDGES=$(linkerd viz edges deployment --namespace emojivoto -o json)
echo "Edges report:"
echo $EDGES | jq
echo $EDGES | python tests/count_edges.py expect-meshed


echo $SEPARATOR
echo "It seems that all tests passed."