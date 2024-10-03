# Glossary

This is a glossary that attempts to more thoroughly explain terms used within the api proposal, in an effort to give context to API decisions.

<!-- toc -->
- [API Terms](#api)
    - [LLMServerPool](#llmserverpool)
    - [LLMService](#llmservice)
- [Capacity Constrained Routing](#capacity-constrained-routing)
    -   [Priority](#priority)
    -   [Fairness](#fairness)
- [General Routing](#general-routing)
    -   [Latency Based Routing](#latency-based-routing)
    -   [Lora Affinity](#lora-affinity)


<!-- /toc -->

## API
This is a very brief description of terms used to describe API objects, included for completeness.

### LLMServerPool
A grouping of model servers that serve the same set of fine-tunes (LoRA as a primary example). 

Shortened to: `LSP`

### LLMService
An LLM workload that is defined and runs on a LLMServerPool with other use cases.

# Capacity Constrained Routing

## Priority

### Summary
Priority specifies the importance of a LLMService relative to other services within a LLMServerPool. 

### Description

For our purposes, priority can be thought of in two classes:
- Critical
- Non-Critical

The primary difference is that non-critical LLMService requests will be rejected in favor of Critical LLMServices the face of resource scarcity. 

Example: 

Your current request load is using 80 Arbitrary Compute Units(ACU) of your pools total of 100ACU capacity. 40ACU are critical workload requests, 40 are non-critical. If you were to lose 30 ACU due to an unforseen outage. Priority would dictate that of the 10 surplus ACU to be rejected, the entirety of them would be from the _non-critical_ requests. 

## Fairness

### Summary
Fairness specifies how resources are shared among different LLMServices, in a way that is most acceptable to the user.

### Description

Fairness, like priority, is only used in resource scarcity events. 

Fairness is utilized when requests of the same priority class need to be rejected, or queued. There are many dimensions that could be considered when considering shared resources. To name a few:
- KV-cache utilization
- Total request count
- SLO adherence

For the v1 MVP, the only objective a User can specify is the SLO objective they would like to meet. So, in following that pattern, fairness in MVP will simply be considered for SLO adherence. SLO Adherence is only being considered over a rolling time window of data. 

The TTL we are currently assuming is: `5 min` 

### Example

**Assumption:** Services have equally weighted fairness for this example.

- Service A has been meeting its SLO 98% of the requests made in the time window, and Service B has met the SLO 94% of the time.

- A request for both Service A and Service B come in at the same time, and there is only capacity to start a single new request in the LSP, this capacity would meet the SLO for both services. The other request would be queued (potentially causing that request to not meet SLO).

- To fairly share these resources. Service B *must* be selected to begin the request immediately as Service A has had its SLO met a larger percentage of the time.

# General Routing
Different from the previous definitons, these terms are used to describe methods of routing that are constant, and seek to better utilize compute resources to avoid capacity constraints as much as possible.

## Latency Based Routing

### Summary
Latency Based Routing uses data to ensure LLMServices meet their specified SLO.

### Description
Data collected from the model servers and data collected from the request is used to predict the time a request will take on a *specific* model server, and route in a way that will best satisfy the SLO of the incoming requests.

## Lora Affinity

### Summary
LoRA Affinity describes the routing strategy displayed in the [demo](https://youtu.be/NUBZg_uqqXk?si=v681EeYdGUGEVqQQ&t=1458), to better utilize Model Servers within the LSP.

### Description
Model Servers that support multi-LoRA handle requests in a FCFS basis. By utilizing the data provided by the model server (the state of loaded LoRA adapters), a routing system can route requests for a given LoRA adapter, to a model server that already has that adapter loaded, to create larger batches than a naive route, which better utilizes the model server hardware. 