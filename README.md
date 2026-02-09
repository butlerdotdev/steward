# Steward

<p align="left">
  <img src="https://img.shields.io/github/license/butlerlabs/steward"/>
  <img src="https://img.shields.io/github/go-mod/go-version/butlerlabs/steward"/>
  <a href="https://github.com/butlerlabs/steward/releases"><img src="https://img.shields.io/github/v/release/butlerlabs/steward"/></a>
  <img src="https://goreportcard.com/badge/github.com/butlerlabs/steward">
  <a href="https://kubernetes.slack.com/archives/C03GLTTMWNN"><img alt="#steward on Kubernetes Slack" src="https://img.shields.io/badge/slack-@kubernetes/steward-blue.svg?logo=slack"/></a>
</p>


### 🤔 What is Steward?

**Steward** is the **Kubernetes Control Plane Manager** leveraging on the concept of [**Hosted Control Plane**]().

Steward's approach is based on running the Kubernetes Control Plane components in Pods instead of dedicated machines.
This allows operating Kubernetes clusters at scale, with a fraction of the operational burden.
Thanks to this approach, running multiple Control Planes can be cheaper and easier to deploy and operate.

_Steward is like a fleet of Site Reliability Engineers with expertise codified into its logic, working 24/7 to keep up and running your Control Planes._

<img src="docs/content/images/architecture.png"  width="600" style="display: block; margin: 0 auto">

### 📖 How it works

Steward is extending the Kubernetes API capabilities thanks to [Custom Resource Definitions](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions).

By installing Steward, two pairs of new APIs will be available:

- `TenantControlPlane`, the instance definition of your desired Kubernetes Control Plane
- `Datastore`, the backing store used by one (or more) `TenantControlPlane`

The `TenantControlPlane` (short-named as `tcp`) objects are Namespace-scoped and allows configuring every aspect of your desired Control Plane.
Besides the Kubernetes configuration values, you can specify the Pod options such as limit, request, tolerations, node selector, etc.,
as well as how these should be exposed (e.g.: using a `ClusterIP`, a `LoadBalancer`, or a `NodePort`).

The `TenantControlPlane` is the stateless definition of the Control Plane allowing to set up the required components for a full-fledged Kubernetest cluster.
The state is managed by the `Datastore` API, a cluster-scoped resource which can hold the data of one or more Kubernetes clusters.

