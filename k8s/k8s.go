package k8s

import (
	pkgK8s "github.com/linkerd/linkerd2/pkg/k8s"
)

const (
	MultusAttachAnnotation    = pkgK8s.Prefix + "/multus"
	LinkerdInjectAnnotation   = pkgK8s.ProxyInjectAnnotation
	LinkerdProxyUIDAnnotation = pkgK8s.ProxyUIDAnnotation

	// MultusNetworkAttachmentDefinitionName is the name of a NetworkAttachmentDefinition
	// created in a namespace if MultusAttachAnnotation is enabled.
	MultusNetworkAttachmentDefinitionName = "linkerd-cni"

	// MultusCNIVersion is a CNI version implemented by Linkerd.
	MultusCNIVersion = "0.3.0"

	// MultusCNIType is Linkerd CNI type field value.
	MultusCNIType = "linkerd-cni"

	// MultusNetworkAttachmentDefinitionAPIVersion is API version of multus
	// MultusNetworkAttachmentDefinition resource.
	MultusNetworkAttachmentDefinitionAPIVersion = "k8s.cni.cncf.io/v1"

	// MultusNetworkAttachmentDefinitionKind is Kind of NetworkAttachmentDefinition.
	MultusNetworkAttachmentDefinitionKind = "NetworkAttachmentDefinition"

	// LinkerdCNIConfigMapName is the name of Linkerd CNI ConfigMap.
	LinkerdCNIConfigMapName = "linkerd-cni-config"

	// LinkerdCNIConfigMapKey is the key in the LinkerdCNIConfigMapName
	// which stores Linkerd CNI config.
	LinkerdCNIConfigMapKey = "cni_network_config"

	// MultusAttachEnabled is assigned to MultusAttachAnnotation to enable
	// NetworkAttachmentDefinition creation in a namespace.
	MultusAttachEnabled = pkgK8s.Enabled

	// MultusAttachDisabled is assigned to MultusAttachAnnotation to disable (also default)
	// NetworkAttachmentDefinition creation in a namespace.
	MultusAttachDisabled = pkgK8s.Disabled

	// MultusNetworkAttachAnnotation is annotation which triggers Multus to
	// run CNI plugins.
	MultusNetworkAttachAnnotation = "k8s.v1.cni.cncf.io/networks"

	OpenshiftNamespaceAllowedUIDsAnnotation = "openshift.io/sa.scc.uid-range"
)
