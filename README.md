# helm2bundle

Inspects a helm chart and outputs an apb.yml file and Dockerfile based on
values in the chart.

## Status

This is pre-release experimental software.


## Usage

```
$ helm2bundle redis-1.1.12.tgz 
$ ls
apb.yml  Dockerfile  redis-1.1.12.tgz
```

On OpenShift you can ``apb push`` to build and push the service bundle into your
cluster's registry.

On plain Kubernetes, you can ``apb build`` and then tag and push to a registry that
your broker is configured to access.
