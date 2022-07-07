#!/bin/sh
BASE_DIR="$(cd "$(dirname "$0")" && pwd)"

kubectl delete -f ${BASE_DIR}/csi-nfs-node.yaml
kubectl delete -f ${BASE_DIR}/csi-nfs-controller.yaml
kubectl delete -f ${BASE_DIR}/csi-nfs-driverinfo.yaml
kubectl delete -f ${BASE_DIR}/rbac-csi-nfs.yaml

kubectl delete secret qsan-auth-secret -n qsan
kubectl delete ns qsan
