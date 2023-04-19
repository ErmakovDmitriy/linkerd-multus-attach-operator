#!/bin/bash

# set -x
set -o pipefail

echo "Installing prerequisities"
apt update
apt-get install curl jq python3 -y

echo "Installing k3s"
curl -sfL https://get.k3s.io | sh -

echo "Downloading kubectl"
curl -L "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" -o /usr/local/bin/kubectl
chmod +x /usr/local/bin/kubectl

alias kubectl="k3s kubectl"
echo "Waiting for k3s to be ready"
kubectl wait --for=condition=Ready node/$(hostname)

echo "Install kustomize"
curl -s "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"  | bash
cp kustomize /usr/local/bin/kustomize

echo "Installing cert-manager for webhook"
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.11.0/cert-manager.yaml
kubectl --namespace cert-manager rollout status deployment/cert-manager-cainjector --timeout=60s
kubectl --namespace cert-manager rollout status deployment/cert-manager --timeout=60s
kubectl --namespace cert-manager rollout status deployment/cert-manager-webhook --timeout=60s

echo "Downloading Multus CNI"
# Special thanks to: https://gist.github.com/janeczku/ab5139791f28bfba1e0e03cfc2963ecf
# The multus-cni.yml is curl -L "https://raw.githubusercontent.com/k8snetworkplumbingwg/multus-cni/v3.9/deployments/multus-daemonset.yml" -o multus-cni.yaml
# with several changed paths to work with K3S:
# volumes:
#   - name: cni
#     hostPath:
#       path: /var/lib/rancher/k3s/agent/etc/cni/net.d/
#   - name: cnibin
#     hostPath:
#       path: /var/lib/rancher/k3s/data/current/bin/
# sed -i -e 's/path: \/etc\/cni\/net.d/path: \/var\/lib\/rancher\/k3s\/agent\/etc\/cni\/net.d/' multus-cni.yaml
# sed -i -e 's/path: \/opt\/cni\/bin/path: \/var\/lib\/rancher\/k3s\/data\/current\/bin/' multus-cni.yaml
# sed -i -e 's/args:/args:\n        - "--multus-kubeconfig-file-host=\/var\/lib\/rancher\/k3s\/agent\/etc\/cni\/net.d\/multus.d\/multus.kubeconfig"/' multus-cni.yaml
kubectl apply --wait -f tests/multus-cni.yaml
kubectl -n kube-system rollout status  daemonset/kube-multus-ds --timeout=120s


echo "Installing Linkerd CLI"
export INSTALLROOT=/usr/local/
curl --proto '=https' --tlsv1.2 -sSfL https://run.linkerd.io/install | sh
linkerd version
linkerd check --pre

# Use /tmp/ for linkerd-cni configuration so as to 
# prevent kubelet from handling linked cni via its configuration file.
# make deploy-test for the controller also configures the controller to use
# "-cni-kubeconfig=/tmp/ZZZ-linkerd-cni-kubeconfig"
echo "Installind Linkerd CNI"
linkerd install-cni \
  --dest-cni-bin-dir=/var/lib/rancher/k3s/data/current/bin \
  --dest-cni-net-dir=/tmp/ | kubectl apply --wait -f -
kubectl -n linkerd-cni rollout status  daemonset/linkerd-cni --timeout=120s

echo "Install the operator and its webhook"
make deploy-test IMG="docker.io/demonihin/linkerd-multus-attach-operator:latest"
kubectl -n linkerd-multus-attach-operator-system rollout status deployment/linkerd-multus-operator-controller-manager --timeout=120s
sleep 20 # Time to get lease and load WebHook TLS certificates


kubectl get pod -A
sleep 10
kubectl -n linkerd-multus-attach-operator-system describe pod

kubectl -n linkerd-multus-attach-operator-system rollout status deployment/linkerd-multus-operator-controller-manager --timeout=120s
sleep 30 # Time to get lease and load WebHook TLS certificates