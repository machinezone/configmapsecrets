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

	"bursavich.dev/zapr"
	"github.com/machinezone/configmapsecrets/pkg/api/v1alpha1"
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
	logCfg := zapr.DevelopmentConfig()
	logCfg.EnableStacktrace = false
	log.SetLogger(zapr.NewLogger(logCfg))
}

func TestMain(m *testing.M) {
	check(clientscheme.AddToScheme(scheme))
	check(v1alpha1.AddToScheme(scheme))
	testenv := &envtest.Environment{
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{"../../manifest"},
		},
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
	ctx, cancel := context.WithCancel(context.TODO())
	mgr, err := manager.New(cfg, manager.Options{
		Scheme: scheme,
		Logger: nil,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// bypass cache for test verification
	api, err := client.New(mgr.GetConfig(), client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := &testReconciler{
		cancel:  cancel,
		closed:  make(chan struct{}),
		mgr:     mgr,
		api:     api,
		waiters: make(map[types.NamespacedName]chan struct{}),
	}
	rec := ConfigMapSecret{testNotifyFn: r.notify}
	if err := rec.SetupWithManager(mgr); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	go func() {
		defer close(r.closed)
		r.err = mgr.Start(ctx)
	}()

	return r
}

func (r *testReconciler) close(t *testing.T) {
	r.cancel()
	<-r.closed
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
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

type T interface {
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Failed() bool
	FailNow()
}

func eventually(t *testing.T, timeout time.Duration, retry <-chan struct{}, test func(t T)) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		// run test
		s := &stubT{}
		func() {
			defer func() {
				if v := recover(); v != nil {
					if x, _ := v.(*stubT); x == s {
						return // fatal error in test
					}
					panic(v) // panic in test
				}
			}()
			test(s)
		}()
		if !s.failed {
			return // PASS
		}

		select {
		case <-retry:
			// run test again
		case <-timer.C:
			// timed out: run final test and let it PASS or FAIL
			test(t)
			return
		}
	}
}

type stubT struct {
	failed bool
}

func (t *stubT) Errorf(format string, args ...interface{}) {
	t.failed = true
}

func (t *stubT) Fatalf(format string, args ...interface{}) {
	t.Errorf(format, args...)
	t.FailNow()
}

func (t *stubT) Failed() bool { return t.failed }

func (t *stubT) FailNow() {
	t.failed = true
	panic(t)
}
