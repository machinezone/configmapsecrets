// Copyright 2019 Machine Zone, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package controllers

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/onsi/gomega"

	"github.com/machinezone/configmapsecrets/pkg/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const timeout = time.Second * 5

func runTests(t *testing.T, tests []test) {
	// g := gomega.NewWithT(t)
	r := newTestReconciler(t)
	var wg sync.WaitGroup
	for _, test := range tests {
		test := test
		test.run(context.TODO(), &wg, t, r)
	}
	t.Run("clean-up", func(t *testing.T) {
		t.Parallel()
		wg.Wait()
		r.close(t)
	})
}

type step func(context.Context, *testing.T, *testReconciler)

type test struct {
	name     string
	steps    []step
	subTests []test
	parallel bool
}

func (test *test) run(ctx context.Context, wg *sync.WaitGroup, t *testing.T, r *testReconciler) {
	wg.Add(1)
	t.Run(test.name, func(t *testing.T) {
		defer wg.Done()
		test := test
		if test.parallel {
			t.Parallel()
		}
		for _, step := range test.steps {
			step(ctx, t, r)
		}
		for _, tt := range test.subTests {
			tt.run(ctx, wg, t, r)
		}
	})
}

func TestReconciler(t *testing.T) {
	runTests(t, []test{

		{
			name: "labels",
			steps: []step{
				createConfigMapSecretStep(&v1alpha1.ConfigMapSecret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "labels",
						Namespace: "default",
					},
					Spec: v1alpha1.ConfigMapSecretSpec{
						Template: v1alpha1.ConfigMapTemplate{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"foo": "bar",
								},
							},
						},
					},
				}),
				checkSecretStep(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "labels",
						Namespace: "default",
						Labels: map[string]string{
							"foo": "bar",
						},
					},
				}),
				checkStatusStep(true, types.NamespacedName{
					Name:      "labels",
					Namespace: "default",
				}),
			},
			subTests: []test{
				{
					name: "update-labels",
					steps: []step{
						updateConfigMapSecretStep(
							types.NamespacedName{
								Name:      "labels",
								Namespace: "default",
							},
							func(obj *v1alpha1.ConfigMapSecret) {
								obj.Spec.Template.ObjectMeta = metav1.ObjectMeta{
									Labels: map[string]string{
										"foo": "abc",
										"bar": "xyz",
									},
								}
							},
						),
						checkSecretStep(&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "labels",
								Namespace: "default",
								Labels: map[string]string{
									"foo": "abc",
									"bar": "xyz",
								},
							},
						}),
						checkStatusStep(true, types.NamespacedName{
							Name:      "labels",
							Namespace: "default",
						}),
					},
				},
			},
			parallel: true,
		},

		{
			name: "annotations",
			steps: []step{
				createConfigMapSecretStep(&v1alpha1.ConfigMapSecret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "annotations",
						Namespace: "default",
					},
					Spec: v1alpha1.ConfigMapSecretSpec{
						Template: v1alpha1.ConfigMapTemplate{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									"foo": "bar",
								},
							},
						},
					},
				}),
				checkSecretStep(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "annotations",
						Namespace: "default",
						Annotations: map[string]string{
							"foo": "bar",
						},
					},
				}),
				checkStatusStep(true, types.NamespacedName{
					Name:      "annotations",
					Namespace: "default",
				}),
			},
			subTests: []test{
				{
					name: "update-annotations",
					steps: []step{
						updateConfigMapSecretStep(
							types.NamespacedName{
								Name:      "annotations",
								Namespace: "default",
							},
							func(obj *v1alpha1.ConfigMapSecret) {
								obj.Spec.Template.ObjectMeta = metav1.ObjectMeta{
									Annotations: map[string]string{
										"foo": "abc",
										"bar": "xyz",
									},
								}
							},
						),
						checkSecretStep(&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "annotations",
								Namespace: "default",
								Annotations: map[string]string{
									"foo": "abc",
									"bar": "xyz",
								},
							},
						}),
						checkStatusStep(true, types.NamespacedName{
							Name:      "annotations",
							Namespace: "default",
						}),
					},
				},
			},
			parallel: true,
		},

		{
			name: "no-values",
			steps: []step{
				createConfigMapSecretStep(&v1alpha1.ConfigMapSecret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "no-values",
						Namespace: "default",
					},
					Spec: v1alpha1.ConfigMapSecretSpec{
						Template: v1alpha1.ConfigMapTemplate{
							Data: map[string]string{
								"test-key": "test-data",
							},
						},
					},
				}),
				checkSecretStep(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "no-values",
						Namespace: "default",
					},
					Data: map[string][]byte{
						"test-key": []byte("test-data"),
					},
				}),
				checkStatusStep(true, types.NamespacedName{
					Name:      "no-values",
					Namespace: "default",
				}),
			},
			subTests: []test{
				{
					name: "update-data",
					steps: []step{
						updateConfigMapSecretStep(
							types.NamespacedName{Namespace: "default", Name: "no-values"},
							func(obj *v1alpha1.ConfigMapSecret) {
								obj.Spec = v1alpha1.ConfigMapSecretSpec{
									Template: v1alpha1.ConfigMapTemplate{
										Data: map[string]string{
											"test-key": "hello, world",
										},
									},
								}
							},
						),
						checkSecretStep(&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "no-values",
								Namespace: "default",
							},
							Data: map[string][]byte{
								"test-key": []byte("hello, world"),
							},
						}),
						checkStatusStep(true, types.NamespacedName{
							Name:      "no-values",
							Namespace: "default",
						}),
					},
				},
			},
			parallel: true,
		},

		{
			name: "values",
			steps: []step{
				createConfigMapSecretStep(&v1alpha1.ConfigMapSecret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "values",
						Namespace: "default",
					},
					Spec: v1alpha1.ConfigMapSecretSpec{
						Template: v1alpha1.ConfigMapTemplate{
							Data: map[string]string{
								"foo": "foo: $(FOO)",
								"bar": "bar: $(BAR)",
							},
						},
						Vars: []v1alpha1.TemplateVariable{
							{
								Name:  "FOO",
								Value: "abc",
							},
							{
								Name:  "BAR",
								Value: "xyz",
							},
						},
					},
				}),
				checkSecretStep(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "values",
						Namespace: "default",
					},
					Data: map[string][]byte{
						"foo": []byte("foo: abc"),
						"bar": []byte("bar: xyz"),
					},
				}),
				checkStatusStep(true, types.NamespacedName{
					Name:      "values",
					Namespace: "default",
				}),
			},
			subTests: []test{
				{
					name: "update-vars",
					steps: []step{
						updateConfigMapSecretStep(
							types.NamespacedName{
								Name:      "values",
								Namespace: "default",
							},
							func(obj *v1alpha1.ConfigMapSecret) {
								obj.Spec.Vars = []v1alpha1.TemplateVariable{
									{
										Name:  "FOO",
										Value: "abc",
									},
								}
							},
						),
						checkSecretStep(&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "values",
								Namespace: "default",
							},
							Data: map[string][]byte{
								"foo": []byte("foo: abc"),
								"bar": []byte("bar: $(BAR)"),
							},
						}),
						checkStatusStep(true, types.NamespacedName{
							Name:      "values",
							Namespace: "default",
						}),
					},
				},
			},
			parallel: true,
		},

		{
			name: "secrets",
			steps: []step{
				createSecretStep(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secrets-foobar",
						Namespace: "default",
					},
					StringData: map[string]string{
						"foo": "abc",
						"bar": "xyz",
					},
				}),
				createSecretStep(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secrets-baz",
						Namespace: "default",
					},
					StringData: map[string]string{
						"baz": "qux",
					},
				}),
				createConfigMapSecretStep(&v1alpha1.ConfigMapSecret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secrets",
						Namespace: "default",
					},
					Spec: v1alpha1.ConfigMapSecretSpec{
						Template: v1alpha1.ConfigMapTemplate{
							Data: map[string]string{
								"foo": "foo: $(FOO)",
								"bar": "bar: $(BAR)",
								"baz": "baz: $(BAZ)",
							},
						},
						Vars: []v1alpha1.TemplateVariable{
							{
								Name: "FOO",
								SecretValue: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "secrets-foobar",
									},
									Key: "foo",
								},
							},
							{
								Name: "BAR",
								SecretValue: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "secrets-foobar",
									},
									Key:      "bar",
									Optional: boolPtr(true),
								},
							},
							{
								Name: "BAZ",
								SecretValue: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "secrets-baz",
									},
									Key:      "baz",
									Optional: boolPtr(true),
								},
							},
						},
					},
				}),
				checkSecretStep(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secrets",
						Namespace: "default",
					},
					Data: map[string][]byte{
						"foo": []byte("foo: abc"),
						"bar": []byte("bar: xyz"),
						"baz": []byte("baz: qux"),
					},
				}),
				checkStatusStep(true, types.NamespacedName{
					Name:      "secrets",
					Namespace: "default",
				}),
			},
			subTests: []test{
				{
					name: "update-secret",
					steps: []step{
						updateSecretStep(
							types.NamespacedName{
								Name:      "secrets-foobar",
								Namespace: "default",
							},
							func(obj *corev1.Secret) {
								obj.Data = nil
								obj.StringData = map[string]string{
									"foo": "abc",
									"bar": "updated",
								}
							},
						),
						checkSecretStep(&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "secrets",
								Namespace: "default",
							},
							Data: map[string][]byte{
								"foo": []byte("foo: abc"),
								"bar": []byte("bar: updated"),
								"baz": []byte("baz: qux"),
							},
						}),
						checkStatusStep(true, types.NamespacedName{
							Name:      "secrets",
							Namespace: "default",
						}),
					},
				},
				{
					name: "delete-optional-secret-key",
					steps: []step{
						updateSecretStep(
							types.NamespacedName{
								Name:      "secrets-foobar",
								Namespace: "default",
							},
							func(obj *corev1.Secret) {
								obj.Data = nil
								obj.StringData = map[string]string{
									"foo": "abc",
								}
							},
						),
						checkSecretStep(&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "secrets",
								Namespace: "default",
							},
							Data: map[string][]byte{
								"foo": []byte("foo: abc"),
								"bar": []byte("bar: $(BAR)"),
								"baz": []byte("baz: qux"),
							},
						}),
						checkStatusStep(true, types.NamespacedName{
							Name:      "secrets",
							Namespace: "default",
						}),
					},
				},
				{
					name: "delete-optional-secret",
					steps: []step{
						deleteSecretStep(types.NamespacedName{
							Name:      "secrets-baz",
							Namespace: "default",
						}),
						checkSecretStep(&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "secrets",
								Namespace: "default",
							},
							Data: map[string][]byte{
								"foo": []byte("foo: abc"),
								"bar": []byte("bar: $(BAR)"),
								"baz": []byte("baz: $(BAZ)"),
							},
						}),
						checkStatusStep(true, types.NamespacedName{
							Name:      "secrets",
							Namespace: "default",
						}),
					},
				},
				{
					name: "delete-vars",
					steps: []step{
						updateConfigMapSecretStep(
							types.NamespacedName{
								Name:      "secrets",
								Namespace: "default",
							},
							func(obj *v1alpha1.ConfigMapSecret) {
								obj.Spec.Vars = nil
							},
						),
						checkSecretStep(&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "secrets",
								Namespace: "default",
							},
							Data: map[string][]byte{
								"foo": []byte("foo: $(FOO)"),
								"bar": []byte("bar: $(BAR)"),
								"baz": []byte("baz: $(BAZ)"),
							},
						}),
						checkStatusStep(true, types.NamespacedName{
							Name:      "secrets",
							Namespace: "default",
						}),
					},
				},
			},
			parallel: true,
		},

		{
			name: "configmaps",
			steps: []step{
				createConfigMapStep(&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "configmaps-foobar",
						Namespace: "default",
					},
					Data: map[string]string{
						"foo": "abc",
						"bar": "xyz",
					},
				}),
				createConfigMapStep(&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "configmaps-baz",
						Namespace: "default",
					},
					BinaryData: map[string][]byte{
						"baz": []byte("qux"),
					},
				}),
				createConfigMapSecretStep(&v1alpha1.ConfigMapSecret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "configmaps",
						Namespace: "default",
					},
					Spec: v1alpha1.ConfigMapSecretSpec{
						Template: v1alpha1.ConfigMapTemplate{
							Data: map[string]string{
								"foo": "foo: $(FOO)",
								"bar": "bar: $(BAR)",
								"baz": "baz: $(BAZ)",
							},
							BinaryData: map[string][]byte{
								"qux": []byte("$(FOO)"),
							},
						},
						Vars: []v1alpha1.TemplateVariable{
							{
								Name: "FOO",
								ConfigMapValue: &corev1.ConfigMapKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "configmaps-foobar",
									},
									Key: "foo",
								},
							},
							{
								Name: "BAR",
								ConfigMapValue: &corev1.ConfigMapKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "configmaps-foobar",
									},
									Key:      "bar",
									Optional: boolPtr(true),
								},
							},
							{
								Name: "BAZ",
								ConfigMapValue: &corev1.ConfigMapKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "configmaps-baz",
									},
									Key:      "baz",
									Optional: boolPtr(true),
								},
							},
						},
					},
				}),
				checkSecretStep(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "configmaps",
						Namespace: "default",
					},
					Data: map[string][]byte{
						"foo": []byte("foo: abc"),
						"bar": []byte("bar: xyz"),
						"baz": []byte("baz: qux"),
						"qux": []byte("abc"),
					},
				}),
				checkStatusStep(true, types.NamespacedName{
					Name:      "configmaps",
					Namespace: "default",
				}),
			},
			subTests: []test{
				{
					name: "update-configmap",
					steps: []step{
						updateConfigMapStep(
							types.NamespacedName{
								Name:      "configmaps-foobar",
								Namespace: "default",
							},
							func(obj *corev1.ConfigMap) {
								obj.Data = map[string]string{
									"foo": "abc",
									"bar": "updated",
								}
							},
						),
						checkSecretStep(&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "configmaps",
								Namespace: "default",
							},
							Data: map[string][]byte{
								"foo": []byte("foo: abc"),
								"bar": []byte("bar: updated"),
								"baz": []byte("baz: qux"),
								"qux": []byte("abc"),
							},
						}),
						checkStatusStep(true, types.NamespacedName{
							Name:      "configmaps",
							Namespace: "default",
						}),
					},
				},
				{
					name: "delete-optional-configmap-key",
					steps: []step{
						updateConfigMapStep(
							types.NamespacedName{
								Name:      "configmaps-foobar",
								Namespace: "default",
							},
							func(obj *corev1.ConfigMap) {
								obj.Data = map[string]string{
									"foo": "abc",
								}
							},
						),
						checkSecretStep(&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "configmaps",
								Namespace: "default",
							},
							Data: map[string][]byte{
								"foo": []byte("foo: abc"),
								"bar": []byte("bar: $(BAR)"),
								"baz": []byte("baz: qux"),
								"qux": []byte("abc"),
							},
						}),
						checkStatusStep(true, types.NamespacedName{
							Name:      "configmaps",
							Namespace: "default",
						}),
					},
				},
				{
					name: "delete-optional-configmap",
					steps: []step{
						deleteConfigMapStep(types.NamespacedName{
							Name:      "configmaps-baz",
							Namespace: "default",
						}),
						checkSecretStep(&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "configmaps",
								Namespace: "default",
							},
							Data: map[string][]byte{
								"foo": []byte("foo: abc"),
								"bar": []byte("bar: $(BAR)"),
								"baz": []byte("baz: $(BAZ)"),
								"qux": []byte("abc"),
							},
						}),
						checkStatusStep(true, types.NamespacedName{
							Name:      "configmaps",
							Namespace: "default",
						}),
					},
				},
			},
			parallel: true,
		},

		{
			name: "render-failure",
			steps: []step{
				createConfigMapSecretStep(&v1alpha1.ConfigMapSecret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "render-failure",
						Namespace: "default",
					},
					Spec: v1alpha1.ConfigMapSecretSpec{
						Template: v1alpha1.ConfigMapTemplate{
							Data: map[string]string{
								"hello": "$(NAME)",
							},
						},
						Vars: []v1alpha1.TemplateVariable{
							{
								Name: "NAME",
								SecretValue: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "render-failure-name",
									},
									Key: "name",
								},
							},
						},
					},
				}),
				waitStep(types.NamespacedName{
					Name:      "render-failure",
					Namespace: "default",
				}),
				checkStatusStep(false, types.NamespacedName{
					Name:      "render-failure",
					Namespace: "default",
				}),
			},
			subTests: []test{
				{
					name: "create-secret",
					steps: []step{
						createSecretStep(&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "render-failure-name",
								Namespace: "default",
							},
							StringData: map[string]string{
								"name": "world",
							},
						}),
						checkSecretStep(&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "render-failure",
								Namespace: "default",
							},
							Data: map[string][]byte{
								"hello": []byte("world"),
							},
						}),
						checkStatusStep(true, types.NamespacedName{
							Name:      "render-failure",
							Namespace: "default",
						}),
					},
				},
			},
			parallel: true,
		},
	})
}

