# Steward Helm Chart

Steward is the Hosted Control Plane Manager for Kubernetes.

## Installation

Add the Butler Labs Helm repository:
```bash
helm repo add butlerlabs https://butlerdotdev.github.io/charts
helm repo update
```

Install the Chart:
```bash
helm upgrade --install --namespace steward-system --create-namespace steward butlerlabs/steward
```

## Configuration

Steward supports multiple datastores. By default, an etcd cluster will be deployed.

To use an external datastore:
```bash
helm upgrade steward --install --namespace steward-system --create-namespace butlerlabs/steward --set etcd.deploy=false
```

## Upgrade
```bash
helm upgrade steward -n steward-system butlerlabs/steward
```

## Uninstall
```bash
helm uninstall steward -n steward-system
```

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | Kubernetes affinity rules to apply to Steward controller pods |
| defaultDatastoreName | string | `"default"` | If specified, all the Steward instances with an unassigned DataStore will inherit this default value. |
| extraArgs | list | `[]` | A list of extra arguments to add to the steward controller default ones |
| fullnameOverride | string | `""` |  |
| healthProbeBindAddress | string | `":8081"` | The address the probe endpoint binds to. (default ":8081") |
| image.pullPolicy | string | `"IfNotPresent"` |  |
| image.repository | string | `"ghcr.io/butlerdotdev/steward"` | The container image of the Steward controller. |
| image.tag | string | `""` | Overrides the image tag whose default is the chart appVersion. |
| imagePullSecrets | list | `[]` |  |
| kubeconfigGenerator.affinity | object | `{}` | Kubernetes affinity rules to apply to Kubeconfig Generator controller pods |
| kubeconfigGenerator.enableLeaderElect | bool | `true` | Enables the leader election. |
| kubeconfigGenerator.enabled | bool | `false` | Toggle to deploy the Kubeconfig Generator Deployment. |
| kubeconfigGenerator.extraArgs | list | `[]` | A list of extra arguments to add to the Kubeconfig Generator controller default ones. |
| kubeconfigGenerator.fullnameOverride | string | `""` |  |
| kubeconfigGenerator.healthProbeBindAddress | string | `":8081"` | The address the probe endpoint binds to. |
| kubeconfigGenerator.loggingDevel.enable | bool | `false` | Development Mode defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn). Production Mode defaults(encoder=jsonEncoder,logLevel=Info,stackTraceLevel=Error) |
| kubeconfigGenerator.nodeSelector | object | `{}` | Kubernetes node selector rules to schedule Kubeconfig Generator controller |
| kubeconfigGenerator.podAnnotations | object | `{}` | The annotations to apply to the Kubeconfig Generator controller pods. |
| kubeconfigGenerator.podSecurityContext | object | `{"runAsNonRoot":true}` | The securityContext to apply to the Kubeconfig Generator controller pods. |
| kubeconfigGenerator.replicaCount | int | `2` | The number of the pod replicas for the Kubeconfig Generator controller. |
| kubeconfigGenerator.resources.limits.cpu | string | `"200m"` |  |
| kubeconfigGenerator.resources.limits.memory | string | `"512Mi"` |  |
| kubeconfigGenerator.resources.requests.cpu | string | `"200m"` |  |
| kubeconfigGenerator.resources.requests.memory | string | `"512Mi"` |  |
| kubeconfigGenerator.securityContext | object | `{"allowPrivilegeEscalation":false}` | The securityContext to apply to the Kubeconfig Generator controller container only. |
| kubeconfigGenerator.serviceAccountOverride | string | `""` | The name of the service account to use. If not set, the root Steward one will be used. |
| kubeconfigGenerator.tolerations | list | `[]` | Kubernetes node taints that the Kubeconfig Generator controller pods would tolerate |
| livenessProbe | object | `{"httpGet":{"path":"/healthz","port":"healthcheck"},"initialDelaySeconds":15,"periodSeconds":20}` | The livenessProbe for the controller container |
| loggingDevel.enable | bool | `false` | Development Mode defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn). Production Mode defaults(encoder=jsonEncoder,logLevel=Info,stackTraceLevel=Error) (default false) |
| metricsBindAddress | string | `":8080"` | The address the metric endpoint binds to. (default ":8080") |
| nameOverride | string | `""` |  |
| nodeSelector | object | `{}` | Kubernetes node selector rules to schedule Steward controller |
| podAnnotations | object | `{}` | The annotations to apply to the Steward controller pods. |
| podSecurityContext | object | `{"runAsNonRoot":true}` | The securityContext to apply to the Steward controller pods. |
| readinessProbe | object | `{"httpGet":{"path":"/readyz","port":"healthcheck"},"initialDelaySeconds":5,"periodSeconds":10}` | The readinessProbe for the controller container |
| replicaCount | int | `1` | The number of the pod replicas for the Steward controller. |
| resources.limits.cpu | string | `"200m"` |  |
| resources.limits.memory | string | `"100Mi"` |  |
| resources.requests.cpu | string | `"100m"` |  |
| resources.requests.memory | string | `"20Mi"` |  |
| securityContext | object | `{"allowPrivilegeEscalation":false}` | The securityContext to apply to the Steward controller container only. It does not apply to the Steward RBAC proxy container. |
| serviceAccount.annotations | object | `{}` |  |
| serviceAccount.create | bool | `true` |  |
| serviceAccount.name | string | `"steward-controller-manager"` |  |
| serviceMonitor.enabled | bool | `false` | Toggle the ServiceMonitor true if you have Prometheus Operator installed and configured |
| steward-etcd | object | `{"clusterDomain":"cluster.local","datastore":{"enabled":true,"name":"default"},"deploy":true,"fullnameOverride":"steward-etcd"}` | Subchart: See https://github.com/butlerlabs/steward-etcd/blob/master/charts/steward-etcd/values.yaml |
| telemetry | object | `{"disabled":false}` | Disable the analytics traces collection |
| temporaryDirectoryPath | string | `"/tmp/steward"` | Directory which will be used to work with temporary files. (default "/tmp/steward") |
| tolerations | list | `[]` | Kubernetes node taints that the Steward controller pods would tolerate |
