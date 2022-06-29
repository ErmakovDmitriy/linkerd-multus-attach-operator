set -x
set -o pipefail

export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
alias k="k3s kubectl"

echo "Test namespace annotations handling"
export NAMESPACE="test-ns-1"

echo "Namespace without Multus annotation"
kubectl create namespace $NAMESPACE
net_attach_must_not_present $NAMESPACE

echo "Annotating namespace $NAMESPACE with Multus annotation"
kubectl annotate --overwrite namespace/$NAMESPACE "linkerd.io/multus=enabled"
sleep 1 # Not necessary as the controller has a watcher so must react immediately.
net_attach_must_present $NAMESPACE

echo "Disable Multus by annotation"
kubectl annotate --overwrite namespace/$NAMESPACE "linkerd.io/multus=disabled"
sleep 1 # Not necessary as the controller has a watcher so must react immediately.
net_attach_must_not_present $NAMESPACE

echo "Annotating namespace $NAMESPACE with Multus annotation"
kubectl annotate --overwrite namespace/$NAMESPACE "linkerd.io/multus=enabled"
sleep 1 # Not necessary as the controller has a watcher so must react immediately.
net_attach_must_present $NAMESPACE

echo "Delete Multus annotation"
kubectl annotate --overwrite namespace/$NAMESPACE "linkerd.io/multus-"
sleep 1 # Not necessary as the controller has a watcher so must react immediately.
net_attach_must_not_present $NAMESPACE

echo "Test Webhook annotations"
export NAMESPACE="test-ns-2"
kubectl create namespace $NAMESPACE
net_attach_must_not_present $NAMESPACE

echo "Test that a pod gets annotations if its namespace has it"

echo "Annotating namespace $NAMESPACE with Multus annotation"
kubectl annotate --overwrite namespace/$NAMESPACE "linkerd.io/multus=enabled"

sleep 1 # Not necessary as the controller has a watcher so must react immediately.
net_attach_must_present $NAMESPACE

echo "Annotate namespace with Linkerd Inject"
kubectl annotate --overwrite namespace/$NAMESPACE "linkerd.io/inject=enabled"

POD_SOURCE=$(cat <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  namespace: $NAMESPACE
spec:
  containers:
    - name: test
      image: busybox
      command: ["sleep", "3600"]
EOF
)

POD_DEFINITION=$(echo $POD_SOURCE | kubectl apply --dry-run=server -o yaml -f -)
if echo $POD_DEFINITION | grep 'k8s.v1.cni.cncf.io/networks: linkerd-cni'; then
  echo "Pod contains linkerd-cni network attach annotation as expected"
  echo $POD_DEFINITION
else
  echo "Pod must contain linkerd-cni network attach annotation but it does not"
  echo $POD_DEFINITION
  exit 1
fi

# # kubectl wait --for=condition=Ready Daemonset/linkerd-cni -n linkerd-cni --timeout=60s
# echo "Installing Linkerd"
# linkerd install --linkerd-cni-enabled | kubectl apply --wait -f -

 