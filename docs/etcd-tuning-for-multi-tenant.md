# Steward etcd Tuning for Multi-Tenant Deployments

## Problem Statement

When running 10+ TenantControlPlanes (TCPs) against a shared steward-etcd cluster, TCP schedulers and controller-managers enter CrashLoopBackOff due to leader election lease renewal timeouts. The kube-apiserver in each TCP pod remains running but becomes too slow to respond to lease renewals within the 5-second timeout.

This issue presents as a "whack-a-mole" pattern: bouncing one TCP pod fixes it temporarily, but knocks others into NotReady as resources are contended.

## Root Cause Analysis

### Issue 1: etcd CPU Starvation

The steward-etcd StatefulSet ships with zero resource requests and limits (empty `{}`). This means:

- etcd pods are scheduled as **BestEffort** QoS class
- TCP pods (also with no resource requests) compete for CPU on the same nodes
- Under contention, the kernel scheduler gives etcd no guaranteed CPU time
- Raft consensus latency degrades from normal ~20ms to 1-3 seconds

**Evidence from etcd logs:**
```
"agreement among raft nodes before linearized reading" (duration: 2.787s)
```
Expected: <50ms. Observed under CPU starvation: 1-3 seconds.

### Issue 2: etcd Snapshot Freeze

The default `--snapshot-count=10000` triggers a full raft snapshot every 10,000 raft log entries. With 10 TCPs generating constant lease renewals, watch events, and resource updates, the raft index advances rapidly (~10,000 entries every few minutes).

During snapshot writes, the etcd leader **blocks all raft heartbeats**, causing:

1. Leader cannot send heartbeats to followers
2. All linearizable reads stall (they require leader confirmation)
3. Every TCP apiserver becomes unresponsive simultaneously (shared etcd)
4. Scheduler and controller-manager lease renewals timeout
5. Mass CrashLoopBackOff across all TCPs

**Evidence from etcd leader logs:**
```
"triggering snapshot" local-member-applied-index:24422504 local-member-snapshot-count:10000
"saved snapshot" snapshot-index:24422504
"leader failed to send out heartbeat on time; took too long, leader is overloaded likely from slow disk"
```

Observed snapshot freeze duration: **3-25 seconds**.

### Issue 3: TCP Pod CPU Starvation

TCP pods (containing kube-apiserver, kube-scheduler, kube-controller-manager, konnectivity-server, and optionally steward-trustd) also ship with zero resource requests. When multiple TCP pods land on the same worker node:

- The kube-apiserver within each TCP pod gets CPU-starved
- Apiserver readyz probes timeout: `post-timeout activity - time-elapsed: 1.16s, GET "/readyz"`
- The co-located scheduler/controller-manager cannot reach 127.0.0.1:6443 (the apiserver in the same pod) within the 5-second lease renewal timeout
- The scheduler/controller-manager exits with "Leaderelection lost"

This occurs even when etcd is healthy, because the apiserver process itself lacks CPU time.

### Combined Feedback Loop

```
TCP pods restart → CPU spike on node
  → etcd gets CPU-starved → raft consensus slows to seconds
    → All apiservers become slow (shared etcd backend)
      → Scheduler/controller-manager can't renew leases
        → They crash → restart → more CPU spike
          → Periodic snapshots compound by freezing leader for seconds
```

## Diagnostic Indicators

### 1. TCP Status Whack-a-Mole

TCPs cycle between Ready and NotReady. Bouncing one pod fixes it but knocks others off.

### 2. Bimodal Raft Latency

```
Normal:  17ms, 21ms, 22ms, 17ms   ← healthy raft consensus
Spikes:  3.7s, 11.0s, 25.2s       ← snapshot freeze events
```

The normal latency confirms storage (even Longhorn) is adequate. The spikes correlate with snapshot events.

### 3. Container Restart Pattern

Always kube-scheduler and kube-controller-manager crashing (they use leader election leases), while kube-apiserver stays running. Check with:

```bash
kubectl get pods -l steward.butlerlabs.dev/component -A \
  -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name}{"\t"}{range .status.containerStatuses[*]}{.name}={.ready}/{.restartCount} {end}{"\n"}{end}'
```

### 4. Scheduler/Controller-Manager Logs

