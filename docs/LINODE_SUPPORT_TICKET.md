# Linode Support Ticket: GPU Node NVML Issue

**Date**: 2025-11-19
**Cluster ID**: lke531921
**Region**: fr-par-2
**GPU Node ID**: lke531921-776619-2c1affb80000
**GPU**: NVIDIA RTX 4000 Ada

## Issue Summary

GPU nodes in Linode LKE are not properly configured for Kubernetes GPU workloads. The NVIDIA Management Library (NVML) is not functional, preventing GPU discovery by the NVIDIA device plugin and GPU operator.

## Expected Behavior

According to the [Akamai/Linode documentation](https://techdocs.akamai.com/cloud-computing/docs/gpus-on-lke), GPU nodes should:

1. Have NVIDIA drivers automatically installed
2. Have NVIDIA Container Toolkit configured
3. Work with NVIDIA Kubernetes device plugin
4. Expose `nvidia.com/gpu` resources to Kubernetes

## Actual Behavior

- GPU hardware is detected (`lspci` shows RTX 4000 Ada)
- NVIDIA drivers are installed
- Device files exist (`/dev/nvidia0`, `/dev/nvidiactl`, etc.)
- Kernel modules are loaded (`nvidia`, `nvidia_uvm`, `nvidia_drm`)
- **BUT**: `nvidia-smi` fails with "Failed to initialize NVML: Unknown Error"
- **RESULT**: NVIDIA device plugin cannot detect GPU
- **IMPACT**: Cannot deploy GPU workloads on LKE

## Steps We've Taken

### 1. Followed Official Linode Documentation

```bash
# Installed GPU operator with Linode-specific flags
helm install --wait --generate-name \
  -n gpu-operator --create-namespace \
  nvidia/gpu-operator \
  --set driver.enabled=false \
  --set toolkit.enabled=false

# Installed NVIDIA device plugin v0.17.3
kubectl create -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.17.3/deployments/static/nvidia-device-plugin.yml
```

### 2. Manual Configuration of NVIDIA Runtime

```bash
# Configured containerd to use NVIDIA runtime
nvidia-ctk runtime configure --runtime=containerd --set-as-default

# Restarted containerd
systemctl restart containerd
```

### 3. Verified GPU Hardware

```bash
root@lke531921-776619-2c1affb80000:/# lspci | grep NVIDIA
03:00.0 VGA compatible controller: NVIDIA Corporation AD104GL [RTX 4000 Ada Generation] (rev a1)
03:00.1 Audio device: NVIDIA Corporation Device 22be (rev a1)

root@lke531921-776619-2c1affb80000:/# ls -la /dev/nvidia*
crw-rw-rw- 1 root root 195,   0 Nov 19 09:18 /dev/nvidia0
crw-rw-rw- 1 root root 195, 255 Nov 19 09:18 /dev/nvidiactl
crw-rw-rw- 1 root root 195, 254 Nov 19 09:18 /dev/nvidia-uvm
crw-rw-rw- 1 root root 237,   0 Nov 19 09:18 /dev/nvidia-uvm-tools

root@lke531921-776619-2c1affb80000:/# lsmod | grep nvidia
nvidia_uvm           5382144  0
nvidia_drm            114688  0
nvidia_modeset       1654784  1 nvidia_drm
nvidia              62894080  2 nvidia_uvm,nvidia_modeset
drm_kms_helper        311296  1 nvidia_drm
drm                   753664  4 drm_kms_helper,nvidia,nvidia_drm
```

### 4. Confirmed NVML Failure

```bash
root@lke531921-776619-2c1affb80000:/# nvidia-smi
Failed to initialize NVML: Unknown Error
```

## NVIDIA Device Plugin Logs

```
E1119 17:44:43.345466       1 factory.go:112] Incompatible strategy detected auto
E1119 17:44:43.345479       1 factory.go:113] If this is a GPU node, did you configure the NVIDIA Container Toolkit?
I1119 17:44:43.345499       1 main.go:381] No devices found. Waiting indefinitely.
```

## Root Cause

The automatic driver installation process mentioned in the Linode documentation appears to be incomplete or failing:

**From Linode Docs** (https://techdocs.akamai.com/cloud-computing/docs/gpus-on-lke):
```bash
wget -q https://developer.download.nvidia.com/compute/cuda/repos/debian12/x86_64/cuda-keyring_1.1-1_all.deb
dpkg -i cuda-keyring_1.1-1_all.deb
apt update
apt install -y nvidia-driver-cuda linux-headers-cloud-amd64 nvidia-container-toolkit nvidia-kernel-open-dkms
nvidia-ctk runtime configure --runtime=containerd --set-as-default
```

While the packages appear to be installed, **NVML is not functional**, suggesting either:

1. The driver installation is incomplete
2. There's a mismatch between kernel version and driver version
3. The NVML libraries are missing or misconfigured
4. The GPU node provisioning process has a bug

## Impact

- **Cannot deploy GPU workloads** on Linode LKE
- **Cannot use vLLM or other ML inference tools** that require GPU
- **Kubernetes cannot schedule GPU pods** (no `nvidia.com/gpu` resource)

## Request

Please investigate why NVML is not functional on LKE GPU nodes and ensure that:

1. **nvidia-smi works** without errors
2. **NVIDIA device plugin can detect GPUs**
3. **Kubernetes exposes `nvidia.com/gpu` as allocatable resource**

This appears to be an issue with the automated GPU node provisioning process in Linode LKE.

## Additional Context

### Kubernetes Configuration

- **Cluster Version**: LKE (Kubernetes 1.x)
- **Node Count**: 4 (3 standard + 1 GPU)
- **GPU Node Labels**: `node-type=gpu`
- **GPU Node Taints**: `gpu-workload=true:NoSchedule`

### Containerd Configuration

After manual configuration, containerd config includes:

```toml
# /etc/containerd/config.d/99-nvidia.toml
[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "nvidia"
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia]
          privileged_without_host_devices = false
          runtime_engine = ""
          runtime_root = ""
          runtime_type = "io.containerd.runc.v2"
          [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia.options]
            BinaryName = "/usr/bin/nvidia-container-runtime"
```

### Test Command to Verify Fix

Once fixed, this command should show `nvidia.com/gpu: 1`:

```bash
kubectl get nodes lke531921-776619-2c1affb80000 -o json | jq '.status.allocatable'
```

## Contact

Please update this ticket with:
1. Root cause of the NVML issue
2. Whether this is a known issue
3. Timeline for resolution
4. Any workarounds available

Thank you!
