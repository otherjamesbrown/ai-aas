# GPU Hardware Missing on Linode LKE Nodes - Debugging Summary

**Date**: 2025-11-19
**Status**: 🔴 CRITICAL - GPU hardware not present on instances
**Impact**: Cannot deploy GPU workloads

## Issue Summary

GPU node pool instances in Linode LKE are not provisioned with GPU hardware, despite Linode Cloud Manager showing the correct instance type. The instances boot successfully and join the Kubernetes cluster, but `lspci` shows only virtual devices with no NVIDIA GPU present.

## Cluster Information

- **Cluster ID**: lke531921
- **Region**: fr-par-2
- **Kubernetes Version**: v1.34.0
- **Total Nodes**: 4 (3 standard + 1 GPU)

## Affected GPU Nodes

### Node Pool 776619 (Deleted)
- **Node**: lke531921-776619-2c1affb80000
- **External IP**: 172.232.49.84
- **Issue**: GPU hardware detected, but NVML non-functional
- **Symptoms**:
  - `lspci` showed RTX 4000 Ada
  - `nvidia-smi` failed with "Failed to initialize NVML: Unknown Error"
  - Device files present: `/dev/nvidia0`, `/dev/nvidiactl`, `/dev/nvidia-uvm`
  - Kernel modules loaded: `nvidia`, `nvidia_uvm`, `nvidia_drm`

### Node Pool 776662 (Current)
- **Node**: lke531921-776662-3bbc7ff10000
- **External IP**: 172.232.49.84
- **Instance Type (per Cloud Manager)**: g2-gpu-rtx4000a1-m
- **Issue**: NO GPU hardware present at all
- **Symptoms**:
  - `lspci` shows only virtual devices
  - No NVIDIA PCI devices (vendor ID 10de)
  - No `/dev/nvidia*` device files
  - No NVIDIA kernel modules
  - `nvidia-smi` not installed

## Expected vs Actual

