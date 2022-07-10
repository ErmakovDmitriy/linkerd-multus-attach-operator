# Linkerd Multus Network Attach Operator

The operator helps to use Linkerd-CNI in Kubernetes clusters with Multus installed
via creating NetworkAttachmentDefinitions and adding attach annotations to Pods.

## Description

The application contains 2 components:

1. NetworkAttachmentDefinitions controller
2. Mutating webhook

### NetworkAttachmentDefinitions controller

The controller performs watches namespaces for `linkerd.io/multus` annotation.
If the annotation's value is `enabled`, then the controller:

1. Loads the Linkerd-CNI ConfigMap from the CNI namespace (linkerd-cni by default)
2. Generates NetworkAttachmentDefinition spec from the ConfigMap and its settings
3. Creates the NetworkAttachmentDefinition in the namespace

If the annotation is not set or disabled, the NetworkAttachmentDefinition is deleted from
the namespace.

In addition, Linkerd control plane namespace always has the Multus NetworkAttachmentDefinition
present and the control plane Pods (based on `linkerd.io/control-plane-component` labels)
are always patched to attach the NetworkAttachmentDefinition.

The NetworkAttachmentDefinition's settings are configurable via the controller's flags:

| Flag               | Description                                                                                                                                     |
| ------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| -cni-namespace     | Namespace in which Linkerd CNI is installed. It is used to get the CNI ConfigMap                                                                |
| -linkerd-namespace | Namespace in which Linkerd control plane is installed. The control plane namespace must always have NetworkAttachmentDefinition for Linkerd CNI |
| -cni-kubeconfig    | Path on Kubernetes hosts where Linkerd CNI DaemonSet Pods put Kubeconfig                                                                        |

### Mutating Webhook

Mutating webhook adds `k8s.cni.cncf.io/v1=linkerd-cni` annotation to Pods which must be handled
by Linkerd CNI.
Before the decision is made, the webhook copies namespace annotations [linkerd.io/multus, linkerd.io/inject]
to a pod, if the pod does not have them defined.

The webhook adds the `k8s.cni.cncf.io/v1=linkerd-cni` annotation if any of items below is true:

* A Pod has `linkerd.io/multus=enabled` annotation
* A Pod is in Linkerd control plane namespace and has not empty `linkerd.io/control-plane-component` label


## Getting Started

You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster

0. Install [cert-manager](https://cert-manager.io/) as it is necessary to generate webhook certificates

1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

2. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<some-registry>/linkerd-multus-attach-operator:tag
```

3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/linkerd-multus-attach-operator:tag
```

### Uninstall CRDs

To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller

UnDeploy the controller to the cluster:

```sh
make undeploy
```

## Contributing

// TODO(user): Add detailed information on how you would like others to contribute to this project

### How it works

This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/)
which provides a reconcile function responsible for synchronizing resources untile the desired state is reached on the cluster

### Test It Out

1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions

If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

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
