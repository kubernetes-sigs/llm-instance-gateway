
# Gateway API Inference Extension

## Proposal Status
 ***Draft***

## Table of Contents

<!-- toc -->

-   [Summary](#summary)
-   [Goals](#goals)
-   [Non-Goals](#non-goals)
-   [Proposal](#proposal)
    -   [Personas](#personas)
        -   [Inference Platform Admin](#inference-platform-admin)
        -   [Inference Workload Owner](#workload-owner)
    -   [Axioms](#axioms)
    -   [InferencePool](#inferencepool)
    -   [InferenceModel](#inferencemodel)
    -   [Spec](#spec)
    -   [Diagrams](#diagrams)
    -   [Alternatives](#alternatives)
- [FAQ](#faq)
- [Open Questions](#open-questions)
    
<!-- /toc -->

## Summary

This proposal presents 2 new CRD objects to express the needs of the LLM Instance Gateway. **InferencePool** and **InferenceModel**. The InferencePool is the logical grouping of compute, owned by the Inference Platform Admin persona. While the InferenceModel defines the serving objectives of a specific model or LoRA adapter, and is owned by the Inference Workload Owner.

**NOTE: Some routing terms are defined in the [glossary](./glossary.md) file, to more deeply describe how we will handle behaviors like priority and fairness**

## Goals

- Drive concensus on direction of LLM Instance Gateway Solution
- Documentation of API decisions for posterity

## Non-Goals

- Hash out every implementation detail
- Be a formal KEP

## Proposal

### Personas

Before diving into the details of the API, decriptions of the personas will help shape the thought process of the API design.

#### Inference Platform Admin

The Inference Platform Admin creates and manages the infrastructure necessary to run LLM workloads. Including handling Ops for: 
  - Hardware
  - Model Server
  - Base Model
  - Resource Allocation for Workloads
  - Gateway configuration
  - etc

#### Inference Workload Owner

A Inference Workload Owner persona owns and manages 1 or many Generative AI Workloads (LLM focused *currently*). This includes:
- Defining importance
- Managing fine-tunes
  - LoRA Adapters
  - System Prompts
  - Prompt Cache
  - etc.
- Managing rollout of adapters

### Axioms 

The API design is based on these axioms:

- Pools of shared compute should be *discrete* for scheduling to properly work
- Pod-level scheduling should not be handled by a high-level gateway 
- Simple services should be simple to define (or are implicitly defined via reasonable defaults)
- This solution should be composable with other Gateway solutions and flexible to fit customer needs
- The MVP will heavily assume requests are done using the OpenAI spec, but open to extension in the future
- The Gateway should route in a way that does not generate a queue of requests at the model server level

The [PoC](https://youtu.be/NUBZg_uqqXk?si=v681EeYdGUGEVqQQ&t=1458) was focused on lower-level scheduling. And the API follows that similar logic, which lead to the proposal of the **InferencePool**.

### InferencePool

The InferencePool at its core is a logical grouping of compute, expressed in the form of Pods (typically model servers), akin to a K8s Service. The InferencePool would deploy its own routing, and offer administrative configuration to the Platform Admin. 

 It is expected for the InferencePool to:
 - Enforce fair consumption of resources across competing workloads
 - Efficiently route requests across shared compute (as displayed by the PoC)
 
It is _not_ expected for the InferencePool to:
 - Enforce any common set of adapters or base models are available on the Pods
 - Manage Deployments of Pods within the Pool
 - Manage Pod lifecycle of pods within the pool 

Additionally, any Pod that seeks to join a InferencePool would need to support a protocol, defined by LLM Instance Gateway, to ensure the Pool has adequate information to intelligently route requests.

### InferenceModel

A InferenceModel allows the Inference Workload Owner to define:
- Which LoRA adapter(s) to consume 
  - InferenceModel allows for traffic splitting between adapters _in the same InferencePool_ to allow for new LoRA adapter versions to be easily rolled out 
- SLO objectives for the InferenceModel
- The Pools this InferenceModel is relevant to 

### Spec

**InferenceModel**
```golang
// InferenceModel represents a set of Models/Adapters that are multiplexed onto one 
// or more backend pools. This resource is managed by the "Inference Workload Owner"
// persona. The Inference Workload Owner persona is: a team that trains, verifies, and
// leverages a large language model from a model frontend, drives the lifecycle
// and rollout of new versions of those models, and defines the specific
// performance and latency goals for the model. These workloads are
// expected to operate within a InferencePool sharing compute capacity with other
// InferenceModels, defined by the Inference Platform Admin. We allow a user who
// has multiple InferenceModels across multiple pools (with the same config) to
// specify the configuration exactly once, and deploy to many pools 
// simultaneously. Enabling a simpler config and single source of truth
// for a given user. InferenceModel names are unique for a given InferencePool,
// if the name is reused, an error will be  shown on the status of a 
// InferenceModel that attempted to reuse. The oldest InferenceModel, based on
// creation timestamp, will be selected to remain valid. In the event of a race
// condition, one will be selected at random. 
type InferenceModel struct {
        metav1.ObjectMeta
        metav1.TypeMeta

        Spec InferenceModelSpec
}

type InferenceModelSpec struct {
        // The name of the model as the users set in the "model" parameter in the requests.
        // The name should be unique among the workloads that reference the same backend pool.
        // This is the parameter that will be used to match the request with. In the future, we may
        // allow to match on other request parameters. The other approach to support matching on 
        // on other request parameters is to use a different ModelName per HTTPFilter.
        // Names can be reserved without implementing an actual model in the pool.
        // This can be done by specifying a target model and setting the weight to zero,
        // an error will be returned specifying that no valid target model is found.
        ModelName string
        // Optional
        // Defines how important it is to serve the model compared to other models referencing the same pool.
        Criticality *Criticality
        // Optional.
	    // Allow multiple versions of a model for traffic splitting. 
	    // If not specified, the target model name is defaulted to the modelName parameter.
        // modelName is often in reference to a LoRA adapter.
        TargetModels []TargetModel
        // Reference to the backend pools that the model registers to.
        PoolRef []corev1.ObjectReference
}

// Defines how important it is to serve the model compared to other models.
type Criticality string
const (
    // Most important. Requests to this band will be shed last.
    Critical  Criticality = "Critical"
    // More important than Sheddable, less important than Critical.
    // Requests in this band will be shed before critical traffic.
    Default  Criticality = "Default"
    // Least important. Requests to this band will be shed before all other bands.
    Sheddable  Criticality = "Sheddable"
 )

// TargetModel represents a deployed model or a LoRA adapter. The
// Name field is expected to match the name of the LoRA adapter
// (or base model) as it is registered within the model server. Inference
// Gateway assumes that the model exists on the model server and is the
// responsibility of the user to validate a correct match. Should a model fail
// to exist at request time, the error is processed by the Instance Gateway,
// and then emitted on the appropriate InferenceModel object.
type TargetModel struct {
        // The name of the adapter as expected by the ModelServer.
        Name string
        // Weight is used to determine the percentage of traffic that should be 
        // sent to this target model when multiple versions of the model are specified.
        Weight int
}
```

**InferencePool**
```golang
// The InferencePool is a construct for pooling compute (often model servers) to
// serve large models, that have the ability to share capacity across multiple
// services (such as through prompt engineering, LoRA adapters, etc).
// InferencePools have a dependency on a Gateway that is compatible with ext-proc
// (External Processing). When a new LSP object is created, a new ext proc
// deployment is created. InferencePools require at minimum a single InferenceModel to
// be subscribed to them to accept traffic, any traffic with a model not
// definied within a InferenceModel will be rejected.
type InferencePool struct {
        metav1.ObjectMeta
        metav1.TypeMeta

        Spec InferencePoolSpec
}

type InferencePoolSpec struct {
        // ModelServerSelector uses label selection to watch model server pods
        // that should be included in the InferencePool. ModelServers should not
        // be with any other Service or InferencePool, that behavior is not supported
        // and will result in sub-optimal utilization.
        ModelServerSelector map[string]string `json:"modelServerSelector,omitempty"`
}
```

### Yaml Examples

#### InferencePool(s)
Here we create 2 LSPs that subscribe to services to collect the appropriate pods
```yaml
apiVersion: inference.x-k8s.io/v1alpha1
kind: InferencePool
metadata:
  name: llama-2-pool
  services: 
  - llama-2-vllm
---
apiVersion: inference.x-k8s.io/v1alpha1
kind: InferencePool
metadata:
  name: gemini-pool
  services: 
  - gemini-jetstream-tpu-v5e
  - gemini-vllm-a100
```

#### InferenceModel

Here we consume both pools with a single InferenceModel, while also specifying 2 InferenceModels. Where `sql-code-assist` is both the name of the ModelInferenceModel, and the name of the LoRA adapter on the model server. And `npc-bot` has a layer of indirection for those names, as well as a specified objective. Both `sql-code-assist` and `npc-bot` have available LoRA adapters on both InferencePools and routing to each InferencePool happens earlier(at the K8s Gateway). So traffic splitting between separate pools happens at the K8s Gateway.
```yaml
apiVersion: inference.x-k8s.io/v1alpha1
kind: InferenceModel
metadata:
  name: my-llm-service
spec:
  InferenceModels:
  - modelName: sql-code-assist
  - modelName: npc-bot
    targetModels:
      targetModelName: npc-bot-v1
        weight: 50
      targetModelName: npc-bot-v2
        weight: 50 	
  poolRef: 
   - name: llama-2-pool
   - name: gemini-pool
```

### Diagrams

Much of this is better explained visually:

Below is a detailed view of the InferencePool

![InferencePool](./images/lsp.svg)

This diagram lightly follows the example request for a model `name-generator`. 
The flow can be described as:
- The request comes in to our routing solution(Ext-Proc)
- ExtProc looks up the InferenceModels affiliated with this pool `examplePool`
- `name-generator` is currently undergoing a change of LoRA adapters from `name-generator-v3` (20% traffic split) to `name-generator-v2` (80% traffic split)
- `name-generator-v2` is selected as the LoRA adapter, and replaces `name-generator` in the body of the request (mutated by ext-proc) 
- the request is then efficiently scheduled onto one of the valid Pods
- Prometheus metrics are sent back to the LSP, aggregated and re-emitted via sidecar (following the metric standardization)

How Multiple InferencePools might integrate together:

![K8s Gateway with InferencePools](./images/gw_w_lsp.svg)

Here we see that we can have:
- Multiple Routes pointing to the same pool
- Routes splitting traffic across multiple pools

The functionality of the Kubernetes Gateway is unchanged with this proposal, allowing seamless integration with the InferencePool.


### Alternatives

#### Key Decisions

Our alternatives hinge on some key decisions:
- Allowing HTTPRoute to treat the InferencePool as the backendRef
  - Whereas the alternatives might have the InferenceModel as the backend ref
- Creating a separate layer of abstraction, instead of extending HTTPRoute
  - Explained in more detail in the LLMRoute section

#### InferenceModel as a backend ref

We toyed with the idea of allowing an InferenceModel be the target of an HTTPRouteRules backend ref. However, doing so would require the Kubernetes Gateway to be able to interpret body level parameters (assuming OpenAI protocol continues to require the model param in the body), and require that the HTTPRoute also specify the backend the InferenceModel is intended to run on. Since we our primary proposal already specifies the backend, packing this functionality would require substantial work on the Kubernetes Gateway, while not providing much flexibility.

#### LLMRoute

Our original idea was to define all InferenceModel config at the Kubernetes Gateway layer, and have no InferencePool. This is inherently challenging, as LLMRoute would become a superset of HTTPRoute, or the Gateway would become bespoke, and work only for the LLMRoute use case.

## FAQ
- **Why 2 layers of weighting?** (HttpRoute & InferenceModel)
  - Feasibly done - No extension of HttpRoute. Just works, as InferencePool operates like a service.
  - Complexity is only expressed during transition states (model version upgrade)
  - Keeps Pools self contained - multiple K8s gateways can direct traffic to the same pool without needing to re-express Pool-level behavior
- **What is a LSP attempting to define?**
  - InferencePool groups resources that should be shared over the InferenceModels that are affiliated with the pool
  - Best practice would also suggest keeping the same base model for all ModelServers in the pool, but that is not enforced
- **Can a InferenceModel reference multiple LSPs?**
- **How is this deployed?**
  - We will follow [common patterns](https://gateway.envoyproxy.io/docs/tasks/quickstart/#installation) to install the CRDs & Controllers
- **Are all controllers necessary for this solution going to be provided by Instance Gateway(this repo)?**
  - Yes




## Open Questions

- Reasonable defaults (how do we behave in the absence of user-specified values in optional fields)
  - Should services be required? Or can a customer simply create a pool, and direct requests to the pool, and expect even fairness/priority across the different LoRA adapters that are requested?
    - If so? How should we handle the mix between explicit and implicit services? Are implicit InferenceModels just default everything? (and inherently lower prio).
    - NOTE: Current thinking is this is yes we should allow non-use case defined requests, but is a security risk if on by default. So pools should opt-in
- Configuration control
  - How many routing decisions should we make on behalf of the user vs allow for configuration?
     - Do we decide that SLO adherence is stricter than Fairness adherence? Do we allow for configuration of such tooling? (would be expressed in the InferencePool API)
