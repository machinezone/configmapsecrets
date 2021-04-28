// Copyright 2019 Machine Zone, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package controllers

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	"github.com/machinezone/configmapsecrets/pkg/api/v1alpha1"
	"github.com/machinezone/configmapsecrets/third_party/kubernetes/forked/golang/expansion"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var missingValues = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "configmapsecret_controller_missing_value_render_errors_total",
	Help: "Total number of ConfigMapSecret controller render errors due to missing required values.",
}, []string{"namespace"})

func init() {
	metrics.Registry.MustRegister(missingValues)
}

// ConfigMapSecret reconciles a ConfigMapSecret object
type ConfigMapSecret struct {
	client   client.Client
	scheme   *runtime.Scheme
	logger   logr.Logger
	recorder record.EventRecorder

	mu         sync.RWMutex
	secrets    refMap
	configMaps refMap
	owned      refMap

	testNotifyFn func(types.NamespacedName)
}

// SetupWithManager sets up the reconciler with the manager.
func (r *ConfigMapSecret) SetupWithManager(manager manager.Manager) error {
	r.client = manager.GetClient()
	r.scheme = manager.GetScheme()
	r.logger = manager.GetLogger().WithName("controller").WithName("ConfigMapSecret")
	r.recorder = manager.GetEventRecorderFor("configmapsecret-controller")

	return builder.ControllerManagedBy(manager).
		For(&v1alpha1.ConfigMapSecret{}).
		Watches(&source.Kind{Type: &corev1.Secret{}}, handler.Funcs{
			CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
				r.secretEventHandler(q, e.Object.(*corev1.Secret), false)
			},
			UpdateFunc: func(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
				r.secretEventHandler(q, e.ObjectNew.(*corev1.Secret), false)
			},
			DeleteFunc: func(e event.DeleteEvent, q workqueue.RateLimitingInterface) {
				r.secretEventHandler(q, e.Object.(*corev1.Secret), true)
			},
			GenericFunc: func(e event.GenericEvent, q workqueue.RateLimitingInterface) {
				r.secretEventHandler(q, e.Object.(*corev1.Secret), false)
			},
		}).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}}, r.configMapEventHandler()).
		Complete(r)
}

func (r *ConfigMapSecret) configMapEventHandler() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
		namespace := obj.GetNamespace()
		name := obj.GetName()

		r.mu.RLock()
		defer r.mu.RUnlock()
		return toReqs(namespace, r.configMaps.srcs(namespace, name))
	})
}

func (r *ConfigMapSecret) secretEventHandler(q workqueue.RateLimitingInterface, secret *corev1.Secret, deleted bool) {
	name := secret.Name
	namespace := secret.Namespace
	owner := getOwner(secret)

	r.mu.Lock()
	if deleted || owner == nil {
		r.owned.set(namespace, name, nil)
	} else {
		r.owned.set(namespace, name, map[string]bool{string(owner.UID): true})
	}
	cmsNames := keys(r.secrets.srcs(namespace, name))
	r.mu.Unlock()

	if owner != nil {
		q.Add(reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: namespace,
				Name:      owner.Name,
			},
		})
	}
	for _, cmsName := range cmsNames {
		if owner != nil && owner.Name == cmsName {
			continue
		}
		q.Add(reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: namespace,
				Name:      cmsName,
			},
		})
	}
}

func (r *ConfigMapSecret) setRefs(namespace, name string, secrets, configMaps map[string]bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.secrets.set(namespace, name, secrets)
	r.configMaps.set(namespace, name, configMaps)
}

// +kubebuilder:rbac:groups=core,resources=events,verbs=create;update
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=secrets.mz.com,resources=configmapsecrets,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=secrets.mz.com,resources=configmapsecrets/status;configmapsecrets/finalizers,verbs=get;update;patch

