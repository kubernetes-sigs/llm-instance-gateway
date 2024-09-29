# Envoy Ext Proc Gateway with LoRA Integration

This project sets up an Envoy gateway with a custom external processing which implements advanced routing logic tailored for LoRA (Low-Rank Adaptation) adapters. The routing algorithm is based on the model specified (using Open AI API format), and ensuring efficient load balancing based on model server metrics.

![alt text](./envoy-gateway-bootstrap.png)

## Requirements

- Kubernetes cluster
- Envoy Gateway v1.1 installed on your cluster: https://gateway.envoyproxy.io/v1.1/tasks/quickstart/
- `kubectl` command-line tool
- Go (for local development)
- A vLLM based deployment using a custom fork, with LoRA Adapters.  ***This PoC uses a modified vLLM [fork](https://github.com/kaushikmitr/vllm), the public image of the fork is here: `ghcr.io/tomatillo-and-multiverse/vllm:demo`***. A sample deployement is provided under `./manifests/samples/vllm-lora-deployment.yaml`.

## Quickstart

### Steps

1. **Deploy Sample vLLM Application**
   NOTE: Create a HuggingFace API token and store it in a secret named `hf-token` with key hf_api_token`. This is configured in the `HUGGING_FACE_HUB_TOKEN` and `HF_TOKEN` environment variables in `./manifests/samples/vllm-lora-deployment.yaml`.

   ```bash
   kubectl apply -f ./manifests/samples/vllm-lora-deployment.yaml
   kubectl apply -f ./manifests/samples/vllm-lora-service.yaml
   ```

2. **Install GatewayClass with Ext Proc**
   A custom GatewayClass `llm-gateway` which is configured with the llm routing ext proc will be installed into the `llm-gateway` namespace. It's configured to listen on port 8081 for traffic through ext-proc (in addition to the default 8080), see the `EnvoyProxy` configuration in `installation.yaml`. When you create Gateways, make sure the `llm-gateway` GatewayClass is used.

   NOTE: Ensure the `llm-route-ext-proc` deployment is updated with the pod names and internal IP addresses of the vLLM replicas. This step is crucial for the correct routing of requests based on headers. This won't be needed once we make ext proc dynamically read the pods.

   ```bash
   kubectl apply -f ./manifests/installation.yaml
   ```

3. **Deploy Gateway**

   ```bash
   kubectl apply -f ./manifests/samples/gateway.yaml
   ```

4. **Try it out**
   Wait until the gateway is ready.

   ```bash
   IP=$(kubectl get gateway/llm-gateway -o jsonpath='{.status.addresses[0].value}')
   PORT=8081

   curl -i ${IP}:${PORT}/v1/completions -H 'Content-Type: application/json' -d '{
   "model": "tweet-summary",
   "prompt": "Write as if you were a critic: San Francisco",
   "max_tokens": 100,
   "temperature": 0
   }'
   ```

## License

This project is licensed under the MIT License.