func createConfigMapSecretStep(obj *v1alpha1.ConfigMapSecret) step {
	return func(ctx context.Context, t *testing.T, r *testReconciler) {
		t.Run("create-configmapsecret", func(t *testing.T) {
			g := gomega.NewWithT(t)
			err := r.api.Create(ctx, obj)
			g.Expect(err).NotTo(gomega.HaveOccurred(), "Create ConfigMapSecret")
		})
	}
}

func updateConfigMapSecretStep(key types.NamespacedName, fn func(obj *v1alpha1.ConfigMapSecret)) step {
	return func(ctx context.Context, t *testing.T, r *testReconciler) {
		t.Run("update-configmapsecret", func(t *testing.T) {
			g := gomega.NewWithT(t)
			for {
				obj := &v1alpha1.ConfigMapSecret{}
				g.Expect(r.api.Get(ctx, key, obj)).NotTo(gomega.HaveOccurred(), "Get ConfigMapSecret")
				fn(obj)
				if err := r.api.Update(ctx, obj); !errors.IsConflict(err) {
					g.Expect(err).NotTo(gomega.HaveOccurred(), "Update ConfigMapSecret")
					return
				}
			}
		})
	}
}

func deleteConfigMapSecretStep(key types.NamespacedName) step {
	return func(ctx context.Context, t *testing.T, r *testReconciler) {
		t.Run("delete-configmapsecret", func(t *testing.T) {
			g := gomega.NewWithT(t)
			err := r.api.Delete(ctx, &v1alpha1.ConfigMapSecret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: key.Namespace,
					Name:      key.Name,
				},
			})
			g.Expect(err).NotTo(gomega.HaveOccurred(), "Delete ConfigMapSecret")
		})
	}
}

