# Kubernetes LLM Instance Gateway

The LLM Instance Gateway is a part of [wg-serving](https://github.com/kubernetes/community/tree/master/wg-serving), and this repo contains: the load balancing algorithm, [ext-proc](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/ext_proc_filter) code, CRDs, and controllers to support the LLM Instance Gateway.

This Gateway is intented to provide value to multiplexed LLM services on a shared pool of compute. See the [proposal](https://github.com/kubernetes-sigs/wg-serving/tree/main/proposals/012-llm-instance-gateway) for more info.

## Status

This project is currently in development. 

For more rapid testing, our PoC is in the `./examples/` dir.


## Getting Started

**Install the CRDs into the cluster:**

```sh
make install
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**Deploying the ext-proc image**
Refer to this [README](https://github.com/kubernetes-sigs/llm-instance-gateway/blob/main/pkg/README.md) on how to deploy the Ext-Proc image used to support Instance Gateway.

## Contributing

Our community meeting is weekly at Th 10AM PDT; [zoom link here](https://zoom.us/j/9955436256?pwd=Z2FQWU1jeDZkVC9RRTN4TlZyZTBHZz09).

We currently utilize the [#wg-serving](https://kubernetes.slack.com/?redir=%2Fmessages%2Fwg-serving) slack channel for communications.

Contributions are readily welcomed, thanks for joining us!

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).
