# Velero Helm Plugin

This repository contains velero plugin which can backup helm releases deployed by tiller.

This is plugin is fork from https://github.com/Dennor/velero-plugin-helm. But work with velero 1.0.x.

## Using the plugin

To use the plugin just add it to velero.

```
$ velero plugin add runzexia/velero-plugin-helm:latest
```

## Example of backup and restore

1. Deploy example chart `nginx-chart`

```
$ helm install --name nginx-example-release ./examples/nginx-chart
```

2. Once it's the release is deployed and ready create a backup

```
$ velero backup create nginx-example-release-backup -l "OWNER=TILLER,NAME=nginx-example-release"
$ velero backup describe nginx-example-release-backup
```

3. "Accidentally" delete the release.

```
$ helm delete --purge nginx-example-release
$ kubectl delete secret nginx-example-release-nginx
```

4. Restore release

```
$ velero restore create --from-backup nginx-example-release-backup
```

## Build image

```
$ make container
```