// Reconcile reconciles the state of the cluster with the desired state of a
// ConfigMapSecret.
func (r *ConfigMapSecret) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	if r.testNotifyFn != nil {
		defer r.testNotifyFn(req.NamespacedName)
	}
	log := r.logger.WithValues("configmapsecret", req.NamespacedName)

	// Fetch the ConfigMapSecret instance
	cms := &v1alpha1.ConfigMapSecret{}
	if err := r.client.Get(ctx, req.NamespacedName, cms); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found. Owned objects are automatically garbage collected.
			r.setRefs(req.Namespace, req.Name, nil, nil)
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	// Set the Secret and ConfigMap references for the instance
	secretNames, configMapNames := varRefs(cms.Spec.VarsFrom, cms.Spec.Vars)
	r.setRefs(cms.Namespace, cms.Name, secretNames, configMapNames)

	// Sync and cleanup
	requeue, err := r.sync(ctx, log, cms)
	if cleanupErr := r.cleanup(ctx, log, cms); cleanupErr != nil && err == nil {
		err = cleanupErr
	}
	return reconcile.Result{Requeue: requeue}, err
}

func (r *ConfigMapSecret) cleanup(ctx context.Context, log logr.Logger, cms *v1alpha1.ConfigMapSecret) error {
	secretName := cms.Spec.Template.Metadata.Name
	if secretName == "" {
		secretName = cms.Name
	}

	r.mu.Lock()
	owned := keys(r.owned.srcs(cms.Namespace, string(cms.UID)))
	r.mu.Unlock()

	for _, name := range owned {
		if name == secretName {
			continue
		}

		key := types.NamespacedName{Namespace: cms.Namespace, Name: name}
		secretLog := log.WithValues("secret", key)
		secretLog.Info("Cleaning up secret")

		secret := &corev1.Secret{}
		if err := r.client.Get(ctx, key, secret); err != nil {
			if apierrors.IsNotFound(err) {
				secretLog.Info("Cleaning up secret unnecessary, already removed")
				continue
			}
			secretLog.Error(err, "Cleaning up secret, get failed")
			return err
		}
		if err := r.client.Delete(ctx, secret); err != nil {
			secretLog.Error(err, "Cleaning up secret, delete failed")
			return err
		}
	}
	return nil
}

func (r *ConfigMapSecret) sync(ctx context.Context, log logr.Logger, cms *v1alpha1.ConfigMapSecret) (requeue bool, err error) {
	secret, reason, err := r.renderSecret(ctx, cms)
	if err != nil {
		msg := err.Error()
		defer func() {
			if statusErr := r.syncRenderFailureStatus(ctx, log, cms, reason, msg); statusErr != nil {
				if err == nil {
					err = statusErr
				}
				requeue = true
			}
		}()
		if isConfigError(err) {
			missingValues.WithLabelValues(cms.Namespace).Inc()
			log.Info("Unable to render ConfigMapSecret", "warning", err)
			return true, nil
		}
		log.Error(err, "Unable to render ConfigMapSecret")
		return false, err
	}

	key := types.NamespacedName{Namespace: secret.Namespace, Name: secret.Name}
	secretLog := log.WithValues("secret", key)

	// Check if the Secret already exists
	found := &corev1.Secret{}
	if err := r.client.Get(ctx, key, found); err != nil {
		if apierrors.IsNotFound(err) {
			secretLog.Info("Creating Secret")
			if err := r.client.Create(ctx, secret); err != nil {
				secretLog.Error(err, "Unable to create Secret")
				return false, err
			}
			return false, r.syncSuccessStatus(ctx, log, cms)
		}
		secretLog.Error(err, "Unable to get Secret")
		return false, err
	}

	// Confirm or take ownership.
	ownerChanged, err := r.setOwner(secretLog, cms, found)
	if err != nil {
		return false, err
	}

	// Update the object and write the result back if there are any changes
	if ownerChanged || shouldUpdate(found, secret) {
		found.Labels = secret.Labels
		found.Annotations = secret.Annotations
		found.Data = secret.Data
		found.Type = secret.Type
		secretLog.Info("Updating Secret")
		if err := r.client.Update(ctx, found); err != nil {
			secretLog.Error(err, "Unable to update Secret")
			return false, err
		}
	}
	return false, r.syncSuccessStatus(ctx, log, cms)
}