### Expected Behavior (per Linode Docs)
According to [Linode GPU on LKE documentation](https://techdocs.akamai.com/cloud-computing/docs/gpus-on-lke):
1. GPU nodes automatically provisioned with NVIDIA GPU hardware
2. NVIDIA drivers automatically installed
3. GPU visible via `lspci`
4. `nvidia-smi` functional
5. NVIDIA Container Toolkit configured

### Actual Behavior

**Node Pool 776662 - Current State:**
```bash
# lspci output (via kubectl debug)
00:00.0 Host bridge: Intel Corporation 82G33/G31/P35/P31 Express DRAM Controller
00:01.0 VGA compatible controller: Device 1234:1111 (rev 02)  # Virtual/QEMU display
00:02.0 SCSI storage controller: Red Hat, Inc. Virtio SCSI
00:03.0 Ethernet controller: Red Hat, Inc. Virtio network device
00:1f.0 ISA bridge: Intel Corporation 82801IB (ICH9) LPC Interface Controller (rev 02)
00:1f.2 SATA controller: Intel Corporation 82801IR/IO/IH (ICH9R/DO/DH) 6 port SATA Controller [AHCI mode] (rev 02)
00:1f.3 SMBus: Intel Corporation 82801I (ICH9 Family) SMBus Controller (rev 02)

# NO NVIDIA GPU - No PCI vendor 10de present
```

**Node labels from Node Feature Discovery:**
```bash
# PCI-related labels (no nvidia.com/gpu labels)
feature.node.kubernetes.io/pci-1234.present=true  # QEMU/VirtIO
feature.node.kubernetes.io/pci-1af4.present=true  # VirtIO/RedHat

# NVIDIA PCI vendor ID (10de) NOT present
```

## Diagnostic Steps Taken

### 1. Verified Kubernetes Node
```bash
kubectl get nodes lke531921-776662-3bbc7ff10000
# Result: Node Ready, joined cluster successfully
```

### 2. Checked GPU Hardware via lspci
```bash
kubectl debug node/lke531921-776662-3bbc7ff10000 -it --image=ubuntu -- \
  bash -c "chroot /host lspci | grep -i nvidia"
# Result: No output - no NVIDIA devices found
```

### 3. Checked All PCI Devices
```bash
kubectl debug node/lke531921-776662-3bbc7ff10000 -it --image=ubuntu -- \
  bash -c "chroot /host lspci"
# Result: Only virtual devices (VirtIO, QEMU), no physical GPU
```

### 4. Checked for nvidia-smi
```bash
kubectl debug node/lke531921-776662-3bbc7ff10000 -it --image=ubuntu -- \
  bash -c "chroot /host nvidia-smi"
# Result: nvidia-smi: No such file or directory
```

### 5. Attempted GPU Operator Installation
- **Approach 1**: With `driver.enabled=false, toolkit.enabled=false` (per Linode docs)
  - Result: No driver daemonset deployed (expected, as drivers should be pre-installed)
  - Issue: NVML not working on old node

- **Approach 2**: With default settings (driver management enabled)
  - Result: NFD didn't detect GPU, no driver daemonset deployed
  - Issue: Node has no GPU hardware to detect

### 6. Restarted Instance
- Set root password for SSH access
- Rebooted instance via Linode Cloud Manager
- Re-checked with lspci after reboot
- **Result**: Same - no GPU hardware present

## Evidence

### lspci Output (Complete)
```
00:00.0 Host bridge: Intel Corporation 82G33/G31/P35/P31 Express DRAM Controller
00:01.0 VGA compatible controller: Device 1234:1111 (rev 02)
00:02.0 SCSI storage controller: Red Hat, Inc. Virtio SCSI
00:03.0 Ethernet controller: Red Hat, Inc. Virtio network device
00:1f.0 ISA bridge: Intel Corporation 82801IB (ICH9) LPC Interface Controller (rev 02)
00:1f.2 SATA controller: Intel Corporation 82801IR/IO/IH (ICH9R/DO/DH) 6 port SATA Controller [AHCI mode] (rev 02)
00:1f.3 SMBus: Intel Corporation 82801I (ICH9 Family) SMBus Controller (rev 02)
```

### Node Labels (GPU-related only)
```bash
# Expected (for a GPU node):
# nvidia.com/gpu.present=true
# feature.node.kubernetes.io/pci-10de.present=true

# Actual:
# No NVIDIA-related labels
# No PCI vendor 10de labels
```

### Kubernetes Node Status
```
NAME                            STATUS   ROLES    AGE   VERSION   KERNEL-VERSION
lke531921-776662-3bbc7ff10000   Ready    <none>   ~1h   v1.34.0   6.1.0-41-cloud-amd64
```

### Additional Diagnostic Evidence (Collected 2025-11-19)

#### Kernel Boot Parameters
```bash
BOOT_IMAGE=/boot/vmlinuz-6.1.0-41-cloud-amd64 root=UUID=aa1b96cb-f787-027c-42d3-f1d0efdd019f ro linode_kube_1.34.0 console=ttyS0,19200n8 net.ifnames=0
```
**Finding**: No GPU passthrough parameters (no `intel_iommu=on`, `amd_iommu=on`, or `vfio-pci` configuration)

#### Detailed PCI Information
```bash
# All PCI devices report as "Red Hat, Inc. QEMU Virtual Machine"
00:00.0 Host bridge: Intel Corporation 82G33/G31/P35/P31 Express DRAM Controller
	Subsystem: Red Hat, Inc. QEMU Virtual Machine

00:01.0 VGA compatible controller: Device 1234:1111 (rev 02)
	Subsystem: Red Hat, Inc. Device 1100
	Memory at fd000000 (32-bit, prefetchable)

# All other devices: VirtIO (virtual I/O devices)
00:02.0 SCSI storage controller: Red Hat, Inc. Virtio SCSI
00:03.0 Ethernet controller: Red Hat, Inc. Virtio network device
```
**Finding**: Instance is running in QEMU/KVM with NO PCI passthrough configured

#### NVIDIA Software Status
```bash
# No NVIDIA firmware
ls /lib/firmware/nvidia/
# Result: No such file or directory

# No NVIDIA packages
dpkg -l | grep nvidia
# Result: No NVIDIA packages installed

# No NVIDIA kernel module
modprobe -n nvidia
# Result: FATAL: Module nvidia not found in directory /lib/modules/6.1.0-41-cloud-amd64
```
**Finding**: Completely clean system with NO NVIDIA software installed (contradicts Linode documentation)

#### Linode Metadata Service
```bash
curl http://169.254.169.254/v1/instance/type
# Result: errors.reason: Not found

curl http://169.254.169.254/v1/instance
# Result: errors.reason: The Metadata-Token header is required for this request
```
**Finding**: Instance type metadata endpoint returns "Not found" (suspicious)

## Key Observations

1. **Linode Cloud Manager shows correct instance type**: g2-gpu-rtx4000a1-m
2. **Instance boots and joins cluster successfully**
3. **No GPU hardware present in the OS**: `lspci` shows only virtual devices
4. **Instance running as standard QEMU/KVM VM**: All PCI devices show "Red Hat, Inc. QEMU Virtual Machine"
5. **No GPU passthrough configured**: Kernel has no IOMMU/passthrough boot parameters
6. **No NVIDIA software installed**: Despite Linode docs saying drivers are pre-installed
7. **Metadata service anomaly**: Instance type endpoint returns "Not found"
8. **Pattern across multiple attempts**:
   - Pool 776619: GPU hardware present but NVML broken
   - Pool 776662: No GPU hardware at all
9. **Region-specific issue**: Both pools in fr-par-2

## Comparison: Working vs Non-Working

### What Old Node (776619) Had
- ✅ NVIDIA GPU detected via `lspci` (RTX 4000 Ada)
- ✅ NVIDIA device files (`/dev/nvidia*`)
- ✅ NVIDIA kernel modules loaded
- ❌ NVML non-functional ("Unknown Error")

### What Current Node (776662) Has
- ❌ No NVIDIA GPU in `lspci`
- ❌ No NVIDIA device files
- ❌ No NVIDIA kernel modules
- ❌ No drivers installed
- ❌ Only virtual VGA controller

## Hypotheses (Updated with Additional Evidence)

### 1. Instance Type Mismatch / Provisioning Bug ⭐ MOST LIKELY
- **Theory**: Cloud Manager shows g2-gpu-rtx4000a1-m, but hypervisor provisioned standard (non-GPU) instance
- **Evidence**:
  - All PCI devices show "Red Hat, Inc. QEMU Virtual Machine" (standard VM)
  - No IOMMU/passthrough boot parameters
  - Metadata endpoint `/v1/instance/type` returns "Not found"
  - No NVIDIA software installed (contradicts Linode docs about automatic driver installation)
  - Only virtual VGA controller (1234:1111 - QEMU default)
- **Likelihood**: **VERY HIGH**
- **Conclusion**: Instance was likely provisioned as a standard VM, not a GPU instance

### 2. GPU Passthrough Not Configured
- **Theory**: Instance is GPU-type but hypervisor didn't configure PCI passthrough
- **Evidence**:
  - Kernel lacks IOMMU boot parameters needed for GPU passthrough
  - No physical PCI devices, only virtual
- **Likelihood**: HIGH (related to #1)
- **Note**: This could be the root cause of #1

### 3. GPU Pool Provisioning Issue in fr-par-2
- **Theory**: GPU node pools in fr-par-2 region have systematic provisioning failures
- **Evidence**:
  - Two different pools, both problematic (though different issues)
  - Pool 776619: GPU present but NVML broken
  - Pool 776662: No GPU at all
- **Likelihood**: MEDIUM-HIGH
- **Note**: Suggests region-specific infrastructure issue

### 4. Region-Specific GPU Availability
- **Theory**: fr-par-2 region has no GPU inventory, falls back to standard instances
- **Evidence**:
  - Consistent across multiple pool creation attempts
  - Metadata anomaly suggests provisioning fallback
- **Likelihood**: MEDIUM
- **Note**: Would explain why Cloud Manager shows GPU type but instance lacks hardware

## Next Debugging Steps

### 1. Verify Instance Type via Metadata
```bash
# Check Linode instance metadata
curl -H "Metadata-Token: $TOKEN" http://169.254.169.254/v1/instance
```

### 2. Check System Logs for GPU-Related Errors
```bash
kubectl debug node/lke531921-776662-3bbc7ff10000 -it --image=ubuntu -- \
  bash -c "chroot /host dmesg | grep -i 'nvidia\|gpu\|pci'"
```

### 3. Check Kernel Boot Parameters
```bash
kubectl debug node/lke531921-776662-3bbc7ff10000 -it --image=ubuntu -- \
  bash -c "chroot /host cat /proc/cmdline"
```

### 4. Verify IOMMU/Passthrough Support
```bash
kubectl debug node/lke531921-776662-3bbc7ff10000 -it --image=ubuntu -- \
  bash -c "chroot /host dmesg | grep -i iommu"
```

### 5. Check Linode Metadata Service
```bash
# From within the node
curl http://169.254.169.254/v1/instance/type
curl http://169.254.169.254/v1/instance/specs
```

### 6. Compare with Working Linode GPU Instance
- If possible, check `lspci` output from a known-working Linode GPU instance
- Compare kernel version, boot parameters, IOMMU configuration

## Questions for Linode Support

1. **GPU Attachment**: Is the RTX 4000 Ada GPU actually attached to instance lke531921-776662-3bbc7ff10000?
2. **Hypervisor Config**: Is GPU passthrough properly configured for this instance?
3. **Region Availability**: Are there any GPU availability issues in fr-par-2?
4. **Instance Type Verification**: Can you confirm the instance is actually running as g2-gpu-rtx4000a1-m (not a standard instance)?
5. **Known Issues**: Are there any known issues with GPU provisioning on LKE in fr-par-2?
6. **Provisioning Logs**: Can you check provisioning logs for this instance for any GPU-related errors?

## Workarounds Attempted

1. ✅ Deleted and recreated GPU node pool → Same issue
2. ✅ Rebooted instance → Same issue
3. ✅ Different GPU operator configurations → No GPU to configure
4. ❌ SSH access configured → Doesn't fix missing hardware
5. ❌ Different region → Not yet attempted

## Recommended Actions

### Immediate
1. **Contact Linode Support** with this document
2. **Request GPU attachment verification** for instance lke531921-776662-3bbc7ff10000
3. **Ask for provisioning logs** to identify where GPU attachment failed

### Short-term
1. **Try different region** if fr-par-2 has GPU issues
2. **Request instance recreation** with proper GPU attachment
3. **Consider non-LKE GPU instance** as alternative (dedicated Linode with GPU)

### Long-term
1. **Evaluate alternative cloud providers** if Linode GPU issues persist:
   - Google GKE (excellent GPU support)
   - AWS EKS (mature GPU AMIs)
   - Azure AKS (stable GPU node pools)

## Files Reference

- **This document**: `/home/dev/ai-aas/docs/GPU_HARDWARE_MISSING_ISSUE.md`
- **Original NVML issue**: `/home/dev/ai-aas/docs/LINODE_SUPPORT_TICKET.md`
- **Deployment status**: `/home/dev/ai-aas/docs/GPU_DEPLOYMENT_STATUS.md`
- **Ready configurations**:
  - `/home/dev/ai-aas/infra/helm/charts/vllm-deployment/values-unsloth-gpt-oss-20b.yaml`
  - `/home/dev/ai-aas/scripts/deploy-vllm-linode.sh`

## SSH Access

Root password set for direct debugging.

```bash
ssh root@<NODE_IP>
# Then run: lspci, dmesg, etc.
```

## Contact Information

- **Cluster**: lke531921
- **Region**: fr-par-2
- **Node**: lke531921-776662-3bbc7ff10000
- **External IP**: 172.232.49.84
- **Instance Type**: g2-gpu-rtx4000a1-m (per Cloud Manager)
