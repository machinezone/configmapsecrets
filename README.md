# ConfigMapSecrets

### Problem
I have [a config](https://prometheus.io/docs/alerting/configuration/) that contains a mixture
of secret and non-secret data. For [some reason](https://github.com/prometheus/alertmanager/issues/504)
I can't use environment variables to reference the secret data. I want to check my config
into source control, keep my secret data secure, and keep my non-secret data easily
readable and editable.

### Solution
Use a [ConfigMapSecret](docs/api.md) which is safe to store in source control. It's like a
ConfigMap that includes your non-secret data, but it can reference Secret variables, similar to how
[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#container-v1-core)
args can reference env variables. The expanded ConfigMapSecret will be rendered into a Secret
in the same namespace by a controller in the cluster. 

Use [SealedSecrets](https://github.com/bitnami-labs/sealed-secrets) to keep your referenced
Secret data secure.

## Installation

```
kubectl apply -f manifest/*.yaml
```