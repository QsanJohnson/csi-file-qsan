#!/bin/sh
BASE_DIR="$(cd "$(dirname "$0")" && pwd)"

kubectl create ns qsan
kubectl create secret -n qsan generic qsan-auth-secret --from-file=${BASE_DIR}/qsan-auth.yaml

kubectl apply -f ${BASE_DIR}/rbac-csi-nfs.yaml
kubectl apply -f ${BASE_DIR}/csi-nfs-driverinfo.yaml
kubectl apply -f ${BASE_DIR}/csi-nfs-controller.yaml
kubectl apply -f ${BASE_DIR}/csi-nfs-node.yaml

