#!/bin/bash

set -x
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
mkdir ~/.kustomize/bin/ -p
cp kustomize /usr/local/bin/kustomize

echo "Installing cert-manager for webhook"
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.8.2/cert-manager.yaml
kubectl --namespace cert-manager rollout status deployment/cert-manager-cainjector --timeout=60s
kubectl --namespace cert-manager rollout status deployment/cert-manager --timeout=60s
kubectl --namespace cert-manager rollout status deployment/cert-manager-webhook --timeout=60s

echo "Downloading Multus CNI"

## Default settings of k3s create flannel CNI with version 1.0.0 which is not supported by
## Multus so we get errors like:
##
#   Warning  FailedCreatePodSandBox  44s   kubelet            Failed to create pod sandbox: rpc error: code = Unknown desc = failed to setup network for sandbox "ff09bca66f
# 3733c9eb0f176b80bf55a20285c70179c3a96dc2341f816c1ef2a5": plugin type="multus" name="multus-cni-network" failed (add): [linkerd-multus-attach-operator-system/linkerd-multu
# s-operator-controller-manager-f9cbd8d69-pkmkr/3a8e6a25-0ac7-459a-8df3-25e596d95263:cbr0]: error adding container to network "cbr0": unsupported CNI result version "1.0.0"
#   Warning  FailedCreatePodSandBox  29s   kubelet            Failed to create pod sandbox: rpc error: code = Unknown desc = failed to setup network for sandbox "8f755ddf74
# 4665c71ab8103dc900ba5d670f96adc5051b1210b90ed0965223c0": plugin type="multus" name="multus-cni-network" failed (add): [linkerd-multus-attach-operator-system/linkerd-multu
# s-operator-controller-manager-f9cbd8d69-pkmkr/3a8e6a25-0ac7-459a-8df3-25e596d95263:cbr0]: error adding container to network "cbr0": unsupported CNI result version "1.0.0"
#   Warning  FailedCreatePodSandBox  14s   kubelet            Failed to create pod sandbox: rpc error: code = Unknown desc = failed to setup network for sandbox "9736b63765
# c493901d9adf5eda14337ddf4860a7765b7fc38f021dcf5a737f7c": plugin type="multus" name="multus-cni-network" failed (add): [linkerd-multus-attach-operator-system/linkerd-multu
# s-operator-controller-manager-f9cbd8d69-pkmkr/3a8e6a25-0ac7-459a-8df3-25e596d95263:cbr0]: error adding container to network "cbr0": unsupported CNI result version "1.0.0"
#   Warning  FailedCreatePodSandBox  1s    kubelet            Failed to create pod sandbox: rpc error: code = Unknown desc = failed to setup network for sandbox "801520f269
# a6143f9f8dc12d615f5c087be450d0e01db502334e6b08497f81f4": plugin type="multus" name="multus-cni-network" failed (add): [linkerd-multus-attach-operator-system/linkerd-multu
# s-operator-controller-manager-f9cbd8d69-pkmkr/3a8e6a25-0ac7-459a-8df3-25e596d95263:cbr0]: error adding container to network "cbr0": unsupported CNI result version "1.0.0" 
##
## Here is the k3s flannel default config:
# root@fv-az397-294:/var/lib/rancher/k3s/agent/etc/cni/net.d# cat 10-flannel.conflist 
# {
#   "name":"cbr0",
#   "cniVersion":"1.0.0",
#   "plugins":[
#     {
#       "type":"flannel",
#       "delegate":{
#         "hairpinMode":true,
#         "forceAddress":true,
#         "isDefaultGateway":true
#       }
#     },
#     {
#       "type":"portmap",
#       "capabilities":{
#         "portMappings":true
#       }
#     }
#   ]
# }
##
# I could not google any good way to change the config version, so I change it here:
ls -lah /var/lib/rancher/k3s/agent/etc/cni/net.d/
sed -i -e 's/1.0.0/0.3.1/' /var/lib/rancher/k3s/agent/etc/cni/net.d/10-flannel.conflist


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

echo "Install the operator and its webhook"
make deploy-test IMG="docker.io/demonihin/linkerd-multus-attach-operator:latest"

kubectl get pod -A
sleep 10
kubectl -n linkerd-multus-attach-operator-system describe pod

# kubectl -n linkerd-multus-attach-operator-system rollout status deployment/linkerd-multus-operator-controller-manager --timeout=120s
sleep 20 # Time to get lease and load WebHook TLS certificates