func checkStatusStep(ok bool, key types.NamespacedName) step {
	return func(ctx context.Context, t *testing.T, r *testReconciler) {
		t.Run("check-status", func(t *testing.T) {
			g := gomega.NewWithT(t)
			obj := &v1alpha1.ConfigMapSecret{}
			g.Expect(r.api.Get(ctx, key, obj)).NotTo(gomega.HaveOccurred(), "Get ConfigMapSecret")
			g.Expect(obj.Status.ObservedGeneration).To(gomega.Equal(obj.Generation), "Observed Generation")
			g.Expect(obj.Status.Conditions).To(gomega.HaveLen(1), "Conditions")
			cond := obj.Status.Conditions[0]
			g.Expect(cond.Type).To(gomega.Equal(v1alpha1.ConfigMapSecretRenderFailure), "Condition Type")
			if ok {
				g.Expect(cond.Status).To(gomega.Equal(corev1.ConditionFalse), "Condition Status")
			} else {
				g.Expect(cond.Status).To(gomega.Equal(corev1.ConditionTrue), "Condition Status")
				g.Expect(cond.Reason).To(gomega.Equal(CreateVariablesErrorReason), "Condition Reason")
			}
		})
	}
}

func createConfigMapStep(obj *corev1.ConfigMap) step {
	return func(ctx context.Context, t *testing.T, r *testReconciler) {
		t.Run("create-configmap", func(t *testing.T) {
			g := gomega.NewWithT(t)
			err := r.api.Create(ctx, obj)
			g.Expect(err).NotTo(gomega.HaveOccurred(), "Create ConfigMap")
		})
	}
}

