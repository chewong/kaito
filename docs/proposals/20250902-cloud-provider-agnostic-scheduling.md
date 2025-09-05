---
title: Cloud Provider Agnostic Scheduling
authors:
  - "@chewong"
reviewers:
  - "@Fei-Guo"
  - "@zhuangqh"
creation-date: 2025-09-03
last-updated: 2025-09-03
status: draft
see-also:
---

## Goals

Today, KAITO fast-fails on potential OOM errors by ensuring the instance type's GPU count and memory obtained from cloud SKU maps is larger than the preset mode's total GPU memory requirement. This only works to a certain extent where Karpenter is installed beforehand and SKU maps are maintained (currently Azure and AWS). However, this requirement blocks adoption in BYO nodes scenario or other clouds without SKU maps.

The goal is to preserve the same fast-fail behavior in BYO scenario but remove cloud/SKU coupling when instanceType is empty by basing capacity checks on provider-neutral runtime node attributes discovered via node labels and keeping homogeneous placement across identical node configurations (defined by GPU product, count, and memory). BYO will be supported by leaving instanceType empty (or even leaving the entire resource field empty), while NAP scenarios will continue to rely on ``workspace.resource.instanceType`` for provisioning decisions.

## Non-Goals

We are not changing the validating webhook or NAP semantics; we only change where capacity data for existing nodes comes from (provider-neutral node attributes rather than SKU maps). We are not adding a dynamic VRAM estimator (done by #1178), packaging or installing any specific labeler, extending to non-NVIDIA accelerators, or integrating DRA/ResourceSlice yet. SKU maps remain for NAP provisioning decisions.

## Assumptions

- NVIDIA GPUs are used for inference workloads.
- In BYO cases, users pre-provision some nodes with GPUs and GPU drivers installed (via GPU operator or other means).
- Inference presets define the total GPU memory requirement per inference replica (e.g. 8GB for phi-4-mini-instruct).
- No bin-packing of multiple replicas on the same node; each replica may span multiple nodes for distributed inference if needed.

## NVIDIA GPU Feature Discovery (GFD)

One way to source provider‑neutral GPU attributes is NVIDIA [GPU Feature Discovery](https://github.com/NVIDIA/k8s-device-plugin/tree/main/docs/gpu-feature-discovery) (GFD), which publishes node labels such as `nvidia.com/gpu.count`, `nvidia.com/gpu.memory` (in MiB), and `nvidia.com/gpu.product`. GFD builds on [Node Feature Discovery](https://github.com/kubernetes-sigs/node-feature-discovery) (NFD) and targets NVIDIA GPUs; it aligns with the direction in issue #1222.

## Implementation

### Dependency Installation

Through Helm chart dependency defined in `Chart.yaml`:

```yaml
...
dependencies:
  - name: nvidia-device-plugin # need to remove existing DaemonSet
    version: 0.17.2
    repository: https://nvidia.github.io/k8s-device-plugin
  - name: gpu-feature-discovery
    version: 0.17.2
    repository: https://nvidia.github.io/k8s-device-plugin
...
```

### Proposed Node Selection Algorithm

In NAP scenarios, the KAITO controller provisions NodeClaims to create enough nodes of the same instanceType to satisfy the total GPU memory requirement (per-replica memory from the preset times replicas), and removes excess NodeClaims if needed. For instance, a preset requiring 8GB per replica with 2 replicas (total 16GB) on an 80GB instanceType would provision 2 nodes - one per replica, without bin-packing - despite one node sufficing for the memory.

For BYO scenarios (where `workspace.resource.instanceType` is empty), the proposed algorithm must address two key aspects:
- **Homogeneous placement**: All pods from the same Workspace must run on nodes with the same GPU product (e.g. all A100s or all H100s), consistent with NAP behavior and best practices.
- **Node capacity check**: Selected nodes must provide sufficient total GPU memory for the preset (per-replica memory times replicas); otherwise, the validating webhook rejects the Workspace creation or update.

The proposed algorithm is as follows:

- Workspace Validating Webhook:

  1. Confirm `workspace.resource.instanceType` is empty and `nvidia.com/gpu.product` is in labelSelector; fail if absent. This ensures homogeneous placement.
  2. List matching nodes; fail if there is no matching node.
  3. Verify uniformity: All nodes must have identical `nvidia.com/gpu.count` and `nvidia.com/gpu.memory`; fail if not.
  4. Calculate total GPU memory per node: `nvidia.com/gpu.count` * `nvidia.com/gpu.memory`.
  5. If the preset model does not support multi-node distributed inference (e.g. phi-4-mini-instruct), fail if the total GPU memory per node from 4) is less than the preset's total GPU memory requirement. The reason is that without multi-node distributed inference, each inference replica must fit entirely on a single node.

- Workspace Controller Reconciliation:
  1. Skip all NodeClaim-related operations if `workspace.resource.instanceType` is empty.
  2. Create/update underlying Deployment/LeaderWorkerSet with nodeSelector from `workspace.resource.labelSelector`.

>[!IMPORTANT]
> We do not enforce a minimum number of nodes during Workspace validation as long as at least one node matches the labelSelector (including `nvidia.com/gpu.product`), it provides the GPU count and memory hints for capacity computation.
>
> In NAP, KAITO handles provisioning or deleting the delta node amount to meet the desired total nodes based on `workspace.inference.replicas * workspace.status.perReplicaNodeCount`. In BYO, users are responsible for provisioning sufficient nodes; if not enough are available, pods may remain in Pending state, and KAITO will not remediate.

## Work Items

- [ ] ⁠Deprecate `.resource.preferredNodes` in Workspace CRD and remove `disableNodeAutoProvisioning` feature gate
- [ ] Install nvidia-device-plugin and gpu-feature-discovery via Helm chart dependencies, removing existing DaemonSet harcoded in the Workspace Helm chart
- [ ] Implement the proposed node selection algorithm in the KAITO Workspace controller
- [ ] Convert `.resource.instanceType` to an optional field with no default value
