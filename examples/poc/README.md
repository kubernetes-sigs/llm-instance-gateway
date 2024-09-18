# Envoy Ext Proc Gateway with LoRA Integration

This project sets up an Envoy gateway to handle gRPC calls with integration of LoRA (Low-Rank Adaptation). The configuration aims to manage gRPC traffic through Envoy's external processing and custom routing based on headers and load balancing rules. The setup includes Kubernetes services and deployments for both the gRPC server and the vllm-lora application.

## Requirements
- A vLLM based deployment (using the custom image provided below), with LoRA Adapters
- Kubernetes cluster
- Envoy Gateway v1.1 installed on your cluster: https://gateway.envoyproxy.io/v1.1/tasks/quickstart/
- `kubectl` command-line tool
- Go (for local development)

## vLLM
***This PoC uses a modified vLLM fork, the public image of the fork is here: `ghcr.io/tomatillo-and-multiverse/vllm:demo`***

The fork is here: https://github.com/kaushikmitr/vllm.

The summary of changes from standard vLLM are:
- Active/Registered LoRA adapters are returned as a response header (used for lora-aware routing)
- Queue size is returned as a response header
- Active/Registered LoRA adapters are emitted as metrics (for out-of-band scraping during low traffic periods)


## Overview

This project contains the necessary configurations and code to set up and deploy a service using Kubernetes, Envoy, and Go. The service involves routing based on the model specified (using Open AI API format), collecting metrics, and ensuring efficient load balancing.

![alt text](./envoy-gateway-bootstrap.png)


## Quickstart

### Steps

1. **Apply Kubernetes Manifests**
   ```bash
   cd manifests
   kubectl apply -f ext_proc.yaml
   kubectl apply -f vllm/vllm-lora-service.yaml
   kubectl apply -f vllm/vllm-lora-deployment.yaml
   ```

2. **Update `ext_proc.yaml`**
   - Ensure the `ext_proc.yaml` is updated with the pod names and internal IP addresses of the vLLM replicas. This step is crucial for the correct routing of requests based on headers.

2. **Update and apply `gateway.yaml`**
   - Ensure the `gateway.yaml` is updated with the internal IP addresses of the ExtProc service. This step is also crucial for the correct routing of requests based on headers.
    ```bash
   cd manifests
   kubectl apply -f gateway.yaml
   ```

### Monitoring and Metrics

- The Go application collects metrics and saves the latest response headers in memory.
- Ensure Envoy is configured to route based on the metrics collected from the `/metric` endpoint of different service pods.

## Contributing

1. Fork the repository.
2. Create a new branch.
3. Make your changes.
4. Open a pull request.

## License

This project is licensed under the MIT License.

---