# ingress-tls-operator

A Kubernetes operator that continuously monitors Ingress resources across all namespaces and flags any that are exposed without TLS. Built with [Kubebuilder](https://book.kubebuilder.io/).

---

## Problem

In multi-tenant clusters, teams frequently create Ingress routes and forget to configure TLS — leaving plain HTTP endpoints exposed to the internet. There is no built-in Kubernetes mechanism to detect or enforce this.

`ingress-tls-operator` solves this by running as a controller that:

- **Reacts in real time** — reconciles every Ingress create/update event via the controller-runtime watch loop
- **Scans periodically** — runs a full cluster-wide sweep on a configurable interval using a worker pool
- **Logs violations** — emits a structured `CRITICAL` log entry for any Ingress missing a TLS block, making it trivially parseable by log aggregators (Loki, Splunk, etc.)

---

## How It Works

```
Kubernetes API Server
        │
        │  watch (networking.k8s.io/v1 Ingress)
        ▼
 IngressReconciler
        │
        ├── TLS block present?  ──► OK, no action
        │
        └── TLS block absent?   ──► log CRITICAL + path details
                                         │
                              periodic scanner (configurable interval)
                              runs full cluster sweep via worker pool
```

The operator requires no CRDs — it watches the native `networking.k8s.io/v1 Ingress` resource. Configuration is injected via a ConfigMap at startup.

---

## Configuration

The operator reads its runtime config from a ConfigMap named `ingress-tls-watcher-config` in the `ingress-tls-operator-system` namespace.

| Key | Default | Description |
|---|---|---|
| `http-timeout` | `30s` | HTTP request timeout for redirect checks |
| `periodic-scan-interval` | `60m` | How often a full cluster-wide scan runs |
| `periodic-scan-workers` | `5` | Concurrent workers used during periodic scans |
| `tls-insecure` | `false` | Skip TLS verification — **testing only** |
| `tls-trustanchors` | `""` | Additional trusted CAs or domain anchors |

Create or update the ConfigMap:

```bash
kubectl create cm ingress-tls-watcher-config \
  --from-literal http-timeout=30s \
  --from-literal tls-insecure=false \
  --from-literal tls-trustanchors="" \
  --from-literal periodic-scan-interval=60m \
  --from-literal periodic-scan-workers=5 \
  --dry-run=client -o yaml \
  --namespace ingress-tls-operator-system | kubectl apply -f -
```

The resulting ConfigMap looks like:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ingress-tls-watcher-config
  namespace: ingress-tls-operator-system
data:
  http-timeout: 30s
  periodic-scan-interval: 60m
  periodic-scan-workers: "5"
  tls-insecure: "false"
  tls-trustanchors: ""
```

---

## Getting Started

### Prerequisites

| Tool | Minimum version |
|---|---|
| Go | v1.24.6+ |
| Docker | v17.03+ |
| kubectl | v1.11.3+ |
| Kubernetes cluster | v1.11.3+ |

### Deploy to a Cluster

**1. Build and push the operator image:**

```bash
make docker-build docker-push IMG=<your-registry>/ingress-tls-operator:tag
```

**2. Install CRDs and RBAC resources:**

```bash
make install
```

**3. Deploy the operator:**

```bash
make deploy IMG=<your-registry>/ingress-tls-operator:tag
```

**4. Create the bootstrap ConfigMap** (see [Configuration](#configuration) above).

### Teardown

```bash
# Remove the operator deployment
make undeploy

# Remove CRDs and cluster resources
make uninstall
```

---

## Example

Given an Ingress with no TLS configured:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: nginx-ingress
  namespace: ckad
spec:
  ingressClassName: nginx
  rules:
  - host: sklrsn.in
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: nginx
            port:
              number: 8080
  # No tls: block — this will be flagged
```

The operator emits:

```
2026-02-15T15:31:49Z  DEBUG  reconciling ingress
  {"controller": "ingress", "namespace": "ckad", "name": "nginx-ingress", ...}

2026-02-15T15:31:49Z  INFO   CRITICAL: no TLS configured
  {"controller": "ingress", "namespace": "ckad", "name": "nginx-ingress",
   "path": "http://sklrsn.in/", "level": "critical"}
```

The structured log output is intentional — filter on `"level": "critical"` in your log aggregator to build alerts.

---

## Project Structure

```
monitor-ingress/
└── ingress-tls-operator/   # Kubebuilder-scaffolded operator
    ├── api/                # API types (if CRDs are added)
    ├── internal/
    │   └── controller/     # IngressReconciler + periodic scanner
    ├── config/             # Kustomize manifests (RBAC, deployment, etc.)
    ├── Dockerfile
    └── Makefile
```

---

## License

[MIT](LICENSE)
