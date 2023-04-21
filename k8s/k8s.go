package k8s

import (
	pkgK8s "github.com/linkerd/linkerd2/pkg/k8s"
)

const (
	// MultusAttachAnnotation - annotation name to mark a namespace to create Multus NetworkAttach Definition in.
	MultusAttachAnnotation = pkgK8s.Prefix + "/multus"
	// LinkerdInjectAnnotation - pod or namespace annotation which enables Linkerd proxy inject.
	LinkerdInjectAnnotation = pkgK8s.ProxyInjectAnnotation
	// LinkerdProxyUIDAnnotation - annotation to set Linkerd proxy UID.
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

	// NamespaceAllowedUIDRangeAnnotationDefault - should contain allowed UID range
	// for a Pod's SecurityContext as it is done in Openshift.
	// The expected format is "{{ first UID }}/{{ length }}",
	// i.e. 100000/1000 means the range is 100000-101000.
	NamespaceAllowedUIDRangeAnnotationDefault = "openshift.io/sa.scc.uid-range"
	// LinkerdProxyUIDDefaultOffset - default UID offset from the
	// NamespaceAllowedUIDRangeAnnotationDefault (or overridden value)
	// which the Linkerd proxy will use in a namespace.
	// It is defined as 2102 because Linkerd proxy default UID is 2102
	// so I decided to use it as a "base" offset. No technical limitations
	// exist to use any other value, I think.
	// As the resulting proxy UID will be a namespace's {{ first allowed UID }} + {{ LinkerdProxyUIDDefaultOffset }}
	// it is expected that the UID range allows the proxy UID,
	// if not, the LinkerdProxyUIDDefaultOffset should be set to lower value.
	LinkerdProxyUIDDefaultOffset = 2102
)
