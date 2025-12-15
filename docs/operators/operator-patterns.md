---
title: Kubernetes Operator Patterns
last_updated: 2025-12-10
owner: operator-developer
---

# Kubernetes Operator Patterns

This document describes common patterns used in our Kubernetes operators built with controller-runtime.

## Reconciliation Loop

The reconciliation loop is the heart of any operator. It continuously works to make the actual state match the desired state.

### Basic Structure

```go
func (r *MyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := log.FromContext(ctx)

    // 1. Fetch the custom resource
    resource := &myv1.MyResource{}
    if err := r.Get(ctx, req.NamespacedName, resource); err != nil {
        // Resource deleted - nothing to do
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // 2. Handle deletion
    if !resource.DeletionTimestamp.IsZero() {
        return r.handleDeletion(ctx, resource)
    }

    // 3. Ensure finalizer
    if !controllerutil.ContainsFinalizer(resource, finalizerName) {
        controllerutil.AddFinalizer(resource, finalizerName)
        if err := r.Update(ctx, resource); err != nil {
            return ctrl.Result{}, err
        }
    }

    // 4. Reconcile owned resources
    if err := r.reconcileOwnedResources(ctx, resource); err != nil {
        return ctrl.Result{}, err
    }

    // 5. Update status
    if err := r.updateStatus(ctx, resource); err != nil {
        return ctrl.Result{}, err
    }

    return ctrl.Result{}, nil
}
```

### Requeue Strategies

```go
// Success - don't requeue
return ctrl.Result{}, nil

// Requeue immediately (transient error, resource not ready)
return ctrl.Result{Requeue: true}, nil

// Requeue after delay (polling, rate limiting, backoff)
return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil

// Error - controller-runtime handles backoff
return ctrl.Result{}, err
```

## Status Updates

Always use the status subresource for status updates:

```go
// CORRECT: Use Status().Update()
resource.Status.Phase = "Ready"
resource.Status.Message = "All systems operational"
if err := r.Status().Update(ctx, resource); err != nil {
    return ctrl.Result{}, err
}

// WRONG: This updates the whole resource, not just status
if err := r.Update(ctx, resource); err != nil { ... }
```

### Status Conditions Pattern

```go
type MyResourceStatus struct {
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Setting a condition
meta.SetStatusCondition(&resource.Status.Conditions, metav1.Condition{
    Type:               "Ready",
    Status:             metav1.ConditionTrue,
    Reason:             "AllSystemsGo",
    Message:            "Resource is fully operational",
    LastTransitionTime: metav1.Now(),
})
```

## Owner References

Owner references enable automatic garbage collection when the parent is deleted:

```go
// Set owner reference on child resource
if err := ctrl.SetControllerReference(parent, child, r.Scheme); err != nil {
    return ctrl.Result{}, err
}

// Now when parent is deleted, child will be garbage collected
```

### Watching Owned Resources

```go
func (r *MyReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&myv1.MyResource{}).
        Owns(&corev1.ConfigMap{}).    // Watch owned ConfigMaps
        Owns(&appsv1.Deployment{}).   // Watch owned Deployments
        Complete(r)
}
```

## Finalizers

Finalizers prevent deletion until cleanup is complete:

```go
const finalizerName = "myresource.example.com/finalizer"

func (r *MyReconciler) handleDeletion(ctx context.Context, resource *myv1.MyResource) (ctrl.Result, error) {
    if controllerutil.ContainsFinalizer(resource, finalizerName) {
        // Run cleanup logic
        if err := r.cleanupOwnedResources(ctx, resource); err != nil {
            return ctrl.Result{}, err
        }

        // Remove finalizer to allow deletion
        controllerutil.RemoveFinalizer(resource, finalizerName)
        if err := r.Update(ctx, resource); err != nil {
            return ctrl.Result{}, err
        }
    }
    return ctrl.Result{}, nil
}
```

## Error Handling

### Transient vs Permanent Errors

```go
// Transient error - requeue for retry
if errors.IsConflict(err) {
    return ctrl.Result{Requeue: true}, nil
}

// Permanent error - update status, don't requeue
if isPermanentError(err) {
    resource.Status.Phase = "Failed"
    resource.Status.Message = err.Error()
    r.Status().Update(ctx, resource)
    return ctrl.Result{}, nil  // Don't requeue
}

// Unknown error - let controller-runtime handle backoff
return ctrl.Result{}, err
```