func updateConfigMapStep(key types.NamespacedName, fn func(obj *corev1.ConfigMap)) step {
	return func(ctx context.Context, t *testing.T, r *testReconciler) {
		t.Run("update-configmap", func(t *testing.T) {
			g := gomega.NewWithT(t)
			for {
				obj := &corev1.ConfigMap{}
				g.Expect(r.api.Get(ctx, key, obj)).NotTo(gomega.HaveOccurred(), "Get ConfigMap")
				fn(obj)
				if err := r.api.Update(ctx, obj); !errors.IsConflict(err) {
					g.Expect(err).NotTo(gomega.HaveOccurred(), "Update ConfigMap")
					return
				}
			}
		})
	}
}

func deleteConfigMapStep(key types.NamespacedName) step {
	return func(ctx context.Context, t *testing.T, r *testReconciler) {
		t.Run("delete-configmap", func(t *testing.T) {
			g := gomega.NewWithT(t)
			err := r.api.Delete(ctx, &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: key.Namespace,
					Name:      key.Name,
				},
			})
			g.Expect(err).NotTo(gomega.HaveOccurred(), "Delete ConfigMap")
		})
	}
}

func createSecretStep(obj *corev1.Secret) step {
	return func(ctx context.Context, t *testing.T, r *testReconciler) {
		t.Run("create-secret", func(t *testing.T) {
			g := gomega.NewWithT(t)
			err := r.api.Create(ctx, obj)
			g.Expect(err).NotTo(gomega.HaveOccurred(), "Create Secret")
		})
	}
}

