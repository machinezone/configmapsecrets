// Copyright 2019 Machine Zone, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package controllers

import (
	"context"
	stdlog "log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/zapr"
	"github.com/machinezone/configmapsecrets/pkg/api/v1alpha1"
	"github.com/machinezone/configmapsecrets/pkg/mzlog"
	"github.com/onsi/gomega"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	cfg    *rest.Config
	scheme = runtime.NewScheme()
)

func init() {
	logCfg := mzlog.DefaultConfig()
	logCfg.Level = zapcore.DebugLevel
	logCfg.Encoder = mzlog.ConsoleType
	// TODO(abursavich): remove CallerSkip when https://github.com/go-logr/zapr/issues/6 is fixed
	log.SetLogger(zapr.NewLogger(mzlog.NewZapLogger(logCfg).WithOptions(zap.AddCallerSkip(1))))
}

func TestMain(m *testing.M) {
	check(clientscheme.AddToScheme(scheme))
	check(v1alpha1.AddToScheme(scheme))
	testenv := &envtest.Environment{
		CRDDirectoryPaths: []string{"../../manifest"},
	}
	var err error
	cfg, err = testenv.Start()
	check(err)

	code := m.Run()
	check(testenv.Stop())
	os.Exit(code)
}

func check(err error) {
	if err != nil {
		stdlog.Fatalln(err)
	}
}

type testReconciler struct {
	cancel func()
	closed chan struct{}
	mgr    manager.Manager
	err    error
	api    client.Client

	mu      sync.Mutex
	waiters map[types.NamespacedName]chan struct{}
}

func newTestReconciler(t *testing.T) *testReconciler {
	g := gomega.NewWithT(t)
	ctx, cancel := context.WithCancel(context.TODO())
	mgr, err := manager.New(cfg, manager.Options{
		Scheme: scheme,
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// bypass cache for test verification
	api, err := client.New(mgr.GetConfig(), client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	r := &testReconciler{
		cancel:  cancel,
		closed:  make(chan struct{}),
		mgr:     mgr,
		api:     api,
		waiters: make(map[types.NamespacedName]chan struct{}),
	}
	rec := ConfigMapSecret{testNotifyFn: r.notify}
	g.Expect(rec.SetupWithManager(mgr)).NotTo(gomega.HaveOccurred())

	go func() {
		defer close(r.closed)
		r.err = mgr.Start(ctx.Done())
	}()

	return r
}

func (r *testReconciler) close(t *testing.T) {
	r.cancel()
	<-r.closed
	g := gomega.NewWithT(t)
	g.Expect(r.err).NotTo(gomega.HaveOccurred())
}

func (r *testReconciler) waiter(key types.NamespacedName) chan struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()

	ch, ok := r.waiters[key]
	if !ok {
		ch = make(chan struct{}, 1)
		r.waiters[key] = ch
	}
	return ch
}

func (r *testReconciler) notify(key types.NamespacedName) {
	select {
	case r.waiter(key) <- struct{}{}:
	default:
	}
}

func (r *testReconciler) wait(key types.NamespacedName) <-chan struct{} {
	return r.waiter(key)
}

func eventually(t *testing.T, timeout time.Duration, wait <-chan struct{}, test func(g *gomega.WithT)) {
	for {
		// only final test if succeeds or panics
		s := &stubT{}
		func() {
			defer func() {
				if v := recover(); v != nil {
					if x, _ := v.(*stubT); x != s {
						panic(v) // real panic
					}
				}
			}()
			test(gomega.NewWithT(s))
		}()
		if !s.fail {
			return // success
		}

		timer := time.NewTimer(timeout)
		select {
		case <-wait:
			timer.Stop()
		case <-timer.C:
			// final test: succeed or fail
			test(gomega.NewWithT(t))
			return
		}
	}
}

type stubT struct{ fail bool }

func (t *stubT) Fatalf(format string, args ...interface{}) {
	t.fail = true
	panic(t)
}
