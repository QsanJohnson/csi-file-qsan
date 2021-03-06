# csi-file-qsan
A Container Storage Interface ([CSI](https://github.com/container-storage-interface/spec)) Driver for [Qsan](https://qsan.com) NAS Storage.

## Features
- Support NFS protocol on Qsan QSM storage
- Support PV options: block size, provision and compress

## Deployment
### Install
Clone the git repository code
```
git clone https://github.com/QsanJohnson/csi-file-qsan
```

Enter the deploy directory
```
cd csi-file-qsan/deploy
```

Modify qsan-auth.yaml secret file to specify Qsan storage address and login credentials.
It can support multiple Qsan storages.
```
storageArrays:
- server: 192.168.1.1
  port: 80
  https: “false”
  username: admin
  password: 1234
- server: 192.168.1.2
  port: 80
  https: “false”
  username: admin
  password: 1234
```

Finally execute install.sh script to deploy
```
./install.sh
```

### Uninstall
```
cd csi-file-qsan/deploy
./uninstall.sh
```

## Usage
### Create a StorageClass
Here is an example of StorageClass template
```
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: qsan-nas-storage
provisioner: file.csi.qsan.com
parameters:
  server: 192.168.206.156
  datastoreId: "2459490916446723448"
  blocksize: "65536"
  provision: "thick"
  compress: "lz4"
reclaimPolicy: Delete
volumeBindingMode: Immediate
mountOptions:
  - hard
  - nfsvers=3
  - nolock
```
- server: Qsan NAS storage address
- datastoreId: The GUID of datastore to create actual persistent volume on
- blocksize: means recordsize of ZFS filesystem. The valid value can be 1024, 2048 ..., 65536
- provision: "thin" or "thick"
- compress: "on", "off", "genericzero", "empty" or "lz4"

### Create a PersistentVolumeClaim (PVC)
Here is an example of PersistentVolumeClaim template
```
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: qsan-csi-pvc
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 10Gi
  storageClassName: qsan-nas-storage
  ```
