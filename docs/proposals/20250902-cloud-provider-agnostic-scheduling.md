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

The goal is to preserve the same fast-fail behavior in BYO scenario but remove cloud/SKU coupling when instanceType is empty by basing capacity checks on provider-neutral runtime node attributes discovered via node labels and keeping homogeneous placement across identical node configurations (defined by GPU product, count, and memory). BYO will be supported by leaving instanceType empty (or even leaving the entire resource field empty), while NAP scenarios will continue to rely on `workspace.resource.instanceType` for provisioning decisions.

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

Through Helm chart dependency:

```yaml
dependencies:
  - name: nvidia-device-plugin # need to remove existing DaemonSet
    version: 0.17.2
    repository: https://nvidia.github.io/k8s-device-plugin
  - name: gpu-feature-discovery
    version: 0.17.2
    repository: https://nvidia.github.io/k8s-device-plugin
```

### Scheduling Algorithm

In the current NAP scenarios, the KAITO controller creates NodeClaims until there are enough uniform nodes (based on instanceType) to meet the total GPU memory requirement (per-replica memory from the preset times replicas) and removes NodeClaims if there are excess nodes. For example, with a preset model requiring 8GB of GPU memory per replica, with `workspace.inference.replicas` set to 2 replicas, 16GB GPU memory is required. Using an instanceType with 80GB total GPU memory will create 2 nodes (one per replica, no bin-packing) even though one node could satisfy the total memory requirement.

For BYO scenarios, where `workspace.resource.instanceType` is empty, we propose a filter-score framework (partially inspired by kube-scheduler) to estimate the optimal group of nodes based on GPU attributes populated as node labels by GFD. To ensure inference pods originated from the same workspace are scheduled on homogeneous nodes, similar to NAP scenarios, we logically cache nodes into node groups identified by their GPU product, count, and memory.

### Filter-Score Framework

The filter-score framework consists of two main components: filters and scorers. Filters are responsible for narrowing down the list of nodes based on specific criteria, while scorers evaluate each node group and assign scores to them based on their suitability for the workload.

1. **Filters**: Implement the `Filter` interface to define custom filtering logic. Filters can be applied in a chain, with each filter further refining the list of candidate nodes and node groups.

2. **Scorers**: Implement the `Scorer` interface to define custom scoring logic. Scorers are responsible for evaluating the filtered nodes and assigning scores based on their attributes and the workload requirements.

### Go Structs and Interfaces

```go
// Node represents a Kubernetes node with methods to access various attributes.
type Node interface {
    // GetLabels returns all labels of the node.
    GetLabels() map[string]string

    // GetResources returns the resource capacities of the node.
    GetResources() resource.ResourceList

    // GetNodeGroupIdentifer extracts GPU configurations from node labels.
    GetNodeGroupIdentifer() *types.NodeGroupIdentifer
}

// NodeGroupIdentifer allows logical grouping of nodes based on node attributes.
type NodeGroupIdentifer struct {
    // GPUProduct is the GPU model/product name.
    GPUProduct string

    // GPUCount is the number of GPUs.
    GPUCount int64

    // GPUMemory is the memory per GPU.
    GPUMemory resource.Quantity
}

// Filter is used to filter nodes based on custom criteria, such as matching node labels
type Filter interface {
    Filter(ctx context.Context, workspace kaitov1beta1.Workspace, nodeGroups map[types.NodeGroupIdentifer][]types.Node) map[types.NodeGroupIdentifer][]types.Node
}

// Scorer assigns scores to node configurations to prefer certain setups (e.g. minimizing wasted GPU memory)
type Scorer interface {
    Score(ctx context.Context, workspace kaitov1beta1.Workspace, nodeGroups map[types.NodeGroupIdentifer][]types.Node) map[types.NodeGroupIdentifer]float64
}
```