func (r *ConfigMapSecret) setOwner(log logr.Logger, cms *v1alpha1.ConfigMapSecret, secret *corev1.Secret) (bool, error) {
	gvk, err := apiutil.GVKForObject(cms, r.scheme)
	if err != nil {
		return false, err
	}
	owner := metav1.NewControllerRef(cms, gvk)
	for i, ref := range secret.OwnerReferences {
		if ref.Controller == nil || !*ref.Controller {
			continue
		}
		if ref.UID != cms.UID {
			log.Error(err, "Secret has a different owner", "owner", ref)
			return false, &controllerutil.AlreadyOwnedError{Object: cms, Owner: ref}
		}
		if !reflect.DeepEqual(&ref, owner) { // e.g. apiVersion changed
			log.Info("Updating ownership of Secret")
			secret.OwnerReferences[i] = *owner
			return true, nil
		}
		return false, nil
	}
	log.Info("Taking ownership of Secret", "owner", *owner)
	cms.OwnerReferences = append(secret.OwnerReferences, *owner)
	return true, nil
}

func shouldUpdate(a, b *corev1.Secret) bool {
	return a.Type != b.Type ||
		!reflect.DeepEqual(a.Annotations, b.Annotations) ||
		!reflect.DeepEqual(a.Labels, b.Labels) ||
		!reflect.DeepEqual(a.Data, b.Data)
}

func (r *ConfigMapSecret) renderSecret(ctx context.Context, cms *v1alpha1.ConfigMapSecret) (*corev1.Secret, string, error) {
	vars, err := r.makeVariables(ctx, cms)
	if err != nil {
		return nil, CreateVariablesErrorReason, err
	}
	varMapFn := expansion.MappingFuncFor(vars)

	data := make(map[string][]byte)
	for k, v := range cms.Spec.Template.Data {
		data[k] = []byte(expansion.Expand(v, varMapFn))
	}
	for k, v := range cms.Spec.Template.BinaryData {
		data[k] = []byte(expansion.Expand(string(v), varMapFn))
	}

	meta := cms.Spec.Template.Metadata
	name := meta.Name
	if name == "" {
		name = cms.Name
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   cms.Namespace,
			Labels:      meta.Labels,
			Annotations: meta.Annotations,
		},
		Data: data,
		Type: corev1.SecretTypeOpaque,
	}
	if err := controllerutil.SetControllerReference(cms, secret, r.scheme); err != nil {
		return nil, internalError, err
	}
	return secret, "", nil
}

// Same logic as container env vars: Kubelet.makeEnvironmentVariables
// https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/kubelet_pods.go
func (r *ConfigMapSecret) makeVariables(ctx context.Context, cms *v1alpha1.ConfigMapSecret) (vars map[string]string, err error) {
	vars = make(map[string]string)
	mappingFn := expansion.MappingFuncFor(vars)
	configMaps := make(map[string]*corev1.ConfigMap)
	secrets := make(map[string]*corev1.Secret)

	for _, v := range cms.Spec.VarsFrom {
		var (
			kind, name  string
			invalidKeys []string
			srcVars     map[string]string
		)
		switch {
		case v.SecretRef != nil:
			kind = "Secret"
			name = v.SecretRef.Name
			srcVars, invalidKeys, err = r.secretValues(ctx, secrets, cms.Namespace, v.Prefix, *v.SecretRef)
		case v.ConfigMapRef != nil:
			kind = "ConfigMap"
			name = v.ConfigMapRef.Name
			srcVars, invalidKeys, err = r.configMapValues(ctx, configMaps, cms.Namespace, v.Prefix, *v.ConfigMapRef)
		}
		if err != nil {
			return nil, err
		}
		for k, v := range srcVars {
			vars[k] = v
		}
		if len(invalidKeys) > 0 {
			sort.Strings(invalidKeys)
			r.recorder.Eventf(
				cms,
				corev1.EventTypeWarning,
				"InvalidTemplateVariableNames",
				"Keys [%s] from the VarsFrom %s %s/%s were skipped since they are considered invalid template variable names.",
				strings.Join(invalidKeys, ", "),
				kind,
				cms.Namespace,
				name,
			)
		}
	}

	for _, v := range cms.Spec.Vars {
		val := v.Value
		found := true

		switch {
		case val != "":
			val = expansion.Expand(val, mappingFn)
		case v.SecretValue != nil:
			val, found, err = r.secretValue(ctx, secrets, cms.Namespace, *v.SecretValue)
		case v.ConfigMapValue != nil:
			val, found, err = r.configMapValue(ctx, configMaps, cms.Namespace, *v.ConfigMapValue)
		}

		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}

		vars[v.Name] = val
	}

	return vars, nil
}