```
Failed to update lock optimistically: Put "https://127.0.0.1:6443/apis/coordination.k8s.io/v1/namespaces/kube-system/leases/kube-scheduler?timeout=5s": net/http: request canceled (Client.Timeout exceeded while awaiting headers)
failed to renew lease kube-system/kube-scheduler: timed out waiting for the condition
"Leaderelection lost"
```

## Recommended Fixes

### Fix 1: etcd Resource Requests and Limits

Add to the steward-etcd StatefulSet container spec:

```yaml
resources:
  requests:
    cpu: "500m"
    memory: "512Mi"
  limits:
    cpu: "2"
    memory: "1Gi"
```

**Rationale:**
- CPU requests guarantee etcd gets CPU shares under contention (moves from BestEffort to Burstable QoS)
- 500m request is sufficient for 10-20 TCPs based on normal operation (~20ms raft consensus)
- 2 CPU limit allows burst during compaction without unbounded CPU consumption
- Memory: 512Mi request covers the ~94MB in-use DB size with headroom; 1Gi limit prevents runaway growth

**Impact observed:** Raft consensus latency improved from 1-3 seconds to 100-400ms immediately after applying CPU requests.

### Fix 2: Increase Snapshot Count

Change the etcd `--snapshot-count` argument:

```
Before: --snapshot-count=10000  (default)
After:  --snapshot-count=50000
```

**Rationale:**
- With 10 TCPs, raft index advances ~10,000 entries every few minutes, triggering frequent snapshots
- Each snapshot freezes the leader for 3-25 seconds
- 50,000 reduces snapshot frequency by 5x
- Kubernetes upstream recommends 50,000-100,000 for larger clusters
- Trade-off: slightly more WAL to replay on crash recovery, but etcd raft replication across 3 members provides durability

**Impact observed:** After increasing to 50,000, zero slow requests logged and zero leader heartbeat failures over sustained observation periods.

### Fix 3: TCP Pod Default Resources

TCP pods (kube-apiserver, kube-scheduler, kube-controller-manager, konnectivity-server) need resource requests so the apiserver gets guaranteed CPU on shared worker nodes. Steward sets minimal defaults when no resources are specified via `TCP.spec.controlPlane.deployment.resources`:

```yaml
# Default per-container resources (requests only, no limits)
apiServer:     100m CPU, 128Mi memory
scheduler:      25m CPU,  32Mi memory
controllerManager: 25m CPU,  32Mi memory
kine:           25m CPU,  32Mi memory  # only for non-etcd datastores
```

Total per TCP: ~175m CPU, ~224Mi memory requests. No limits set by default.

**Rationale:**
- Without CPU requests, all TCP pods are BestEffort and compete for scraps
- The apiserver needs guaranteed CPU to respond to lease renewals from scheduler/controller-manager
- Minimal requests keep edge deployments viable: a 2-CPU node can fit ~11 TCPs, a 4-CPU node ~22 TCPs
- No limits allows burst during cert generation or heavy API calls without OOMKill
- For dense deployments (10+ TCPs on limited hardware), increase apiserver to 250m+ via the TCP spec

**Override:** Set `TCP.spec.controlPlane.deployment.resources.apiServer` (etc.) to override any default. Setting explicit resources always takes precedence.

## Scaling Reference

### Observed Metrics at 10 TCPs (butler-beta cluster)

| Metric | Value |
|---|---|
| etcd DB size | ~160MB total, ~94MB in-use |
| Raft index | ~24M revisions (after 25 days) |
| etcd quota | 8GiB (`--quota-backend-bytes=8589934592`) |
| Worker nodes | 3x (4 CPU, 8GB RAM each) |
| Control-plane nodes | 3x (dedicated, not running TCP pods) |
| TCP pods per worker | 3-7 (uneven due to zero requests) |

### Per-TCP etcd Load

Each TCP generates etcd traffic from:
- Leader election leases: kube-scheduler, kube-controller-manager
- Flux controllers: helm-controller, kustomize-controller, source-controller, image-automation-controller, notification-controller
- Cilium operator resource lock
- Longhorn CSI sidecars: external-attacher, external-provisioner, external-resizer, external-snapshotter
- democratic-csi sidecars (if installed)
- CRD watches: Longhorn, Cilium, Gateway API, Flux, Traefik — even with zero actual resources of those types

### Node Topology

- etcd pods should be spread across nodes (StatefulSet with pod anti-affinity handles this naturally)
- Avoid co-locating an etcd pod on the same node as 7+ TCP pods
- The management cluster's own etcd runs on dedicated control-plane nodes with local storage — Steward's etcd shares worker nodes, making resource guarantees critical