func updateSecretStep(key types.NamespacedName, fn func(obj *corev1.Secret)) step {
	return func(ctx context.Context, t *testing.T, r *testReconciler) {
		t.Run("update-secret", func(t *testing.T) {
			g := gomega.NewWithT(t)
			for {
				obj := &corev1.Secret{}
				g.Expect(r.api.Get(ctx, key, obj)).NotTo(gomega.HaveOccurred(), "Get Secret")
				fn(obj)
				if err := r.api.Update(ctx, obj); !errors.IsConflict(err) {
					g.Expect(err).NotTo(gomega.HaveOccurred(), "Update Secret")
					return
				}
			}
		})
	}
}

func deleteSecretStep(key types.NamespacedName) step {
	return func(ctx context.Context, t *testing.T, r *testReconciler) {
		t.Run("delete-secret", func(t *testing.T) {
			g := gomega.NewWithT(t)
			err := r.api.Delete(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: key.Namespace,
					Name:      key.Name,
				},
			})
			g.Expect(err).NotTo(gomega.HaveOccurred(), "Delete Secret")
		})
	}
}

func checkSecretStep(want *corev1.Secret) step {
	return func(ctx context.Context, t *testing.T, r *testReconciler) {
		t.Run("check-secret", func(t *testing.T) {
			key := types.NamespacedName{Name: want.GetName(), Namespace: want.GetNamespace()}
			eventually(t, timeout, r.wait(key), func(g *gomega.WithT) {
				got := &corev1.Secret{}
				g.Expect(r.api.Get(ctx, key, got)).NotTo(gomega.HaveOccurred(), "Secret exists")
				g.Expect(got.GetLabels()).To(gomega.Equal(want.GetLabels()), "Secret labels match")
				g.Expect(got.GetAnnotations()).To(gomega.Equal(want.GetAnnotations()), "Secret annotations match")
				g.Expect(got.Data).To(gomega.Equal(want.Data), "Secret data matches")
			})
		})
	}
}

func waitStep(key types.NamespacedName) step {
	return func(ctx context.Context, t *testing.T, r *testReconciler) {
		t.Run("wait", func(t *testing.T) {
			timer := time.NewTimer(timeout)
			defer timer.Stop()
			select {
			case <-r.wait(key):
			case <-timer.C:
				t.Fatal("timeout")
			}
		})
	}
}

func boolPtr(v bool) *bool { return &v }
