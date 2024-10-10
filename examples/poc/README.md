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

   NOTE: Create a HuggingFace API token and store it in a secret named `hf-token` with key `token`. This is configured in the `HUGGING_FACE_HUB_TOKEN` and `HF_TOKEN` environment variables in `./manifests/samples/vllm-lora-deployment.yaml`.

   ```bash
   kubectl apply -f ./manifests/vllm/vllm-lora-deployment.yaml
   kubectl apply -f ./manifests/vllm/vllm-lora-service.yaml
   ```

1. **Update Envoy Gateway Config to enable Patch Policy**

   Our custom LLM Gateway ext-proc is patched into the existing envoy gateway via `EnvoyPatchPolicy`. To enable this feature, we must extend the Envoy Gateway config map. To do this, simply run:
   ```bash
   kubectl apply -f ./manifests/gateway/enable_patch_policy.yaml
   kubectl rollout restart deployment envoy-gateway -n envoy-gateway-system

   ```
   Additionally, if you would like the enable the admin interface, you can uncomment the admin lines and run this again.


1. **Deploy Gateway**

   ```bash
   kubectl apply -f ./manifests/gateway/gateway.yaml
   ```

1. **Deploy Ext-Proc**

   ```bash
   kubectl apply -f ./manifests/gateway/ext_proc.yaml
   kubectl apply -f ./manifests/gateway/patch_policy.yaml
   ```
   **NOTE**: Ensure the `instance-gateway-ext-proc` deployment is updated with the pod names and internal IP addresses of the vLLM replicas. This step is crucial for the correct routing of requests based on headers. This won't be needed once we make ext proc dynamically read the pods.

1. **Try it out**

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
