# remove-cpu-limits

A [Kubewarden](https://kubewarden.io) **mutating** policy, written in Go and
compiled to WebAssembly with [TinyGo](https://tinygo.org), that removes CPU
limits from Pods.

For every container in the Pod's `spec.containers`, `spec.initContainers` and
`spec.ephemeralContainers`, the policy deletes `resources.limits.cpu`. CPU
requests and memory limits/requests are left untouched. If dropping the CPU
limit leaves an empty `limits` (or `resources`) object behind, that empty
object is removed too.

Pods without any CPU limit are accepted unchanged. Requests for kinds other
than `Pod` are accepted unchanged as well.

## Settings

The policy takes no settings. `{}` or omitting settings entirely is valid.

## Example

Input:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: nginx
spec:
  containers:
    - name: nginx
      image: nginx
      resources:
        limits:
          cpu: 500m
          memory: 128Mi
        requests:
          cpu: 100m
          memory: 64Mi
```

Result after admission:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: nginx
spec:
  containers:
    - name: nginx
      image: nginx
      resources:
        limits:
          memory: 128Mi
        requests:
          cpu: 100m
          memory: 64Mi
```

## Deploying

```yaml
apiVersion: policies.kubewarden.io/v1
kind: ClusterAdmissionPolicy
metadata:
  name: remove-cpu-limits
spec:
  module: registry://ghcr.io/cdalar/policies/remove-cpu-limits:v0.1.0
  rules:
    - apiGroups: [""]
      apiVersions: ["v1"]
      resources: ["pods"]
      operations: ["CREATE", "UPDATE"]
  mutating: true
  settings: {}
```

Note: Pods created by controllers (Deployments, StatefulSets, Jobs, ...) go
through admission too, so their CPU limits are removed as well. The workload
object itself is not modified.

## Building and testing

```console
make test                   # Go unit tests (plain `go`)
make policy.wasm            # tinygo build (local tinygo, or the tinygo/tinygo:0.39.0 image)
make annotated-policy.wasm  # attach metadata.yml with kwctl
make e2e-tests              # bats + kwctl end-to-end tests
```

Try it locally with `kwctl`:

```console
kwctl run annotated-policy.wasm -r test_data/pod_with_cpu_limits.json --settings-json '{}'
```

## CI / release

- `.github/workflows/test.yml` runs on every push and pull request: Go unit
  tests, `go vet` + golangci-lint, and a TinyGo build followed by the `kwctl`
  e2e tests. The built `policy.wasm` is uploaded as a workflow artifact.
- `.github/workflows/release.yml` runs the same checks and then:
  - on every push to `main`, pushes `ghcr.io/cdalar/policies/remove-cpu-limits:latest`;
  - on a `vX.Y.Z` tag, pushes `:vX.Y.Z` and `:latest`, signs the artifact with
    cosign (keyless) and creates a GitHub release with the Wasm files attached.
    The tag must match `io.kubewarden.policy.version` in `metadata.yml`.

To cut a release: bump `io.kubewarden.policy.version`, commit, then
`git tag v0.1.0 && git push origin v0.1.0`.
