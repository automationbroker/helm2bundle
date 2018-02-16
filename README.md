# helm2bundle

Inspects a helm chart and outputs an apb.yml file based on values in the chart.

Example output:

```
$ go run helm2bundle.go redis-1.1.12.tgz 
version: 1.0
name: redis-apb
description: Open source, advanced key-value store. It is often referred to as a data structure server since keys can contain strings, hashes, lists, sets and sorted sets.
bindable: False
async: optional
metadata:
  displayName: redis-helm
  imageURL: https://bitnami.com/assets/stacks/redis/img/redis-stack-220x234.png
plans:
  - name: default
    description: This default plan deploys helm chart redis
    free: True
    metadata: {}
    parameters: []
```
