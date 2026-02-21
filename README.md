# ktunnels [![go](https://github.com/int128/ktunnels/actions/workflows/go.yaml/badge.svg)](https://github.com/int128/ktunnels/actions/workflows/go.yaml)

This is a Kubernetes controller to set up an [Envoy Proxy](https://www.envoyproxy.io) for port-forwarding from your computer to remote hosts.

## Purpose

For local development, it is nice to develop an application using a real database.
If a database is running in a **private network** and outside a cluster, such as Amazon RDS or Azure Database, it is **unreachable** from your computer.

This allows you to connect from your computer to a host outside a cluster.
You just run `kubectl port-forward` and set up your application to connect to localhost.

![diagram](docs/diagram.svg)

This solution is an alternative of SSH or SOCKS bastion.
You no longer maintain your bastion servers.

## Getting Started

<<<<<<< HEAD
### Prerequisites
- go version v1.24.6+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.
=======
### For administrator
>>>>>>> tmp-original-21-02-26-14-49

Deploy the controller.

```sh
kubectl apply -f https://github.com/int128/ktunnels/releases/download/v0.9.1/ktunnels.yaml
```

### For user

Create a `Proxy` resource and `Tunnel` resource.

```yaml
# kubectl apply -f proxy.yaml
apiVersion: ktunnels.int128.github.io/v1
kind: Proxy
metadata:
  name: default
```

```yaml
# kubectl apply -f tunnel.yaml
apiVersion: ktunnels.int128.github.io/v1
kind: Tunnel
metadata:
  name: backend-db
spec:
  host: backend-db.staging
  port: 5432
  proxy:
    name: default
```

Run port-forward on your computer.

```sh
kubectl port-forward svc/backend-db 5432:5432
```

You can connect to the database via `localhost:5432`.

## How it works

This controller sets up a set of `Deployment` and `ConfigMap` for each proxy.

```console
% kubectl tree proxy default
NAMESPACE  NAME                                               READY  REASON  AGE
default    Proxy/default                                      -              5m9s
default    ├─ConfigMap/ktunnels-proxy-default                 -              5m9s
default    └─Deployment/ktunnels-proxy-default                -              5m9s
default      └─ReplicaSet/ktunnels-proxy-default-5db5d68b6c   -              5m9s
default        └─Pod/ktunnels-proxy-default-5db5d68b6c-wnncc  True           5m9s
```

It also sets up a `Service` for each tunnel.

```console
% k tree tunnel main-db
NAMESPACE  NAME                             READY  REASON  AGE
default    Tunnel/main-db                   -              32m
default    └─Service/main-db                -              32m
default      └─EndpointSlice/main-db-cxx65  -              32m
```

<<<<<<< HEAD
>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/ktunnels:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/ktunnels/<tag or branch>/dist/install.yaml
```

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

```sh
kubebuilder edit --plugins=helm/v2-alpha
```

2. See that a chart was generated under 'dist/chart', and users
can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
=======
## Contributions
>>>>>>> tmp-original-21-02-26-14-49

This is an open source software licensed under Apache License 2.0.
Feel free to open issues and pull requests for improving code and documents.
