# ConfigMapSecrets

[![Release](https://img.shields.io/github/release/machinezone/configmapsecrets)](https://github.com/machinezone/configmapsecrets/releases) [![API Reference](https://img.shields.io/badge/API-reference-blue)](/docs/api.md) [![Go Report Card](https://goreportcard.com/badge/github.com/machinezone/configmapsecrets)](https://goreportcard.com/report/github.com/machinezone/configmapsecrets) [![License](https://img.shields.io/github/license/machinezone/configmapsecrets.svg)](/LICENSE)

### Problem
I have [a config](https://prometheus.io/docs/alerting/configuration/) that contains a mixture
of secret and non-secret data. For [some reason](https://github.com/prometheus/alertmanager/issues/504)
I can't use environment variables to reference the secret data. I want to check my config
into source control, keep my secret data secure, and keep my non-secret data easily
readable and editable.

### Solution
Use a [ConfigMapSecret](docs/api.md#configmapsecret) which is safe to store in source control. It's like
a ConfigMap that includes your non-secret data, but it can reference Secret variables, similar to how
[container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#container-v1-core)
args can reference env variables. The controller will expand and render it into a Secret in the same
namespace, keeping it updated to reflect changes to the ConfigMapSecret or its referenced variables.

Use [SealedSecrets](https://github.com/bitnami-labs/sealed-secrets) to keep your referenced
Secret data secure.

## Installation

```
kubectl apply -f manifest/*.yaml
```

## Example

### Input
```yaml
apiVersion: secrets.k8s.mz.com/v1alpha1
kind: ConfigMapSecret
metadata:
  name: alertmanager-config
  namespace: monitoring
  labels:
    app: alertmanager
spec:
  template:
    metadata:
      # optional: name defaults to same as ConfigMapSecret
      name: alertmanager-config
      labels:
        app: alertmanager
    data:
      alertmanager.yaml: |
          global:
            resolve_timeout: 5m
            opsgenie_api_key: $(OPSGENIE_API_KEY)
            slack_api_url: $(SLACK_API_URL)
          route:
            receiver: default
            group_by: ["alertname", "job", "team"]
            group_wait: 30s
            group_interval: 5m
            repeat_interval: 12h
            routes:
              - receiver: foobar-sre
                match:
                  team: foobar-sre
              - receiver: widget-sre
                match:
                  team: widget-sre
          receivers:
            - name: default
              slack_configs:
                - channel: unrouted-alerts
            - name: foobar-sre
              opsgenie_configs:
                - responders:
                    - name: foobar-sre
                      type: team
              slack_configs:
                - channel: foobar-sre-alerts
            - name: widget-sre
              opsgenie_configs:
                - responders:
                    - name: widget-sre
                      type: team
              slack_configs:
                - channel: widget-sre
  vars:
    - name: OPSGENIE_API_KEY
      secretValue:
        name: alertmanager-keys
        key: opsgenieKey
    - name: SLACK_API_URL
      secretValue:
        name: alertmanager-keys
        key: slackURL
---
apiVersion: v1
kind: Secret
metadata:
  name: alertmanager-keys
  namespace: monitoring
  labels:
    app: alertmanager
stringData:
  opsgenieKey: 9eccf784-bbad-11e9-9cb5-2a2ae2dbcce4
  slackURL: https://hooks.slack.com/services/EFNPN1/EVU44X/J51NVTYSKwuPtCz3
type: Opaque
```

### Output
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: alertmanager-config
  namespace: monitoring
  labels:
    app: alertmanager
stringData:
  alertmanager.yaml: |
    global:
      resolve_timeout: 5m
      opsgenie_api_key: 9eccf784-bbad-11e9-9cb5-2a2ae2dbcce4
      slack_api_url: https://hooks.slack.com/services/EFNPN1/EVU44X/J51NVTYSKwuPtCz3
    route:
      receiver: default
      group_by: ["alertname", "job", "team"]
      group_wait: 30s
      group_interval: 5m
      repeat_interval: 12h
      routes:
        - receiver: foobar-sre
          match:
           team: foobar-sre
        - receiver: widget-sre
          match:
            team: widget-sre
    receivers:
      - name: default
        slack_configs:
          - channel: unrouted-alerts
      - name: foobar-sre
        opsgenie_configs:
          - responders:
              - name: foobar-sre
                type: team
        slack_configs:
          - channel: foobar-sre
      - name: widget-sre
        opsgenie_configs:
          - responders:
              - name: widget-sre
                type: team
        slack_configs:
          - channel: widget-sre
type: Opaque
```
