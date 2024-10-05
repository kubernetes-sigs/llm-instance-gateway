import argparse
from collections import Counter
import numpy as np
import simpy
from llmactor import LLMActor
from loadbalancer import LoadBalancer

def main():
    parser = argparse.ArgumentParser(description="Simulate LLM load balancing with configurable parameters.")
    parser.add_argument("--rates_lo", nargs='+', type=int, default=[30, ], help="List of low rates.")
    parser.add_argument("--rates_hi", nargs='+', type=int, default=[30,], help="List of high rates.")
    parser.add_argument("--no_of_messages", type=int, default=2500, help="Number of messages to simulate.")
    parser.add_argument("--mean_request_size_1", type=int, default=202, help="Mean request size for set 1.")
    parser.add_argument("--std_request_size_1", type=int, default=20, help="Standard deviation of request size for set 1.")
    parser.add_argument("--mean_output_size_1", type=int, default=179, help="Mean output size for set 1.")
    parser.add_argument("--std_output_size_1", type=int, default=17, help="Standard deviation of output size for set 1.")
    parser.add_argument("--mean_request_size_2", type=int, default=202, help="Mean request size for set 2.")
    parser.add_argument("--std_request_size_2", type=int, default=20, help="Standard deviation of request size for set 2.")
    parser.add_argument("--mean_output_size_2", type=int, default=179, help="Mean output size for set 2.")
    parser.add_argument("--std_output_size_2", type=int, default=17, help="Standard deviation of output size for set 2.")
    parser.add_argument("--queueing_perc", type=float, default=0.19, help="Queueing percentage.")
    parser.add_argument('--target-latency-lo', nargs='+', type=float, help='List of target latencies for low priority requests.')
    parser.add_argument('--target-latency-hi', nargs='+', type=float, help='List of target latencies for high priority requests.')
    
    parser.add_argument('--prefix-latency-lo', nargs='+', type=float, help='List of prefix of target latencies for low priority requests.')
    parser.add_argument('--prefix-latency-hi', nargs='+', type=float, help='List of prefix of target latencies for high priority requests.')
    
    args = parser.parse_args()

     # Use provided arguments or defaults
    rates_lo = args.rates_lo
    rates_hi = args.rates_hi
    no_of_messages = args.no_of_messages
    SIM_DURATIONS = [no_of_messages / r + 100 for r in rates_lo]
    mean_request_size_1 = args.mean_request_size_1
    std_request_size_1 = args.std_request_size_1
    mean_output_size_1 = args.mean_output_size_1
    std_output_size_1 = args.std_output_size_1

    mean_request_size_2 = args.mean_request_size_2
    std_request_size_2 = args.std_request_size_2
    mean_output_size_2 = args.mean_output_size_2
    std_output_size_2 = args.std_output_size_2
    
    queueing_perc = args.queueing_perc
    lora_requested_lo = ""
    lora_requested_hi = ""
    
    target_latency_list_lo = args.target_latency_lo if args.target_latency_lo else [0.025]
    target_latency_list_hi = args.target_latency_hi if args.target_latency_hi else [0.5]
    
    prefix_latency_list_lo = args.prefix_latency_lo if args.prefix_latency_lo else ['lo']
    prefix_latency_list_hi = args.prefix_latency_hi if args.prefix_latency_hi else ['hi']

    # Define a structure to store results for all routing types
    results = {
        'leastPseudo': {'latency': [], 'latency_lo': [], 'latency_hi': [],
               'throughput_prefill': [], 'throughput_decode': [],
               'throughput_prefill_lo': [], 'throughput_decode_lo': [],
               'throughput_prefill_hi': [], 'throughput_decode_hi': [],
               'ttft': [], 'ttft_lo': [], 'ttft_hi': [],
               'tpot': [], 'tpot_lo': [], 'tpot_hi': [],
               'target_pods_lo': [], 'target_pods_hi': [],
               'recompute_cnt' : [], 'recompute_cnt_hi' : [], 'recompute_cnt_lo' : [],
               'pct_below_latency_target_lo': [], 'pct_below_latency_target_hi': [],  'queue_time_lo': [], 'queue_time_hi': [],
              'tol_lat_time_lo': [], 'tol_lat_time_hi': []},

    'smart': {'latency': [], 'latency_lo': [], 'latency_hi': [],
              'estimated_latency': [], 'estimated_latency_lo': [], 'estimated_latency_hi': [],
              'throughput_prefill': [], 'throughput_decode': [],
              'throughput_prefill_lo': [], 'throughput_decode_lo': [],
              'throughput_prefill_hi': [], 'throughput_decode_hi': [],
              'ttft': [], 'ttft_lo': [], 'ttft_hi': [],
              'tpot': [], 'tpot_lo': [], 'tpot_hi': [],
              'target_pods_lo': [], 'target_pods_hi': [],
              'recompute_cnt' : [], 'recompute_cnt_hi' : [], 'recompute_cnt_lo' : [],
              'pct_below_latency_target_lo': [], 'pct_below_latency_target_hi': [],
               'pct_below_latency_target_lo': [], 'pct_below_latency_target_hi': [], 'queue_time_lo': [], 'queue_time_hi': [],
              'tol_lat_time_lo': [], 'tol_lat_time_hi': []},

    'leastlatency': {'latency': [], 'latency_lo': [], 'latency_hi': [],
                'throughput_prefill': [], 'throughput_decode': [],
                'throughput_prefill_lo': [], 'throughput_decode_lo': [],
                'throughput_prefill_hi': [], 'throughput_decode_hi': [],
                'ttft': [], 'ttft_lo': [], 'ttft_hi': [],
                'tpot': [], 'tpot_lo': [], 'tpot_hi': [],
                'target_pods_lo': [], 'target_pods_hi': [],
                'recompute_cnt' : [], 'recompute_cnt_hi' : [], 'recompute_cnt_lo' : [],
                'pct_below_latency_target_lo': [], 'pct_below_latency_target_hi': [], 'queue_time_lo': [], 'queue_time_hi': [],
              'tol_lat_time_lo': [], 'tol_lat_time_hi': []},
    'least': {'latency': [], 'latency_lo': [], 'latency_hi': [],
                'throughput_prefill': [], 'throughput_decode': [],
                'throughput_prefill_lo': [], 'throughput_decode_lo': [],
                'throughput_prefill_hi': [], 'throughput_decode_hi': [],
                'ttft': [], 'ttft_lo': [], 'ttft_hi': [],
                'tpot': [], 'tpot_lo': [], 'tpot_hi': [],
                'target_pods_lo': [], 'target_pods_hi': [],
                'recompute_cnt' : [], 'recompute_cnt_hi' : [], 'recompute_cnt_lo' : [],
               'pct_below_latency_target_lo': [], 'pct_below_latency_target_hi': [], 'queue_time_lo': [], 'queue_time_hi': [],
              'tol_lat_time_lo': [], 'tol_lat_time_hi': []},
    'random': {'latency': [], 'latency_lo': [], 'latency_hi': [],
                'throughput_prefill': [], 'throughput_decode': [],
                'throughput_prefill_lo': [], 'throughput_decode_lo': [],
                'throughput_prefill_hi': [], 'throughput_decode_hi': [],
                'ttft': [], 'ttft_lo': [], 'ttft_hi': [],
                'tpot': [], 'tpot_lo': [], 'tpot_hi': [],
                'target_pods_lo': [], 'target_pods_hi': [],
                'recompute_cnt' : [], 'recompute_cnt_hi' : [], 'recompute_cnt_lo' : [],
               'pct_below_latency_target_lo': [], 'pct_below_latency_target_hi': [], 'queue_time_lo': [], 'queue_time_hi': [],
              'tol_lat_time_lo': [], 'tol_lat_time_hi': []},
}

    all_routing_types = ["least", ]
    prompt_output_tuple = None

