/*

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
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"bursavich.dev/zapr"
	"github.com/machinezone/configmapsecrets/pkg/api/v1alpha1"
	"github.com/machinezone/configmapsecrets/pkg/buildinfo"
	"github.com/machinezone/configmapsecrets/pkg/controllers"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	clientscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
	// +kubebuilder:scaffold:imports
)

var (
	logger zapr.Logger
	scheme = runtime.NewScheme()
)

func init() {
	check(clientscheme.AddToScheme(scheme), "Unable to add kubernetes client set to scheme")
	check(v1alpha1.AddToScheme(scheme), "Unable to add secrets.mz.com/v1alpha1 to scheme")
	// +kubebuilder:scaffold:scheme
}

// RBAC for leader election, scoped to the namespace used by default manifests (kube-system).
//
// N.B.: RBAC cannot use resourceNames with "list", "watch", "create", or "deletecollection" verbs.
//
// +kubebuilder:rbac:namespace=kube-system,groups=core,resources=configmaps,verbs=create
// +kubebuilder:rbac:namespace=kube-system,groups=core,resources=configmaps,verbs=get;update,resourceNames=configmapsecret-controller-leader
// +kubebuilder:rbac:namespace=kube-system,groups=core,resources=events,verbs=create;patch

func main() {
	var (
		healthAddr              string
		metricsAddr             string
		allNamespaces           bool
		leaderElection          bool
		leaderElectionNamespace string
	)
	flag.StringVar(&healthAddr, "health-addr", ":9090", "The address to which the health endpoint binds.")
	flag.StringVar(&metricsAddr, "metrics-addr", ":9091", "The address to which the metric endpoint binds.")
	flag.BoolVar(&allNamespaces, "all-namespaces", true,
		"Enable the contoller to manage all namespaces, instead of only its own namespace.")
	flag.BoolVar(&leaderElection, "enable-leader-election", false,
		"Enable leader election, which will ensure there is only one active controller.")
	flag.StringVar(&leaderElectionNamespace, "leader-election-namespace", "",
		"Namespace of leader election object. Defaults to `kube-system` when all-namespaces is enabled "+
			"and to the controller's own namespace when all-namespaces is disabled.")
	logCfg := zapr.DefaultConfig().RegisterCommonFlags(flag.CommandLine)
	flag.Parse()

	logMetrics := zapr.NewPrometheusMetrics()
	logCfg.Metrics = logMetrics

	logger = zapr.NewLogger(logCfg)
	buildinfo.Log(logger)
	log.SetLogger(logger)

	check(metrics.Registry.Register(logMetrics), "Unable to register logging metrics")
	check(metrics.Registry.Register(buildinfo.Collector()), "Unable to register build metrics")

	cfg, err := config.GetConfig()
	check(err, "Unable to load kubeconfig")

	namespace := ""
	electionNamespace := "kube-system" // Default to cluster-wide leader election.
	if !allNamespaces {
		namespace, err = currentNamespace()
		check(err, "Unable to detect namespace")
		electionNamespace = namespace // Default to namespace-wide leader election.
	}
	if leaderElectionNamespace != "" {
		electionNamespace = leaderElectionNamespace // Override leader election namespace.
	}
	opts := manager.Options{
		Scheme:                  scheme,
		HealthProbeBindAddress:  healthAddr,
		MetricsBindAddress:      metricsAddr,
		Namespace:               namespace,
		LeaderElection:          leaderElection,
		LeaderElectionID:        "configmapsecret-controller-leader",
		LeaderElectionNamespace: electionNamespace,
	}

	mgr, err := manager.New(cfg, opts)
	check(err, "Unable to create manager")
	check(mgr.AddHealthzCheck("ping", healthz.Ping), "Unable to install healthz check")

	rec := controllers.ConfigMapSecret{}
	check(rec.SetupWithManager(mgr), "Unable to create controller")
	// +kubebuilder:scaffold:builder

	logger.Info("Starting manager")
	stopCh := signals.SetupSignalHandler()
	check(mgr.Start(stopCh), "Problem running manager")
}

func currentNamespace() (string, error) {
	buf, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(buf)), nil
}

func check(err error, msg string) {
	if err == nil {
		return
	}
	if logger == nil {
		fmt.Fprintf(os.Stderr, "%s: %v", msg, err)
		os.Exit(1)
	}
	logger.Underlying().WithOptions(zap.AddCallerSkip(1)).Fatal(msg, zap.Error(err))
}
