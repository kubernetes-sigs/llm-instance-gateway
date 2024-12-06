## Quickstart

### Requirements
The current manifests rely on Envoy Gateway [v1.2.1](https://gateway.envoyproxy.io/docs/install/install-yaml/#install-with-yaml) or higher.

### Steps

1. **Deploy Sample vLLM Application**

   A sample vLLM deployment with the proper protocol to work with LLM Instance Gateway can be found [here](https://github.com/kubernetes-sigs/llm-instance-gateway/blob/6f9869d6595d2d0f8e6febcbec0f348cb44a3012/examples/poc/manifests/samples/vllm-lora-deployment.yaml#L18).

1. **Update Envoy Gateway Config to enable Patch Policy**

   Our custom LLM Gateway ext-proc is patched into the existing envoy gateway via `EnvoyPatchPolicy`. To enable this feature, we must extend the Envoy Gateway config map. To do this, simply run:
   ```bash
   kubectl apply -f ./manifests/enable_patch_policy.yaml
   kubectl rollout restart deployment envoy-gateway -n envoy-gateway-system

   ```
   Additionally, if you would like to enable the admin interface, you can uncomment the admin lines and run this again.


1. **Deploy Gateway**

   ```bash
   kubectl apply -f ./manifests/gateway.yaml
   ```

1. **Deploy Ext-Proc**

   ```bash
   kubectl apply -f ./manifests/ext_proc.yaml
   kubectl apply -f ./manifests/patch_policy.yaml
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