> For further information about the API specifications and all the available options,
> refer to the official [API reference](https://steward.butlerlabs.io/reference/api/#tenantcontrolplane).

### TenantControlPlane Example

```yaml
apiVersion: steward.butlerlabs.dev/v1alpha1
kind: TenantControlPlane
metadata:
  name: tenant-1
  namespace: tenants
spec:
  dataStore: default
  kubernetes:
    version: "1.29.0"
    kubelet:
      preferredAddressTypes:
        - InternalIP
        - ExternalIP
        - Hostname
    admissionControllers: CertificateApproval,CertificateSigning,CertificateSubjectRestriction,DefaultIngressClass,DefaultStorageClass,DefaultTolerationSeconds,LimitRanger,MutatingAdmissionWebhook,NamespaceLifecycle,PersistentVolumeClaimResize,Priority,ResourceQuota,RuntimeClass,ServiceAccount,StorageObjectInUseProtection,TaintNodesByCondition,ValidatingAdmissionWebhook
  networkProfile:
    address: ""  # Auto-assigned by LoadBalancer
    port: 6443
    serviceCidr: "10.96.0.0/16"
    podCidr: "10.244.0.0/16"
    certSANs:
      - "tenant-1.example.com"
  controlPlane:
    deployment:
      replicas: 2
      resources:
        limits:
          cpu: "500m"
          memory: "512Mi"
        requests:
          cpu: "250m"
          memory: "256Mi"
    service:
      serviceType: LoadBalancer
  addons:
    coreDNS: {}
    kubeProxy: {}
    konnectivity:
      server:
        port: 8132
```

### DataStore Examples

Steward supports multiple backend storage drivers for Tenant Control Plane data.

#### etcd DataStore

```yaml
apiVersion: steward.butlerlabs.dev/v1alpha1
kind: DataStore
metadata:
  name: default
spec:
  driver: etcd
  endpoints:
    - etcd-0.etcd.steward-system.svc.cluster.local:2379
    - etcd-1.etcd.steward-system.svc.cluster.local:2379
    - etcd-2.etcd.steward-system.svc.cluster.local:2379
  tlsConfig:
    certificateAuthority:
      certificate:
        secretReference:
          name: etcd-certs
          namespace: steward-system
          keyPath: ca.crt
      privateKey:
        secretReference:
          name: etcd-certs
          namespace: steward-system
          keyPath: ca.key
    clientCertificate:
      certificate:
        secretReference:
          name: etcd-certs
          namespace: steward-system
          keyPath: tls.crt
      privateKey:
        secretReference:
          name: etcd-certs
          namespace: steward-system
          keyPath: tls.key
```

#### PostgreSQL DataStore

```yaml
apiVersion: steward.butlerlabs.dev/v1alpha1
kind: DataStore
metadata:
  name: postgres-store
spec:
  driver: PostgreSQL
  endpoints:
    - postgres.steward-system.svc.cluster.local:5432
  basicAuth:
    username:
      secretReference:
        name: postgres-credentials
        namespace: steward-system
        keyPath: username
    password:
      secretReference:
        name: postgres-credentials
        namespace: steward-system
        keyPath: password
```

#### MySQL DataStore

```yaml
apiVersion: steward.butlerlabs.dev/v1alpha1
kind: DataStore
metadata:
  name: mysql-store
spec:
  driver: MySQL
  endpoints:
    - mysql.steward-system.svc.cluster.local:3306
  basicAuth:
    username:
      secretReference:
        name: mysql-credentials
        namespace: steward-system
        keyPath: username
    password:
      secretReference:
        name: mysql-credentials
        namespace: steward-system
        keyPath: password
```

#### NATS DataStore

```yaml
apiVersion: steward.butlerlabs.dev/v1alpha1
kind: DataStore
metadata:
  name: nats-store
spec:
  driver: NATS
  endpoints:
    - nats.steward-system.svc.cluster.local:4222
  basicAuth:
    username:
      secretReference:
        name: nats-credentials
        namespace: steward-system
        keyPath: username
    password:
      secretReference:
        name: nats-credentials
        namespace: steward-system
        keyPath: password
```

### ⭐️ Main features

- **Fast provisioning time**: depending on the infrastructure, Tenant Control Planes are up and ready to serve traffic in **16 seconds**.
- **Streamlined update**: the rollout to a new Kubernetes version for a given Tenant Control Plane takes just **10 seconds**, with a Blue/Green deployment to avoid serving mixed Kubernetes versions.
- **Resource optimization**: thanks to the Datastore decoupling, there's no need of odd number instances (e.g.: RAFT consensus) by allowing to save up to 60% of HW resources.
- **Scale from zero to the moon**: scale down the instance when there's no usage, or automatically scale to support the traffic spikes reusing the Kubernetes patterns.
- **Declarative approach, constant reconciliation**: thanks to the Operator pattern, drift detection happens in real-time, maintaining the desired state.
- **Automated certificates management**: Steward leverages on `kubeadm` and the certificates are automatically created and rotated for you.
- **Managing core addons**: Steward allows configuring automatically `kube-proxy`, `CoreDNS`, and `konnectivity`, with automatic remediation in case of user errors (e.g.: deleting the `CoreDNS` deployment).
- **Auto Healing**: the `TenantControlPlane` objects in the management cluster are tracked by Steward, in case of deletion of those, everything is created in an idempotent way.
- **Datastore multi-tenancy**: optionally, Steward allows running multiple Control Planes on the same _Datastore_ instance leveraging on the multi-tenancy of each driver, decreasing operations and optimizing costs.
- **Overcoming `etcd` limitations**: optionally, Steward allows using a different _Datastore_ thanks to [`kine`](https://github.com/k3s-io/kine) by supporting `MySQL`, `PostgreSQL`, or `NATS` as an alternative.
- **Simplifying mixed-networks setup**: thanks to [`Konnectivity`](https://kubernetes.io/docs/tasks/extend-kubernetes/setup-konnectivity/),
  the Tenant Control Plane is connected to the worker nodes hosted in a different network, overcoming the no-NAT availability when dealing with nodes with a non routable IP address
  (e.g.: worker nodes in a different infrastructure).

### 🚀 Use cases

- **Kubernetes Control Plane as a Service:** centrally manage multiple Kubernetes clusters from a single management point (_Multi-Cluster management_). 
- **High-density Control Plane:** place multiple control planes on the same infrastructure, instead of having dedicated machines for each control plane.
- **Strong Multi-tenancy:** leave users to access the control plane with admin permissions while keeping them isolated at the infrastructure level.
- **Kubernetes Inception:** use Kubernetes to manage Kubernetes with automation, high-availability, fault tolerance, and autoscaling out of the box. 
- **Bring Your Own Device:** keep the control plane isolated from data plane. Worker nodes can join and run consistently from everywhere: cloud, edge, and data-center.
- **Full CNCF compliant:** all clusters are built with upstream Kubernetes binaries, resulting in full CNCF compliant Kubernetes clusters.

> 🤔 You'd like to do the same but don't know how?
> 💡 [Butler Labs](https://butlerlabs.dev/) can help you with your needs!

### 🧑‍💻‍ Production grade

Steward is empowering several businesses, and it counts public adopters.
Check out the [adopters](./ADOPTERS.md) file to learn more.

> 🤗 If you're using Steward, share your love by opening a PR!

### 🍦 Vanilla Kubernetes clusters

Steward is **not** yet-another-Kubernetes distribution: you have full freedom on the technology stack to provide to end users.
Steward is a perfect fit for Platform Engineering, hiding the complexity of the Control Plane management to developers and DevOps engineers.

The provided Kubernetes Control Planes are [CNCF compliant clusters](https://steward.butlerlabs.io/reference/conformance/).

<img src="https://raw.githubusercontent.com/cncf/artwork/master/projects/kubernetes/certified-kubernetes/versionless/color/certified-kubernetes-color.png" style="display: block; width: 75px; margin: 0 auto">

### 🐢 Cluster API support

Steward is **not** a [Cluster API](https://cluster-api.sigs.k8s.io/) replacement, rather, it plays very well with it.

Since Steward is just focusing on the Control Plane a [Steward's Cluster API Control Plane provider](https://github.com/butlerlabs/cluster-api-control-plane-provider-steward) has been developed.

### 🛣️ Roadmap

- [x] Dynamic address on Load Balancer
- [x] Zero Downtime Tenant Control Plane upgrade
- [x] [Join worker nodes from anywhere thanks to Konnectivity](https://steward.butlerlabs.io/concepts/#konnectivity)
- [x] [Alternative datastore MySQL, PostgreSQL, NATS](https://steward.butlerlabs.io/guides/alternative-datastore/)
- [x] [Pool of multiple datastores](https://steward.butlerlabs.io/concepts/#datastores)
- [x] [Seamless migration between datastores](https://steward.butlerlabs.io/guides/datastore-migration/)
- [ ] Automatic assignment to a datastore
- [ ] Autoscaling of Tenant Control Plane
- [x] [Provisioning through Cluster APIs](https://github.com/butlerlabs/cluster-api-control-plane-provider-steward)
- [ ] Terraform provider
- [ ] Custom Prometheus metrics

### 🏷️ Versioning

Versioning adheres to the [Semantic Versioning](http://semver.org/) principles.
A full list of the available releases is available in the GitHub repository's [**Release** section](https://github.com/butlerlabs/steward/releases).

### 📄 Documentation

Further documentation can be found on the official [Steward documentation website](https://steward.butlerlabs.io/).

### 🤝 Contributions

Contributions are highly appreciated and very welcomed!

In case of bugs, please, check if the issue has been already opened by checking the [GitHub Issues](https://github.com/butlerlabs/steward/issues) section.
In case it isn't, you can open a new one: a detailed report will help us to replicate it, assess it, and work on a fix.

You can express your intention in working on the fix on your own.
The commit messages are checked according to the described [semantics](https://github.com/projectcapsule/capsule/blob/main/CONTRIBUTING.md#semantics).
Commits are used to generate the changelog, and their author will be referenced in it.

In case of **✨ Feature Requests** please use the [Discussion's Feature Request section](https://github.com/butlerlabs/steward/discussions/categories/feature-requests).

### 📜 Project History

Steward is a community-governed fork of [Kamaji](https://github.com/clastix/kamaji), created in July 2024 after CLASTIX Labs gated stable releases behind commercial support.

**Key differences from Kamaji:**
- Stable releases published to public registries
- Community-driven roadmap and governance
- Integration with the [Butler](https://github.com/butlerdotdev/butler) Kubernetes-as-a-Service platform
- No commercial gating of features

Steward is maintained by [Butler Labs](https://butlerlabs.dev) and the open source community, with a goal of achieving CNCF Sandbox status by ~2027.

We thank the CLASTIX team for their foundational work on Kamaji.

### 📝 License

Steward is licensed under Apache 2.0.
The code is provided as-is with no warranties.
