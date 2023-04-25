# set -x
set -o pipefail

SEPARATOR="##########"

net_attach_must_not_present() {
  if kubectl -n $1 get network-attachment-definition linkerd-cni; then
    echo "Fail: Multus NetworkAttachmentDefinition is in $1 namespace. Expected: not present"
    return 1
  else
    echo "Pass: Multus NetworkAttachmentDefinition is not in $1 namespace. Expected: not present"
    return 0
  fi
}

net_attach_must_present() {
  if kubectl -n $1 get network-attachment-definition linkerd-cni; then
    echo "Pass: Multus NetworkAttachmentDefinition is in $1 namespace. Expected: present"
    return 0
  else
    echo "Fail: Multus NetworkAttachmentDefinition is not in $1 namespace. Expected: present"
    return 1
  fi
}

# export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
alias k="k3s kubectl"

echo "$SEPARATOR"
echo "Test namespace annotations handling"
export NAMESPACE="test-ns-1"

echo "$SEPARATOR"
echo "Namespace without Multus annotation"
kubectl delete namespace $NAMESPACE || echo "Namespace $NAMESPACE is not found - nothing to delete"
kubectl create namespace $NAMESPACE
net_attach_must_not_present $NAMESPACE

echo "$SEPARATOR"
echo "Annotating namespace $NAMESPACE with Multus annotation"
kubectl annotate --overwrite namespace/$NAMESPACE "linkerd.io/multus=enabled"
sleep 1 # Not necessary as the controller has a watcher so must react immediately.
net_attach_must_present $NAMESPACE

echo "$SEPARATOR"
echo "Disable Multus by annotation"
kubectl annotate --overwrite namespace/$NAMESPACE "linkerd.io/multus=disabled"
sleep 1 # Not necessary as the controller has a watcher so must react immediately.
net_attach_must_not_present $NAMESPACE

echo "$SEPARATOR"
echo "Annotating namespace $NAMESPACE with Multus annotation"
kubectl annotate --overwrite namespace/$NAMESPACE "linkerd.io/multus=enabled"
sleep 1 # Not necessary as the controller has a watcher so must react immediately.
net_attach_must_present $NAMESPACE

echo "$SEPARATOR"
echo "Delete Multus annotation"
kubectl annotate --overwrite namespace/$NAMESPACE "linkerd.io/multus-"
sleep 1 # Not necessary as the controller has a watcher so must react immediately.
net_attach_must_not_present $NAMESPACE

kubectl delete namespace $NAMESPACE

echo "$SEPARATOR"
echo "Test Webhook annotations"
export NAMESPACE="test-ns-2"
kubectl delete ns $NAMESPACE || echo "Namespace not found - nothing to delete"
kubectl create namespace $NAMESPACE
net_attach_must_not_present $NAMESPACE

echo "$SEPARATOR"
echo "Test that a pod gets annotations if its namespace has it"
echo "Annotating namespace $NAMESPACE with Multus annotation"
kubectl annotate --overwrite namespace/$NAMESPACE "linkerd.io/multus=enabled"
sleep 1 # Not necessary as the controller has a watcher so must react immediately.
net_attach_must_present $NAMESPACE

echo "$SEPARATOR"
echo "Annotate namespace with Linkerd Inject"
kubectl annotate --overwrite namespace/$NAMESPACE "linkerd.io/inject=enabled"
sleep 1

POD_SOURCE=$(cat <<EOF
{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": {
        "name": "test-pod",
        "namespace": "$NAMESPACE"
    },
    "spec": {
        "initContainers": [
            {
                "command": [
                    "sleep",
                    "3600"
                ],
                "image": "busybox",
                "name": "linkerd-network-validator"
            }
        ],
        "containers": [
            {
                "command": [
                    "sleep",
                    "3600"
                ],
                "image": "busybox",
                "name": "test"
            },
            {
                "command": ["start-proxy"],
                "image": "linkerd-proxy",
                "name": "linkerd-proxy"
            }
        ]
    }
}
EOF
)

echo "$SEPARATOR"
echo "Source Pod definition:"
echo $POD_SOURCE | jq
echo "Check a Pod, expected to have the Multus annotation on the Pod"
__POD_WITH_EXPECTED_ANNOTATIONS=$(echo $POD_SOURCE | kubectl apply --dry-run=server -o json -f -)
echo "Pod with expected annotations definition:"
echo "$__POD_WITH_EXPECTED_ANNOTATIONS" | jq
echo $__POD_WITH_EXPECTED_ANNOTATIONS | python tests/check_annotations.py must-contain

echo "$SEPARATOR"
POD_SOURCE=$(cat <<EOF
{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": {
        "name": "test-pod",
        "namespace": "$NAMESPACE"
    },
    "spec": {
        "containers": [
            {
                "command": [
                    "sleep",
                    "3600"
                ],
                "image": "busybox",
                "name": "test"
            },
            {
                "command": ["start-proxy"],
                "image": "linkerd-proxy",
                "name": "linkerd-proxy"
            }
        ]
    }
}
EOF
)
echo "Check a Pod, expected not to have the Multus annotation on the Pod"
echo "Source Pod definition:"
echo $POD_SOURCE | jq
__POD_WITHOUT_EXPECTED_ANNOTATIONS=$(echo $POD_SOURCE | kubectl apply --dry-run=server -o json -f -)
echo "Pod without expected annotations definition:"
echo "$__POD_WITHOUT_EXPECTED_ANNOTATIONS" | jq
echo $__POD_WITHOUT_EXPECTED_ANNOTATIONS | python tests/check_annotations.py must-not-contain

echo "$SEPARATOR"
POD_SOURCE=$(cat <<EOF
{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": {
        "name": "test-pod",
        "namespace": "$NAMESPACE"
    },
    "spec": {
        "initContainers": [
            {
                "command": [
                    "sleep",
                    "3600"
                ],
                "image": "busybox",
                "name": "linkerd-network-validator"
            }
        ],
        "containers": [
            {
                "command": [
                    "sleep",
                    "3600"
                ],
                "image": "busybox",
                "name": "test"
            }
        ]
    }
}
EOF
)
echo "Check a Pod, expected not to have the Multus annotation on the Pod"
echo "Source Pod definition:"
echo $POD_SOURCE | jq
__POD_WITHOUT_EXPECTED_ANNOTATIONS=$(echo $POD_SOURCE | kubectl apply --dry-run=server -o json -f -)
echo "Pod without expected annotations definition:"
echo "$__POD_WITHOUT_EXPECTED_ANNOTATIONS" | jq
echo $__POD_WITHOUT_EXPECTED_ANNOTATIONS | python tests/check_annotations.py must-not-contain
