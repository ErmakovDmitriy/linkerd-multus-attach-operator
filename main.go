/*
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
*/

package main

import (
	"flag"
	"os"
	"strings"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	whapiv1 "github.com/ErmakovDmitriy/linkerd-multus-attach-operator/api/v1"
	"github.com/ErmakovDmitriy/linkerd-multus-attach-operator/controllers"
	"github.com/ErmakovDmitriy/linkerd-multus-attach-operator/k8s"
	netattachv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(netattachv1.AddToScheme(scheme))

	//+kubebuilder:scaffold:scheme
}

func main() {
	var (
		metricsAddr              string
		enableLeaderElection     bool
		probeAddr                string
		cniNamespace             string
		rawCNIKubeconfigFilePath string
		linkerdNamespace         string
		webHookPort              int

		allowedUIDAnnotationName string
		linkerdProxyUIDOffset    int64
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", true,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&cniNamespace, "cni-namespace", "linkerd-cni", "Namespace name in which Linkerd-CNI is installed")
	flag.StringVar(&rawCNIKubeconfigFilePath, "cni-kubeconfig", "/etc/cni/net.d/ZZZ-linkerd-cni-kubeconfig", "Linkerd-CNI Kubeconfig path")
	flag.StringVar(&linkerdNamespace, "linkerd-namespace", "linkerd", "Namespace name in which Linkerd is installed")
	flag.IntVar(&webHookPort, "webhook-port", 9443, "TCP port for webhook to listen on")
	flag.StringVar(&allowedUIDAnnotationName, "namespace-uid-range-annotation",
		k8s.NamespaceAllowedUIDRangeAnnotationDefault, "Namespace annotation name which should contain allowed container UID range in {{ first UID }}/{{ length }} format")
	flag.Int64Var(&linkerdProxyUIDOffset, "linkerd-proxy-uid-offset", k8s.LinkerdProxyUIDDefaultOffset, "Offset to add to the first allowed UID in a namespace to generate Linkerd proxy UID")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// I am not sure that this is a good way but it is better than preserve the quotes and pass them further.
	// The quotes being then embedded in the Linkerd-CNI configuration cause its failure so they must be removed.
	cniKubeconfigFilePath := strings.Trim(rawCNIKubeconfigFilePath, `\"`)

	setupLog.Info("Starting controller with parameters",
		"metrics-bind-addr", metricsAddr,
		"health-probe-bind-address", probeAddr,
		"enable-leader-election", enableLeaderElection,
		"cni-namespace", cniNamespace,
		"cni-kubeconfig", cniKubeconfigFilePath,
		"linkerd-namespace", linkerdNamespace,
		"webhook-port", webHookPort)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   webHookPort,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "1a7407a5.multus.linkerd.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.NamespaceReconciler{
		Client:                       mgr.GetClient(),
		Scheme:                       mgr.GetScheme(),
		LinkerdControlPlaneNamespace: linkerdNamespace,
		LinkerdCNINamespace:          cniNamespace,
		LinkerdCNIKubeconfigPath:     cniKubeconfigFilePath,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Namespace")
		os.Exit(1)
	}

	whapiv1.SetupWebhookWithManager(mgr, linkerdNamespace, allowedUIDAnnotationName, linkerdProxyUIDOffset)

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
