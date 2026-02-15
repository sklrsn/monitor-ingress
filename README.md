# ingress-tls-operator
Kubernetes operator that monitors ingresses across namespaces. If it finds any ingress
 that isn't secured by a TLS certificate, it logs an error.

## Description
Multi-tenant Kubernetes cluster has a problem with unsafe configurations. People constantly 
create ingress routes and leave them insecure without TLS encryption, exposing plain HTTP 
endpoints towards the internet.

## Getting Started

### Prerequisites
- go version v1.24.6+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/ingress-tls-operator:tag
```

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/ingress-tls-operator:tag
```
**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

**Create bootstrap configMap:**
```sh
kubectl create cm ingress-tls-watcher-config --from-literal http-timeout=30s --from-literal tls-insecure=false --from-literal tls-trustanchors="" --from-literal periodic-scan-interval=60m --from-literal periodic-scan-workers=5 --dry-run -o yaml --namespace ingress-tls-operator-system | kubectl apply -f -
```

```
apiVersion: v1
data:
  http-timeout: 30s
  periodic-scan-interval: 60m
  periodic-scan-workers: "5"
  tls-insecure: "false"
  tls-trustanchors: ""
kind: ConfigMap
metadata:
  name: ingress-tls-watcher-config
```

http-timeout: 30s            # HTTP request timeout for redirect checks

periodic-scan-interval: 60m  # Full cluster scan frequency

periodic-scan-workers: "5"   # Concurrent workers used to process while scanning ingress resources

tls-insecure: "false"        # Skip TLS verification (testing only)

tls-trustanchors: ""         # Additional trusted CAs or domains


### Sample Output

```
  % kubectl get ingress --namespace ckad nginx-ingress -o yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"networking.k8s.io/v1","kind":"Ingress","metadata":{"annotations":{},"name":"nginx-ingress","namespace":"ckad"},"spec":{"ingressClassName":"nginx","rules":[{"host":"sklrsn.in","http":{"paths":[{"backend":{"service":{"name":"nginx","port":{"number":8080}}},"path":"/","pathType":"Prefix"}]}}]},"status":{"loadBalancer":{}}}
  creationTimestamp: "2026-02-15T15:18:26Z"
  generation: 1
  name: nginx-ingress
  namespace: ckad
  resourceVersion: "18709"
  uid: 36ec7a7e-08c3-4119-ac43-4423b5b8c392
spec:
  ingressClassName: nginx
  rules:
  - host: sklrsn.in
    http:
      paths:
      - backend:
          service:
            name: nginx
            port:
              number: 8080
        path: /
        pathType: Prefix
status:
  loadBalancer: {}
```

```sh
2026-02-15T15:31:49Z	DEBUG	reconciling ingress	{"controller": "ingress", "controllerGroup": "networking.k8s.io", "controllerKind": "Ingress", "Ingress": {"name":"nginx-ingress","namespace":"ckad"}, "namespace": "ckad", "name": "nginx-ingress", "reconcileID": "6b59652b-71a2-4b6a-b67f-801de5777081", "namespace": "ckad", "name": "nginx-ingress"}
2026-02-15T15:31:49Z	INFO	CRITICAL: no TLS configured	{"controller": "ingress", "controllerGroup": "networking.k8s.io", "controllerKind": "Ingress", "Ingress": {"name":"nginx-ingress","namespace":"ckad"}, "namespace": "ckad", "name": "nginx-ingress", "reconcileID": "6b59652b-71a2-4b6a-b67f-801de5777081", "namespace": "ckad", "name": "nginx-ingress", "path": "http://sklrsn.in/", "level": "critical"}
```