func (r *ConfigMapSecret) secret(ctx context.Context, cache map[string]*corev1.Secret, namespace string, ref v1alpha1.SecretVarsSource) (secret *corev1.Secret, err error) {
	name := ref.Name
	secret, found := cache[name]
	if found {
		return secret, nil
	}
	secret = &corev1.Secret{}
	err = r.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, secret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if ref.Optional != nil && *ref.Optional {
				return nil, nil
			}
			return nil, &configError{err}
		}
		return nil, err
	}
	cache[name] = secret
	return secret, nil
}

func (r *ConfigMapSecret) secretValues(ctx context.Context, cache map[string]*corev1.Secret, namespace, prefix string, ref v1alpha1.SecretVarsSource) (values map[string]string, invalidKeys []string, err error) {
	secret, err := r.secret(ctx, cache, namespace, ref)
	if secret == nil || err != nil {
		return nil, nil, err
	}
	values = make(map[string]string, len(secret.Data))
	for k, v := range secret.Data {
		switch k, valid := validPrefixedKey(prefix, k); valid {
		case true:
			values[k] = string(v)
		case false:
			invalidKeys = append(invalidKeys, k)
		}
	}
	return values, invalidKeys, nil
}

func (r *ConfigMapSecret) secretValue(ctx context.Context, cache map[string]*corev1.Secret, namespace string, ref corev1.SecretKeySelector) (value string, found bool, err error) {
	key := ref.Key
	secret, err := r.secret(ctx, cache, namespace, v1alpha1.SecretVarsSource{
		LocalObjectReference: ref.LocalObjectReference,
		Optional:             ref.Optional,
	})
	if secret == nil || err != nil {
		return "", false, err
	}
	if buf, found := secret.Data[key]; found {
		return string(buf), true, nil
	}
	if ref.Optional != nil && *ref.Optional {
		return "", false, nil
	}
	return "", false, newConfigError("Couldn't find key %s in Secret %s/%s", key, namespace, ref.Name)
}

func (r *ConfigMapSecret) configMap(ctx context.Context, cache map[string]*corev1.ConfigMap, namespace string, ref v1alpha1.ConfigMapVarsSource) (configMap *corev1.ConfigMap, err error) {
	name := ref.Name
	configMap, found := cache[name]
	if found {
		return configMap, nil
	}
	configMap = &corev1.ConfigMap{}
	err = r.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, configMap)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if ref.Optional != nil && *ref.Optional {
				return nil, nil
			}
			return nil, &configError{err}
		}
		return nil, err
	}
	cache[name] = configMap
	return configMap, nil
}

func (r *ConfigMapSecret) configMapValues(ctx context.Context, cache map[string]*corev1.ConfigMap, namespace, prefix string, ref v1alpha1.ConfigMapVarsSource) (values map[string]string, invalidKeys []string, err error) {
	configMap, err := r.configMap(ctx, cache, namespace, ref)
	if configMap == nil || err != nil {
		return nil, nil, err
	}
	values = make(map[string]string, len(configMap.Data)+len(configMap.BinaryData))
	for k, v := range configMap.Data {
		switch k, valid := validPrefixedKey(prefix, k); valid {
		case true:
			values[k] = v
		case false:
			invalidKeys = append(invalidKeys, k)
		}
	}
	for k, v := range configMap.BinaryData {
		switch k, valid := validPrefixedKey(prefix, k); valid {
		case true:
			values[k] = string(v)
		case false:
			invalidKeys = append(invalidKeys, k)
		}
	}
	return values, invalidKeys, nil
}

func (r *ConfigMapSecret) configMapValue(ctx context.Context, cache map[string]*corev1.ConfigMap, namespace string, ref corev1.ConfigMapKeySelector) (value string, found bool, err error) {
	key := ref.Key
	configMap, err := r.configMap(ctx, cache, namespace, v1alpha1.ConfigMapVarsSource{
		LocalObjectReference: ref.LocalObjectReference,
		Optional:             ref.Optional,
	})
	if configMap == nil || err != nil {
		return "", false, err
	}
	if str, found := configMap.Data[key]; found {
		return str, true, nil
	}
	if buf, found := configMap.BinaryData[key]; found {
		return string(buf), true, nil
	}
	if ref.Optional != nil && *ref.Optional {
		return "", false, nil
	}
	return "", false, newConfigError("Couldn't find key %s in ConfigMap %s/%s", key, namespace, ref.Name)
}

