# Chapter 1: Kubernetes Basics

In this chapter you'll learn to work with Kubernetes using `kubectl` on your Docker Desktop cluster. By the end, you'll understand pods, deployments, services, namespaces, and how to debug problems.

## Your Cluster

Docker Desktop gives you a single-node Kubernetes cluster. It's a real Kubernetes cluster, just running on your laptop instead of on remote servers.

```bash
# See your cluster info
kubectl cluster-info

# See the node (just one — your laptop)
kubectl get nodes

# See what's already running (system components)
kubectl get pods -A
```

The `-A` flag means "all namespaces." You'll see system pods like `coredns`, `kube-proxy`, etc. These are Kubernetes internals — don't touch them.

## Step 1: Create a Namespace

A namespace is a virtual folder for organizing your resources. Let's create one for this tutorial:

```bash
kubectl create namespace tutorial
```

From now on, we'll work in this namespace by adding `-n tutorial` to our commands. You can also set it as the default:

```bash
kubectl config set-context --current --namespace=tutorial
```

Now `kubectl get pods` will automatically look in the `tutorial` namespace.

## Step 2: Run Your First Pod

A pod is the smallest unit in Kubernetes. Let's run one:

```bash
kubectl run my-nginx --image=nginx:latest -n tutorial
```

This tells Kubernetes: "Run a container using the `nginx` image and call it `my-nginx`."

```bash
# Check if it's running
kubectl get pods -n tutorial

# You should see:
# NAME       READY   STATUS    RESTARTS   AGE
# my-nginx   1/1     Running   0          10s
```

### Inspect the Pod

```bash
# Detailed information about the pod
kubectl describe pod my-nginx -n tutorial

# See the pod's logs (nginx access log)
kubectl logs my-nginx -n tutorial

# Open a shell inside the pod
kubectl exec -it my-nginx -n tutorial -- /bin/bash

# Inside the container, you can run:
# curl localhost     (see the nginx welcome page)
# exit               (leave the container)
```

### Delete the Pod

```bash
kubectl delete pod my-nginx -n tutorial
```

The pod is gone. If you run `kubectl get pods -n tutorial`, it won't be listed. Pods by themselves are ephemeral — if they crash or get deleted, they're gone forever. That's why we use Deployments.

## Step 3: Create a Deployment

A Deployment ensures your pods stay running. If a pod crashes, the Deployment creates a new one.

Create a file called `deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
  namespace: tutorial
spec:
  replicas: 3
  selector:
    matchLabels:
      app: web-app
  template:
    metadata:
      labels:
        app: web-app
    spec:
      containers:
        - name: nginx
          image: nginx:latest
          ports:
            - containerPort: 80
```

Apply it:

```bash
kubectl apply -f deployment.yaml
```

Check the result:

```bash
# See the deployment
kubectl get deployments -n tutorial

# See the 3 pods it created
kubectl get pods -n tutorial

# You should see 3 pods:
# web-app-xxxxx-yyyyy   1/1     Running   0          5s
# web-app-xxxxx-zzzzz   1/1     Running   0          5s
# web-app-xxxxx-wwwww   1/1     Running   0          5s
```

### Test Self-Healing

Delete one pod and watch Kubernetes create a new one:

```bash
# Get the pod names
kubectl get pods -n tutorial

# Delete one (use the actual pod name from the output above)
kubectl delete pod web-app-xxxxx-yyyyy -n tutorial

# Immediately check again — a new pod is already being created
kubectl get pods -n tutorial
```

The Deployment noticed one pod was missing and created a replacement. This is why production workloads use Deployments, not bare pods.

### Scale the Deployment

```bash
# Scale to 5 replicas
kubectl scale deployment web-app --replicas=5 -n tutorial

# Check
kubectl get pods -n tutorial
# Now you should see 5 pods

# Scale back down
kubectl scale deployment web-app --replicas=2 -n tutorial
```

## Step 4: Expose with a Service

Your pods are running, but they're only reachable inside the cluster. A Service gives them a stable network address.

Create a file called `service.yaml`:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: web-app
  namespace: tutorial
spec:
  type: NodePort
  selector:
    app: web-app
  ports:
    - port: 80
      targetPort: 80
      nodePort: 30080
```

Apply it:

```bash
kubectl apply -f service.yaml
```

Now you can access your app from your browser:

```bash
# Open in browser or curl
curl http://localhost:30080

# You should see the nginx welcome page
```

The Service routes traffic to any pod with the label `app: web-app`. It doesn't matter which pod handles the request — the Service load-balances between them.

## Step 5: ConfigMaps and Secrets

### ConfigMap

A ConfigMap holds non-secret configuration:

```bash
kubectl create configmap my-config \
  --from-literal=greeting="Hello from ConfigMap" \
  --from-literal=environment="tutorial" \
  -n tutorial

# View it
kubectl get configmap my-config -n tutorial -o yaml
```

### Secret

A Secret holds sensitive data (base64-encoded):

```bash
kubectl create secret generic my-secret \
  --from-literal=password="s3cret123" \
  --from-literal=api-key="abc-def-ghi" \
  -n tutorial

# View it (values are base64-encoded)
kubectl get secret my-secret -n tutorial -o yaml

# Decode a value
kubectl get secret my-secret -n tutorial -o jsonpath='{.data.password}' | base64 -d
# Output: s3cret123
```

## Step 6: Labels and Selectors

Labels are key-value pairs attached to resources. They're how Kubernetes connects things together.

```bash
# See labels on your pods
kubectl get pods -n tutorial --show-labels

# Filter pods by label
kubectl get pods -n tutorial -l app=web-app

# Add a label
kubectl label pod <pod-name> tier=frontend -n tutorial
```

The Service you created uses `selector: app: web-app` to find the right pods. If you remove that label from a pod, the Service stops routing traffic to it.

## Step 7: Debugging

When things go wrong, these commands help you investigate:

```bash
# What's happening with a pod?
kubectl describe pod <pod-name> -n tutorial

# Pod logs
kubectl logs <pod-name> -n tutorial

# If the pod keeps crashing, see previous logs
kubectl logs <pod-name> -n tutorial --previous

# See events in the namespace
kubectl get events -n tutorial --sort-by='.lastTimestamp'

# See all resources in the namespace
kubectl get all -n tutorial
```

## Cleanup

```bash
kubectl delete namespace tutorial
```

This deletes everything in the namespace — pods, services, deployments, configmaps, secrets — all gone.

## What You Learned

- **kubectl** is how you talk to a Kubernetes cluster
- **Pods** are the smallest unit — one or more containers running together
- **Deployments** keep pods running and handle scaling and self-healing
- **Services** provide stable network access to pods
- **ConfigMaps** and **Secrets** hold configuration data
- **Labels** connect resources together (Services find pods by labels)
- **Namespaces** organize resources into logical groups

## Next Chapter

[Chapter 2: Helm Basics](02-helm-basics.md) — learn to install applications as packages instead of writing YAML files manually.
