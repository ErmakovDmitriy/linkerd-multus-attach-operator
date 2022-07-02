#!/bin/bash

set -x
set -o pipefail

# Used Debian 11 and k3s as k3d has issues with Multus daemon set.
# Multus Pod can not be created due to mounts as below:
#   Warning  Failed     2m48s                  kubelet            Error: failed to generate container "d40741365de7982e5a57ea12215081e0acb8f0100379c04bc0a64cde2f49a115" spec: failed to generate spec: path "/var/lib/rancher/k3s/data/current/bin" is mounted on "/var/lib/rancher/k3s" but it is not a shared mount

echo "Installing prerequisities"
apt update
apt-get install docker.io docker curl jq python3 containerd runc -y

echo "Installing k3s"
curl -sfL https://get.k3s.io | sh -

echo "Downloading kubectl"
curl -L "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" -o /usr/local/bin/kubectl
chmod +x /usr/local/bin/kubectl

alias kubectl="k3s kubectl"
echo "Waiting for k3s to be ready"
kubectl wait --for=condition=Ready node/$(hostname)

echo "Install Helm"
curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

echo "Install kustomize"
curl -s "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"  | bash
mkdir ~/.kustomize/bin/ -p
cp kustomize /usr/local/bin/kustomize

echo "Downloading Multus CNI"
# Special thanks to: https://gist.github.com/janeczku/ab5139791f28bfba1e0e03cfc2963ecf
curl -L "https://raw.githubusercontent.com/k8snetworkplumbingwg/multus-cni/v3.9/deployments/multus-daemonset.yml" -o multus-cni.yaml
sed -i -e 's/path: \/etc\/cni\/net.d/path: \/var\/lib\/rancher\/k3s\/agent\/etc\/cni\/net.d/' multus-cni.yaml
sed -i -e 's/path: \/opt\/cni\/bin/path: \/var\/lib\/rancher\/k3s\/data\/current\/bin/' multus-cni.yaml
sed -i -e 's/args:/args:\n        - "--multus-kubeconfig-file-host=\/var\/lib\/rancher\/k3s\/agent\/etc\/cni\/net.d\/multus.d\/multus.kubeconfig"/' multus-cni.yaml
kubectl apply --wait -f multus-cni.yaml
kubectl -n kube-system rollout status  daemonset/kube-multus-ds --timeout=120s


echo "Installing Linkerd CLI"
export INSTALLROOT=/usr/local/
curl --proto '=https' --tlsv1.2 -sSfL https://run.linkerd.io/install | sh
linkerd version

echo "Check Kubernetes cluster before Linkerd install"
linkerd check --pre

# Use /tmp/ for linkerd-cni configuration so as to 
# prevent kubelet from handling linked cni via its configuration file.
# make deploy-test for the controller also configures the controller to use
# "-cni-kubeconfig=/tmp/ZZZ-linkerd-cni-kubeconfig"
echo "Installind Linkerd CNI"
linkerd install-cni \
  --cni-image=docker.io/demonihin/linkerd2-cni \
  --linkerd-version=latest \
  --dest-cni-bin-dir=/var/lib/rancher/k3s/data/current/bin \
  --dest-cni-net-dir=/tmp/ | kubectl apply --wait -f -
kubectl -n linkerd-cni rollout status  daemonset/linkerd-cni --timeout=120s

echo "Installing cert-manager for webhook"
helm repo add jetstack https://charts.jetstack.io
helm repo update
helm install \
  cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --version v1.8.2 \
  --set installCRDs=true \
  --atomic=true

echo "Install the operator and its webhook"
make deploy-test IMG="docker.io/demonihin/linkerd-multus-attach-operator:latest"
kubectl -n linkerd-multus-attach-operator-system rollout status deployment/linkerd-multus-operator-controller-manager --timeout=120s
sleep 20 # Time to get lease and load WebHook TLS certificates