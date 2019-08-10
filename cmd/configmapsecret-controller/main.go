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
	"flag"

	"github.com/go-logr/zapr"
	"github.com/machinezone/configmapsecrets/pkg/api/v1alpha1"
	"github.com/machinezone/configmapsecrets/pkg/buildinfo"
	"github.com/machinezone/configmapsecrets/pkg/controllers"
	"github.com/machinezone/configmapsecrets/pkg/mzlog"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	clientscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// +kubebuilder:scaffold:imports
)

var scheme = runtime.NewScheme()

func init() {
	clientscheme.AddToScheme(scheme)
	v1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	logCfg := mzlog.DefaultConfig().RegisterCommonFlags(flag.CommandLine)
	flag.Parse()

	logger := mzlog.NewZapLogger(logCfg)
	mzlog.Process(logger)
	// TODO(abursavich): remove CallerSkip when https://github.com/go-logr/zapr/issues/6 is fixed
	log.SetLogger(zapr.NewLogger(logger.WithOptions(zap.AddCallerSkip(1))))

	metrics.Registry.Register(logCfg.Metrics)
	metrics.Registry.Register(buildinfo.Collector())

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "configmapsecret-controller-leader",
	})
	if err != nil {
		logger.Fatal("Unable to start manager", zap.Error(err))
	}

	var rec controllers.ConfigMapSecret
	if err := rec.SetupWithManager(mgr); err != nil {
		logger.Fatal("Unable to create controller", zap.Error(err))
	}
	// +kubebuilder:scaffold:builder

	logger.Info("Starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logger.Fatal("Problem running manager", zap.Error(err))
	}
}
