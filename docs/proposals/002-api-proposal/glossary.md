# Glossary

This is a glossary that deep-dives on terms used within the api proposal, in an effort to give context to the API decisions 

<!-- toc -->
-   [API Terms](#api-terms)
    - [BackendPool](#backendpool)
-   [Priority](#priority)
-   [Fairness](#fairness)
-   [Lora Affinity](#lora-affinity)
-   [Latency Based Routing](#latency-based-routing)


<!-- /toc -->

## API Terms
This is a very brief description of terms used to describe API objects, this is included only if the glossary is the first doc you are reading.

### BackendPool
A grouping of model servers that serve the same set of fine-tunes (LoRA as a primary example).

### UseCase
An LLM workload that is defined and runs on a BackendPool with other use cases.
 
## Priority

### Summary
Priority specifies the importance of a UseCase relative to other usecases within a BackendPool. 

### Description

For our purposes, priority can be thought of in two classes:
- Critical
- Non-Critical

The primary difference is that non-critical UseCase requests will be rejected in favor of Critical UseCases the face of resource scarcity. 

Example: 

Your current request load is using 80 Arbitrary Compute Units(ACU) of your pools total of 100ACU capacity. 40ACU are critical workload requests, 45 are non-critical. If you were to lose 30 ACU due to an unforseen outage. Priority would dictate that of the 10 surplus ACU to be rejected the entirety of them would be from the non-critical requests. 

## Fairness

## Lora Affinity

## Latency Based Routing