```go
type Scheduler struct {
    filters  []Filter
    scorers  []Scorer
    nodeCache map[types.NodeGroupIdentifer][]types.Node // Cached nodes grouped by NodeGroupIdentifer
}

func (s *Scheduler) Schedule(ctx context.Context, workspace kaitov1beta1.Workspace) types.NodeGroupIdentifer {
    // 1. Filter nodes
    filteredNodes := s.nodeCache
    for _, filter := range s.filters {
        filteredNodes = filter.Filter(ctx, workspace, filteredNodes)
    }

    // 3. Score groups
    scores := make(map[types.NodeGroupIdentifer]float64)
    for _, scorer := range s.scorers {
        scorerScores := scorer.Score(ctx, workspace, filteredNodes)
        for config, score := range scorerScores {
            if _, exists := scores[config]; !exists {
                scores[config] = 0
            }
            scores[config] += score
        }
    }

    // 4. Select best group based on scores and capacity requirements
    var bestScore float64 = -1
    var bestConfig types.NodeGroupIdentifer
    for config, score := range scores {
        if score > bestScore {
            bestScore = score
            bestConfig = config
        }
    }

    return bestConfig
}
```

### Proposed Filters and Scorers

#### Filters

| Filter Name            | Description                                                                                                                                           |
|------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------|
| **LabelFilter**        | Filters nodes based on required labels specified in the workspace. In NAP scenarios, this includes `node.kubernetes.io/instance-type=<instanceType>`. |
| **NodeGroupSizeFilter** | Filters out node groups that do not have enough nodes to satisfy the replica count (e.g. if replicas=2, a node group with only 1 node is filtered out). |
| **NodeResourceFilter** | Filters nodes that do not have `nvidia.com/gpu` resources.                                                                                            |
| **CapacityFilter**     | Filters node groups that cannot meet the total GPU memory requirement (preset.TotalGPUMemoryRequirement * .inference.replicas).                       |

#### Scorers

| Scorer Name                       | Description                                                                                                                                           |
|-----------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------|
| **ResourceUtilizationScorer**     | Scores node configurations based on how much buffer memory would be wasted after allocating the required per-replica memory. Lower buffer (without OOM) leads to higher GPU memory utilization, thus a higher score. |

### Examples

Say we have the following nodes group in the cluster:

| NodeGroupIdentifer                                    | Number of Nodes |
|-------------------------------------------------------|-----------------|
| `{GPUProduct: "A100", GPUCount: 4, GPUMemory: 40960}` | 2               |
| `{GPUProduct: "A100", GPUCount: 8, GPUMemory: 81920}` | 1               |
| `{GPUProduct: "A10", GPUCount: 1, GPUMemory: 24576}`  | 1               |

Given a preset model requiring 8GB of GPU memory per replica, with `workspace.inference.replicas` set to 2 replicas, 16GB GPU memory is required.

### Validating Webhook

The validating webhook performs fast-fail checks during Workspace creation to ensure model feasibility before admission. Note that if user increases workspace.inference.replicas later, the Workspace controller will NOT re-schedule existing inference workloads; it will only update the replica count of the inference Deployment/LeaderWorkerSet, which may lead to pod(s) being stuck in Pending state if there is insufficient capacity in the cluster. This behavior ensures that scheduling decision is made only once at creation time (aligns with scheduling best practices and user expectations), avoiding disruptions from re-scheduling existing inference workloads.

#### BYO Scenario

1. If workspace.resource.instanceType is empty, invoke the Scheduler's Schedule method with the preset.
2. If no NodeGroupIdentifer is returned, deny admission.
3. Raise a descriptive denial message: "No suitable node group found: insufficient capacity or no matching configurations after filtering."
4. This preserves fail-fast without creating the inference resource.

### Workspace Controller

The controller reconciles Workspace CRs, creating or updating inference resources and deploying the inference workload.

#### BYO Scenario

1. Invoke Scheduler's Schedule to get the best NodeGroupIdentifer.
1. If none, set Workspace status conditions to failed and requeue.
1. With the returned NodeGroupIdentifer, create the Deployment/LeaderWorkerSet with the same node selector as `workspace.resource.labelSelector`, plus:
   - `nvidia.com/gpu.product=<GPUProduct>` to ensure homogeneous GPU product
   - resource claim: `nvidia.com/gpu: <GPUCount>` to request the maximum number of GPUs

### Migration

Existing Workspaces with instanceType will continue using NAP logic unchanged. To migrate to BYO, users can pre-provision GPU nodes and create new Workspaces without instanceType. Ensure nodeCache is populated via a separate controller watching Node resources.

## Work Items

- [ ] ⁠Deprecate `.resource.preferredNodes` in Workspace CRD and remove `disableNodeAutoProvisioning` feature gate
- [ ] Implement BYO Scheduler framework
- [ ] Convert `.resource.instanceType` to an optional field with no default value