## Longhorn Storage Consideration

The steward-etcd PVCs use the default StorageClass (Longhorn with 3 replicas). This works:

- Normal read/write latency with Longhorn: ~17-22ms — adequate for etcd
- The bimodal latency pattern proves storage is fine during normal operation
- Only snapshot events cause freezes (CPU-bound, not I/O-bound)

However, etcd's own 3-member raft replication already provides data durability, making Longhorn's 3-replica replication redundant (9 total copies). Options to consider:

- **Reduce Longhorn replicas to 1** for etcd PVCs — cuts write amplification without sacrificing durability
- **Local PVs** — optimal for etcd but requires cluster-specific configuration
- **Keep Longhorn 3x** — redundant but not harmful if CPU resources are properly allocated

## Compaction Settings

The existing compaction settings are adequate:

```
--auto-compaction-mode=periodic
--auto-compaction-retention=5m
```

Observed compaction performance: completed in ~410ms. Not a bottleneck.

## Implementation Checklist

### Helm Chart Changes (steward-etcd)

- [x] Add `resources.requests` to steward-etcd StatefulSet container spec in chart values.yaml (200m/256Mi)
- [x] Resources configurable via values.yaml (already was, now has non-empty defaults)
- [x] Change `--snapshot-count` default from `10000` to `50000`
- [x] `--snapshot-count` configurable via values.yaml (already was)
- [ ] Update chart README (`make -C charts/steward docs`)
- [x] Bump chart version (0.14.0 → 0.15.0)

### TCP Pod Resource Defaults (Chart-Level)

- [ ] Set recommended resource requests in steward Helm chart values for TCP pod containers
- [ ] Document recommended values for different deployment sizes (edge vs. datacenter)

### Testing

- [ ] Deploy 10+ TCPs on a 3-worker-node cluster
- [ ] Verify all TCPs reach and maintain Ready status for 30+ minutes without any flapping
- [ ] Verify zero `"took too long"` warnings in etcd logs over a 10-minute window
- [ ] Verify zero `"leader failed to send out heartbeat"` messages
- [ ] Trigger a TCP pod restart and verify other TCPs remain Ready
- [ ] Verify etcd snapshot events complete without causing TCP status changes

### Monitoring Recommendations

- [ ] Alert on etcd request latency p99 > 200ms
- [ ] Alert on `"leader failed to send out heartbeat on time"` log entries
- [ ] Track TCP status flap rate (Ready → NotReady transitions per hour)
- [ ] Dashboard for etcd DB size, raft index rate, and snapshot frequency

## Quick Diagnostic Commands

```bash
# Check etcd health and DB size
kubectl exec steward-etcd-0 -n steward-system -- etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/etcd/pki/ca.crt \
  --cert=/etc/etcd/pki/server.pem \
  --key=/etc/etcd/pki/server-key.pem \
  endpoint status --write-out=table

# Count slow requests (should be 0 in healthy state)
kubectl logs steward-etcd-0 -n steward-system --since=60s | grep -c "took too long"

# Check raft consensus latency distribution
kubectl logs steward-etcd-0 -n steward-system --since=60s | \
  grep "agreement among raft" | \
  sed 's/.*duration: //' | sed 's/).*//'

# Check snapshot frequency (how often snapshots trigger)
kubectl logs steward-etcd-0 -n steward-system --since=1h | grep "triggering snapshot"

# Verify etcd resources are set
kubectl get statefulset steward-etcd -n steward-system \
  -o jsonpath='{.spec.template.spec.containers[0].resources}'

# Check leader heartbeat health
for i in 0 1 2; do
  echo "=== steward-etcd-$i ==="
  kubectl logs steward-etcd-$i -n steward-system --since=5m | \
    grep "leader failed to send out heartbeat"
done

# Check TCP pod container restarts
kubectl get pods -l steward.butlerlabs.dev/component -A \
  -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name}{"\t"}{range .status.containerStatuses[*]}{.name}={.ready}/{.restartCount} {end}{"\n"}{end}'

# Check etcd snapshot-count setting
kubectl get statefulset steward-etcd -n steward-system \
  -o jsonpath='{.spec.template.spec.containers[0].command}' | \
  python3 -c "import json,sys; [print(c) for c in json.load(sys.stdin) if 'snapshot' in c]"
```
