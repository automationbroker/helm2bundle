# helm2bundle

Inspects a helm chart and outputs an apb.yml file and Dockerfile based on
values in the chart.

Example usage:

```
$ helm2bundle redis-1.1.12.tgz 
$ ls
apb.yml  Dockerfile  redis-1.1.12.tgz
```

Then you can ``apb push`` to build and push the service bundle into your
cluster's registry.