func (r *ConfigMapSecret) syncSuccessStatus(ctx context.Context, log logr.Logger, cms *v1alpha1.ConfigMapSecret) error {
	return r.syncStatus(ctx, log, cms, corev1.ConditionFalse, "", "")
}

func (r *ConfigMapSecret) syncRenderFailureStatus(ctx context.Context, log logr.Logger, cms *v1alpha1.ConfigMapSecret, reason, message string) error {
	return r.syncStatus(ctx, log, cms, corev1.ConditionTrue, reason, message)
}

func (r *ConfigMapSecret) syncStatus(ctx context.Context, log logr.Logger, cms *v1alpha1.ConfigMapSecret, condStatus corev1.ConditionStatus, reason, message string) error {
	status := v1alpha1.ConfigMapSecretStatus{
		ObservedGeneration: cms.Generation,
		Conditions:         cms.Status.Conditions,
	}
	cond := NewConfigMapSecretCondition(v1alpha1.ConfigMapSecretRenderFailure, condStatus, reason, message)
	SetConfigMapSecretCondition(&status, *cond) // original backing array not modified
	if reflect.DeepEqual(cms.Status, status) {
		return nil
	}
	cms.Status = status
	log.Info("Updating status")
	if err := r.client.Status().Update(ctx, cms); err != nil {
		log.Error(err, "Unable to update status")
		return err
	}
	return nil
}

func validPrefixedKey(prefix, key string) (string, bool) {
	if prefix != "" {
		key = prefix + key
	}
	if errMsgs := validation.IsEnvVarName(key); len(errMsgs) > 0 {
		return key, false
	}
	return key, true
}

func getOwner(secret *corev1.Secret) *metav1.OwnerReference {
	owner := metav1.GetControllerOf(secret)
	if owner == nil || owner.Kind != "ConfigMapSecret" {
		return nil
	}
	if gv, _ := schema.ParseGroupVersion(owner.APIVersion); gv.Group != v1alpha1.GroupVersion.Group {
		return nil
	}
	return owner
}

func keys(set map[string]bool) []string {
	n := len(set)
	if n == 0 {
		return nil
	}
	s := make([]string, 0, n)
	for k := range set {
		s = append(s, k)
	}
	return s
}

func toReqs(namespace string, names map[string]bool) []reconcile.Request {
	var reqs []reconcile.Request
	for name := range names {
		reqs = append(reqs, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: namespace,
				Name:      name,
			},
		})
	}
	return reqs
}

func varRefs(varsFrom []v1alpha1.VarsFromSource, vars []v1alpha1.Var) (secrets, configMaps map[string]bool) {
	addSecret := func(name string) {
		if secrets == nil {
			secrets = make(map[string]bool)
		}
		secrets[name] = true
	}
	addConfigMap := func(name string) {
		if configMaps == nil {
			configMaps = make(map[string]bool)
		}
		configMaps[name] = true
	}
	for _, v := range varsFrom {
		if v.SecretRef != nil {
			addSecret(v.SecretRef.Name)
		}
		if v.ConfigMapRef != nil {
			addConfigMap(v.ConfigMapRef.Name)
		}
	}
	for _, v := range vars {
		if v.SecretValue != nil {
			addSecret(v.SecretValue.Name)
		}
		if v.ConfigMapValue != nil {
			addConfigMap(v.ConfigMapValue.Name)
		}
	}
	return secrets, configMaps
}

type configError struct {
	err error
}

func newConfigError(format string, v ...interface{}) *configError {
	if len(v) == 0 {
		return &configError{errors.New(format)}
	}
	return &configError{fmt.Errorf(format, v...)}
}

func (e *configError) Error() string {
	return e.err.Error()
}

func (*configError) IsConfigError() bool { return true }

func isConfigError(err error) bool {
	v, ok := err.(interface {
		IsConfigError() bool
	})
	return ok && v.IsConfigError()
}
