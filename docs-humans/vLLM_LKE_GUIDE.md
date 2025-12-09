# vLLM Deployment Guide for Linode Kubernetes Engine (LKE)

This guide provides a comprehensive walkthrough for deploying vLLM models on a Linode Kubernetes Engine (LKE) cluster.

## Step 1: Add a GPU Node Pool to your LKE Cluster

1.  **Navigate to your LKE Cluster in Linode Cloud Console.**
    - Go to: [https://cloud.linode.com/kubernetes/clusters/](https://cloud.linode.com/kubernetes/clusters/) and select your cluster.

2.  **Add a New Node Pool.**
    - Click on **"Add a Node Pool"**.
    - Select the **"Dedicated GPU"** tab.
    - Choose a GPU type. The **RTX 6000 (24GB)** is a good choice for models up to 13B parameters.
    - Set the number of nodes to **1** to start.
    - Click **"Add Pool"**.

3.  **Wait for the node to be provisioned.**
    - This usually takes 3-5 minutes. You can monitor the progress in the Linode console or by using `kubectl`.
    - Configure your local environment to connect to the cluster by setting the `KUBECONFIG` environment variable.
    ```bash
    export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
    watch kubectl get nodes
    ```
    - A new node should appear in the list.

## Step 2: Configure the GPU Node

1.  **Identify the new GPU node.**
    - The new node will be the one with the most recent `AGE`.
    ```bash
    GPU_NODE=$(kubectl get nodes --sort-by=.metadata.creationTimestamp | tail -1 | awk '{print $1}')
    echo "Found GPU Node: $GPU_NODE"
    ```

2.  **Label the GPU node.**
    - This label will be used to ensure that vLLM pods are scheduled on the correct node.
    ```bash
    kubectl label nodes $GPU_NODE node-type=gpu
    ```
    - Verify the label was applied:
    ```bash
    kubectl get nodes -l node-type=gpu
    ```

3.  **Taint the GPU node.**
    - This prevents non-GPU workloads from running on this expensive node.
    ```bash
    kubectl taint nodes $GPU_NODE gpu-workload=true:NoSchedule
    ```
    - Verify the taint was applied:
    ```bash
    kubectl describe node $GPU_NODE | grep Taints
    ```

## Step 3: Install the NVIDIA Device Plugin

The NVIDIA device plugin exposes the GPU to the Kubernetes scheduler.

1.  **Install the plugin.**
    ```bash
    kubectl apply -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.14.0/nvidia-device-plugin.yml
    ```

2.  **Verify the GPU is available.**
    - Wait about 30 seconds for the plugin to start.
    ```bash
    kubectl describe node $GPU_NODE | grep nvidia.com/gpu
    ```
    - You should see `nvidia.com/gpu: 1` in the `Allocatable` resources section.

## Step 4: Deploy a vLLM Model

This project uses a script to simplify the deployment of vLLM models.

1.  **Choose a model to deploy.**
    - For a 24GB GPU like the RTX 6000, you can run models like `meta-llama/Llama-2-7b-chat-hf` or `mistralai/Mistral-7B-Instruct-v0.2`.

2.  **Run the deployment script.**
    - From the root of the `ai-aas` project:
    ```bash
    # Deploy Llama-2-7B
    ./scripts/deploy-vllm-linode.sh meta-llama/Llama-2-7b-chat-hf

    # Or deploy Mistral-7B
    ./scripts/deploy-vllm-linode.sh mistralai/Mistral-7B-Instruct-v0.2
    ```
    - The script will:
        - Verify the GPU node and NVIDIA plugin are ready.
        - Deploy vLLM using a Helm chart.
        - Wait for the model to be downloaded from HuggingFace (this can take 5-15 minutes).
        - Test the health and inference endpoints.

3.  **Monitor the deployment.**
    - The script will monitor the deployment, but you can also check the status manually in another terminal:
    ```bash
    # Watch the pods in the 'system' namespace
    kubectl get pods -n system -w

    # View the logs of the vLLM pod
    kubectl logs -n system -l app.kubernetes.io/name=vllm-deployment -f
    ```

## Step 5: Test the Deployment

1.  **Get the service name.**
    - The deployment script will create a Kubernetes service for the model.
    ```bash
    kubectl get svc -n system
    ```
    - The service name will be based on the model name, e.g., `llama-2-7b-chat-hf`.

2.  **Run the end-to-end test script.**
    - This script will automatically port-forward to the service and send a test inference request.
    ```bash
    # Replace <service-name> with the name from the previous step
    ./test-api-inference.sh cluster system <service-name> meta-llama/Llama-2-7b-chat-hf
    ```
    - You should see a success message indicating that the test passed.

3.  **Manual testing with `curl`.**
    - You can also test the model manually using `curl`.
    ```bash
    # Port-forward to the service in the background
    kubectl port-forward -n system svc/<service-name> 8000:8000 &

    # Send a request
    curl -X POST http://localhost:8000/v1/chat/completions \
      -H "Content-Type: application/json" \
      -d 
      {
        "model": "meta-llama/Llama-2-7b-chat-hf",
        "messages": [
          {"role": "user", "content": "What is the capital of France?"}
        ],
        "max_tokens": 10
      }
    ```

## Step 6: Clean Up

To avoid unnecessary costs, you can scale down or remove the GPU node pool when it's not in use.

1.  **Delete the model deployment.**
    ```bash
    helm uninstall <release-name> -n system
    ```
    - You can find the release name with `helm list -n system`.

2.  **Remove the GPU Node Pool.**
    - In the Linode Cloud Console, navigate to your LKE cluster's page.
    - Find the GPU node pool and click the trash can icon to delete it.