### Exponential Backoff

```go
const (
    initialBackoff = 1 * time.Minute
    maxBackoff     = 16 * time.Minute
)

func calculateBackoff(retryCount int32) time.Duration {
    backoff := initialBackoff * time.Duration(1<<retryCount)
    if backoff > maxBackoff {
        return maxBackoff
    }
    return backoff
}

// Usage
resource.Status.RetryCount++
backoff := calculateBackoff(resource.Status.RetryCount)
return ctrl.Result{RequeueAfter: backoff}, nil
```

## Creating Child Resources

### Idempotent Creation

```go
func (r *MyReconciler) ensureDeployment(ctx context.Context, resource *myv1.MyResource) error {
    deployment := &appsv1.Deployment{}
    err := r.Get(ctx, client.ObjectKey{
        Name:      resource.Name + "-deployment",
        Namespace: resource.Namespace,
    }, deployment)

    if err != nil && errors.IsNotFound(err) {
        // Create new deployment
        deployment = r.buildDeployment(resource)
        if err := ctrl.SetControllerReference(resource, deployment, r.Scheme); err != nil {
            return err
        }
        return r.Create(ctx, deployment)
    } else if err != nil {
        return err
    }

    // Update existing deployment if needed
    if needsUpdate(deployment, resource) {
        deployment.Spec = r.buildDeploymentSpec(resource)
        return r.Update(ctx, deployment)
    }

    return nil
}
```

## Working with Unstructured Resources

For CRDs you don't have Go types for (like KServe):

```go
import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

// Define GVK
var InferenceServiceGVK = schema.GroupVersionKind{
    Group:   "serving.kserve.io",
    Version: "v1beta1",
    Kind:    "InferenceService",
}

// Create unstructured resource
isvc := &unstructured.Unstructured{}
isvc.SetGroupVersionKind(InferenceServiceGVK)
isvc.SetName(name)
isvc.SetNamespace(namespace)

// Set nested fields
unstructured.SetNestedField(isvc.Object, "vllm", "spec", "predictor", "model", "runtime")

// Get nested fields
runtime, found, err := unstructured.NestedString(isvc.Object, "spec", "predictor", "model", "runtime")
```

## Event Recording

Emit events for important state changes:

```go
import "k8s.io/client-go/tools/record"

type MyReconciler struct {
    client.Client
    Scheme   *runtime.Scheme
    Recorder record.EventRecorder
}

// In reconcile
r.Recorder.Event(resource, corev1.EventTypeNormal, "Created", "Successfully created deployment")
r.Recorder.Eventf(resource, corev1.EventTypeWarning, "Failed", "Failed to create deployment: %v", err)
```

## Metrics

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
    reconcileTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "myresource_reconcile_total",
            Help: "Total reconciliations",
        },
        []string{"result"},
    )
)

func init() {
    metrics.Registry.MustRegister(reconcileTotal)
}

// In reconcile
defer func() {
    if err != nil {
        reconcileTotal.WithLabelValues("error").Inc()
    } else {
        reconcileTotal.WithLabelValues("success").Inc()
    }
}()
```

## Testing Patterns

### envtest Setup

```go
var _ = BeforeSuite(func() {
    testEnv = &envtest.Environment{
        CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
    }
    cfg, err := testEnv.Start()
    Expect(err).NotTo(HaveOccurred())

    k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
    Expect(err).NotTo(HaveOccurred())
})
```

### Testing Reconciliation

```go
It("should create deployment when resource is created", func() {
    resource := &myv1.MyResource{
        ObjectMeta: metav1.ObjectMeta{
            Name:      "test",
            Namespace: "default",
        },
        Spec: myv1.MyResourceSpec{...},
    }
    Expect(k8sClient.Create(ctx, resource)).To(Succeed())

    // Trigger reconciliation
    _, err := reconciler.Reconcile(ctx, ctrl.Request{
        NamespacedName: types.NamespacedName{
            Name:      resource.Name,
            Namespace: resource.Namespace,
        },
    })
    Expect(err).NotTo(HaveOccurred())

    // Verify deployment was created
    deployment := &appsv1.Deployment{}
    Expect(k8sClient.Get(ctx, client.ObjectKey{
        Name:      resource.Name + "-deployment",
        Namespace: resource.Namespace,
    }, deployment)).To(Succeed())
})
```