# Iterate over routing types
    for routing_type in all_routing_types:
        print(f'Routing Type: {routing_type}')

        for i, _ in enumerate(rates_lo):
            req_dict = {}
            req_dict_prefill = {}
            SIM_DURATION = SIM_DURATIONS[i]
            print(f'Simulate with rate: for lo {rates_lo[i]} and for hi {rates_hi[i]} and routing type: {routing_type}')

            # Simpy environment and LLM actors setup
            env = simpy.Environment()
            number_of_servers =6
            list_of_llmactors = [LLMActor(env, 1, id) for id in range(number_of_servers)]
            lb = LoadBalancer(env, number_of_servers=number_of_servers, list_of_llmactors=list_of_llmactors, req_dict_prefill=req_dict_prefill, req_dict=req_dict, messages_remaining_cnt=no_of_messages*2)
            lb.queueing_perc = queueing_perc

            estimated_output_size = mean_output_size_1
            lb.process(rates_lo[i], lora_requested_lo, target_latency_list_lo, prefix_latency_list_lo, routing_type, prompt_output_tuple, mean_request_size_1, std_request_size_1, mean_output_size_1, std_output_size_1, estimated_output_size)
            lb.process(rates_hi[i], lora_requested_hi, target_latency_list_hi, prefix_latency_list_hi, routing_type, prompt_output_tuple, mean_request_size_2, std_request_size_2, mean_output_size_2, std_output_size_2, estimated_output_size)
            env.run(until=SIM_DURATION)

            # Track which pod processed each request (lo and hi)
            target_pods_lo = []
            target_pods_hi = []

            for req in req_dict.values():
                if "lo:" in req.id:
                    target_pods_lo.append(req.target_pod)
                elif "hi:" in req.id:
                    target_pods_hi.append(req.target_pod)

            # Completed requests
            completed_req = list(filter(lambda x: x.output_size_remaining == 0, req_dict.values()))
            completed_req_lo = list(filter(lambda x: x.output_size_remaining == 0 and ("lo:" in x.id), req_dict.values()))
            completed_req_hi = list(filter(lambda x: x.output_size_remaining == 0 and ("hi:" in x.id), req_dict.values()))

            completed_req_sorted = sorted(completed_req, key=lambda x: x.arrival_time)
            completed_req_lo_sorted = sorted(completed_req_lo, key=lambda x: x.arrival_time)
            completed_req_hi_sorted = sorted(completed_req_hi, key=lambda x: x.arrival_time)

            # Exclude the first 10% of requests based on end_decode_time
            exclude_count = int(0 * len(completed_req_sorted))
            exclude_count_lo = int(0 * len(completed_req_lo_sorted))
            exclude_count_hi = int(0 * len(completed_req_hi_sorted))

            # Filter out the first 10%
            filtered_req = completed_req_sorted[exclude_count:]
            filtered_req_lo = completed_req_lo_sorted[exclude_count:]
            filtered_req_hi = completed_req_hi_sorted[exclude_count:]


            # Calculate ttft, tpot, latency, and throughput
            ttft_cur = np.mean([x.end_prefill_time - x.arrival_time for x in req_dict.values()])
            ttft_cur_lo = np.mean([x.end_decode_time - x.arrival_time for x in filtered_req_lo])
            ttft_cur_hi = np.mean([x.end_decode_time - x.arrival_time for x in filtered_req_hi])

            tpot_cur = np.mean([(x.end_decode_time - x.start_prefill_time) / (x.output_size - x.output_size_remaining) for x in req_dict.values()])
            tpot_cur_lo = np.mean([(x.end_decode_time - x.start_prefill_time) / (x.output_size - x.output_size_remaining) for x in filtered_req_lo])
            tpot_cur_hi = np.mean([(x.end_decode_time - x.start_prefill_time) / (x.output_size - x.output_size_remaining) for x in filtered_req_hi])

            latency_cur = np.mean([(x.end_decode_time - x.arrival_time) / (x.output_size - x.output_size_remaining) for x in filtered_req])
            latency_cur_lo = np.mean([(x.end_decode_time - x.arrival_time) / (x.output_size - x.output_size_remaining) for x in filtered_req_lo])
            latency_cur_hi = np.mean([(x.end_decode_time - x.arrival_time) / (x.output_size - x.output_size_remaining) for x in filtered_req_hi])

            estimated_latency_cur = np.mean([x.estimated_latency for x in filtered_req])
            estimated_latency_cur_lo = np.mean([x.estimated_latency for x in filtered_req_lo])
            estimated_latency_cur_hi = np.mean([x.estimated_latency for x in filtered_req_hi])

            recompute_cur = np.sum([x.recompute_count for x in filtered_req]) / len(filtered_req)
            recompute_cur_lo = np.sum([x.recompute_count for x in filtered_req_lo]) / len(filtered_req_lo)
            recompute_cur_hi = np.sum([x.recompute_count for x in filtered_req_hi]) / len(filtered_req_hi)

            tt = SIM_DURATION
            throughput_prefill_cur = np.sum([x.input_size for x in filtered_req]) / tt
            throughput_decode_cur = np.sum([max(0, x.output_size - x.output_size_remaining - 1) for x in filtered_req]) / tt
            throughput_prefill_lo_cur = np.sum([x.input_size if ("lo:" in x.id) else 0 for x in filtered_req_lo]) / tt
            throughput_decode_lo_cur = np.sum([max(0, x.output_size - x.output_size_remaining - 1) if ("lo:" in x.id) else 0 for x in filtered_req_lo]) / tt
            throughput_prefill_hi_cur = np.sum([x.input_size if ("hi:" in x.id) else 0 for x in filtered_req_hi]) / tt
            throughput_decode_hi_cur = np.sum([max(0, x.output_size - x.output_size_remaining - 1) if ("hi:" in x.id) else 0 for x in filtered_req_hi]) / tt

            # Calculate % of requests below latency target
            latencies_lo = [(x.end_decode_time - x.arrival_time) / (x.output_size - x.output_size_remaining) for x in filtered_req_lo]
            latencies_hi = [(x.end_decode_time - x.arrival_time) / (x.output_size - x.output_size_remaining) for x in filtered_req_hi]
            pct_below_target_lo = (np.sum([1 if  x < target_latency_list_lo[0] else 0 for x in latencies_lo]) / len(latencies_lo)) * 100
            pct_below_target_hi = (np.sum([1 if  x < target_latency_list_hi[0] else 0 for x in latencies_hi]) / len(latencies_hi)) * 100


            queue_time_cur_lo = np.mean([(x.start_prefill_time - x.arrival_time)  for x in filtered_req_lo])
            queue_time_cur_hi = np.mean([(x.start_prefill_time - x.arrival_time) for x in filtered_req_hi])

            tol_lat_time_cur_lo = np.mean([(x.end_decode_time - x.arrival_time)  for x in filtered_req_lo])
            tol_lat_time_cur_hi = np.mean([(x.end_decode_time - x.arrival_time) for x in filtered_req_hi])

            # Store results for the current routing type
            results[routing_type]['latency'].append(latency_cur)
            results[routing_type]['latency_lo'].append(latency_cur_lo)
            results[routing_type]['latency_hi'].append(latency_cur_hi)
            results[routing_type]['throughput_prefill'].append(throughput_prefill_cur)
            results[routing_type]['throughput_decode'].append(throughput_decode_cur)
            results[routing_type]['throughput_prefill_lo'].append(throughput_prefill_lo_cur)
            results[routing_type]['throughput_decode_lo'].append(throughput_decode_lo_cur)
            results[routing_type]['throughput_prefill_hi'].append(throughput_prefill_hi_cur)
            results[routing_type]['throughput_decode_hi'].append(throughput_decode_hi_cur)
            results[routing_type]['ttft'].append(ttft_cur)
            results[routing_type]['ttft_lo'].append(ttft_cur_lo)
            results[routing_type]['ttft_hi'].append(ttft_cur_hi)
            results[routing_type]['tpot'].append(tpot_cur)
            results[routing_type]['tpot_lo'].append(tpot_cur_lo)
            results[routing_type]['tpot_hi'].append(tpot_cur_hi)

            results[routing_type]['recompute_cnt'].append(recompute_cur)
            results[routing_type]['recompute_cnt_lo'].append(recompute_cur_lo)
            results[routing_type]['recompute_cnt_hi'].append(recompute_cur_hi)

            results[routing_type]['pct_below_latency_target_lo'].append(pct_below_target_lo)
            results[routing_type]['pct_below_latency_target_hi'].append(pct_below_target_hi)

            # Store pod distribution results
            results[routing_type]['target_pods_lo'].append(Counter(target_pods_lo))
            results[routing_type]['target_pods_hi'].append(Counter(target_pods_hi))

            results[routing_type]['queue_time_lo'].append(queue_time_cur_lo)
            results[routing_type]['queue_time_hi'].append(queue_time_cur_hi)

            results[routing_type]['tol_lat_time_lo'].append(tol_lat_time_cur_lo)
            results[routing_type]['tol_lat_time_hi'].append(tol_lat_time_cur_hi)

            l1 = [np.sum(list(dict(x).values())) for x in results[routing_type]['target_pods_lo']]
            l2 = [np.sum(list(dict(x).values())) for x in results[routing_type]['target_pods_hi']]

            print(f'req count {[(l1[i], l2[i]) for i in range(len(l1))]}')

            if routing_type == 'smart':
                results[routing_type]['estimated_latency'].append(estimated_latency_cur)
                results[routing_type]['estimated_latency_lo'].append(estimated_latency_cur_lo)
                results[routing_type]['estimated_latency_hi'].append(estimated_latency_cur_hi)
                print(f"lo dist {Counter(target_pods_lo)} latency {latency_cur_lo} estimated_latency_lo {estimated_latency_cur_lo}")
                print(f"hi dist {Counter(target_pods_hi)}  latency {latency_cur_hi} estimated_latency_hi {estimated_latency_cur_hi}")
            else:
                print(f"lo dist {Counter(target_pods_lo)} latency {latency_cur_lo} ")
                print(f"hi dist {Counter(target_pods_hi)}  latency {latency_cur_hi} ")

            # Print the results for this qps
            print(f'QPS: {rates_lo[i]} (lo), {rates_hi[i]} (hi)')
            print(f'% of lo requests below target: {pct_below_target_lo}%')
            print(f'% of hi requests below target: {pct_below_target_hi}%')

if __name__ == "__main__":
        main()