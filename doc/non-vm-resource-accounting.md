# Non-VM Resource Accounting in the Autoscaling Scheduler

This document describes how the autoscale-scheduler plugin and autoscaler-agent account for
resources consumed by non-VM workloads on a node.

There are three distinct layers, each responsible for a different category of consumer.

---

## 1. OS / Kubelet / System Processes — via `node.Status.Allocatable`

`NodeStateFromK8sObj` (`pkg/plugin/state/node.go:123-133`) initialises node capacity from:

```go
cpuQ := node.Status.Allocatable.Cpu()
memQ := node.Status.Allocatable.Memory()
```

`Allocatable` is Kubernetes' pre-computed value:

```
Allocatable = Capacity - kube-reserved - system-reserved - eviction-threshold
```

This subtraction is performed by the kubelet and is entirely outside this codebase. The scheduler
plugin simply trusts whatever K8s reports as `Allocatable` and uses it as `Node.CPU.Total` /
`Node.Mem.Total`.

**Consequence:** if `kube-reserved` / `system-reserved` are not configured on the nodes, OS and
kubelet processes compete with pods for the same capacity pool and the scheduler has no visibility
into that pressure.

---

## 2. Non-VM Pods (DaemonSets, system pods, etc.) — tracked at declared requests, no overcommit

`PodStateFromK8sObj` (`pkg/plugin/state/pod.go:107-113`) branches on whether the pod belongs to a
VirtualMachine:

```go
if vmRef, ok := vmv1.VirtualMachineOwnerForPod(pod); ok {
    return podStateForVMRunner(pod, vmRef)
} else {
    return podStateForNormalPod(pod), nil
}
```

For non-VM pods, `podStateForNormalPod` sums all container **resource requests** and assigns an
overcommit factor of `1.0` (i.e. no discount):

```go
for _, container := range pod.Spec.Containers {
    cpu += vmv1.MilliCPUFromResourceQuantity(*container.Resources.Requests.Cpu())
    mem += api.BytesFromResourceQuantity(*container.Resources.Requests.Memory())
}
// Overcommit = 1000m = 1.0
```

These pods feed directly into `Node.CPU.Reserved` / `Node.Mem.Reserved`, which is the counter
checked by `OverBudget()` and used in scoring. They are marked `Migratable: false`, so the
scheduler will never attempt to move them to relieve node pressure.

**Consequences:**
- A non-VM pod with no resource requests declared contributes **zero** to `Reserved`, regardless
  of its actual runtime CPU/memory usage.
- A non-VM pod with resource requests is accounted at those requests permanently — there is no
  mechanism for the scheduler to observe its actual consumption.

---

## 3. Pods in `IgnoredNamespaces` — invisible to node state, but visible during Filter

The `IgnoredNamespaces` config field (`pkg/plugin/config.go:69-77`) is designed for
over-provisioning placeholder pods used to trigger cluster-autoscaler. Pods in these namespaces are
dropped before they touch node state:

```go
// HandlePodEvent, handle_pod.go:30-33
if s.config.ignoredNamespace(pod.Namespace) {
    return nil, nil
}
```

However, the `Filter` method reconciles the plugin's local state with the **K8s core scheduler's
proposed pod list** (`nodeInfo.Pods`). Since the core scheduler sees all pods regardless of
namespace, ignored-namespace pods appear in `proposedPods` and are temporarily added to the node
via `PodStateFromK8sObj` for the filter-only capacity check.

This is intentional: an over-provisioning placeholder pod **can cause a VM to be filtered out** of
a node, which prompts cluster-autoscaler to evict the placeholder and scale the cluster up.

---

## 4. The Autoscaler-Agent Has No Node-Level View

The autoscaler-agent knows nothing about other workloads on the same node. Its Goal CU calculation
(`pkg/agent/core/goalcu.go`) is driven entirely by metrics from inside its own VM:

- CPU load average (1-minute and 5-minute, blended)
- Memory usage bytes
- Linux page-cache working set size (LFC)

The agent relies entirely on the scheduler plugin's `Permit` response to enforce node-level budget.
If a non-VM pod spikes far above its declared requests, the agent receives no signal — only the
node's OS-level CPU scheduler experiences the contention.

---

## Summary

| Resource consumer | How it is accounted |
|---|---|
| OS / kubelet / system daemons | Subtracted from `Allocatable` at the kubelet level; not visible to the plugin |
| Non-VM pods — requests declared | Tracked at declared requests, overcommit = 1.0, never migrated |
| Non-VM pods — no requests declared | **Not accounted** — contribute 0 to `Reserved` |
| `IgnoredNamespaces` pods | Invisible to node state; visible to `Filter` only (to allow eviction) |
| Actual runtime usage of any non-VM pod | **Not tracked** — only `requests` matter to the scheduler plugin |

---

## Relevant Source Locations

| File | Purpose |
|---|---|
| `pkg/plugin/state/node.go:110-141` | Reads `Allocatable` to set `Node.CPU.Total` / `Node.Mem.Total` |
| `pkg/plugin/state/pod.go:115-152` | `podStateForNormalPod` — sums requests, sets overcommit = 1.0 |
| `pkg/plugin/state/pod.go:154-248` | `podStateForVMRunner` — reads approved/requested scaling annotations |
| `pkg/plugin/handle_pod.go:30-33` | Early-exit for ignored namespaces |
| `pkg/plugin/framework_methods.go:171-265` | `filterCheck` — reconciles local state with K8s proposed pods |
| `pkg/plugin/config.go:69-77` | `IgnoredNamespaces` field and its documented intent |
| `pkg/agent/core/goalcu.go` | Agent goal CU — purely intra-VM metrics, no node-level view |
