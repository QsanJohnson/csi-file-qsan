package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/QsanJohnson/csi-file-qsan/pkg/nfs"

	"k8s.io/klog/v2"
)

var (
	endpoint          = flag.String("endpoint", "unix://tmp/csi.sock", "CSI endpoint")
	nodeID            = flag.String("nodeid", "", "node id")
	mountPermissions  = flag.Uint64("mount-permissions", 0777, "mounted folder permissions")
	driverName        = flag.String("drivername", nfs.DefaultDriverName, "name of the driver")
	workingMountDir   = flag.String("working-mount-dir", "/tmp", "working directory for provisioner to mount nfs shares temporarily")
	maxVolumesPerNode = flag.Int64("maxvolumespernode", 128, "limit of volumes per node")
	secretFile        = flag.String("secret-file", "", "secret file to authenticate Qsan storage server")
)

func init() {
	_ = flag.Set("logtostderr", "true")
}

func main() {
	// klog.InitFlags(nil)
	flag.Parse()
	if *nodeID == "" {
		klog.Warning("nodeid is empty")
	}
	if *secretFile == "" {
		fmt.Fprintf(os.Stderr, "secret-file argument is mandatory")
		os.Exit(1)
	}

	handle()
	os.Exit(0)
}

func handle() {
	driverOptions := nfs.DriverOptions{
		NodeID:            *nodeID,
		DriverName:        *driverName,
		Endpoint:          *endpoint,
		MountPermissions:  *mountPermissions,
		WorkingMountDir:   *workingMountDir,
		MaxVolumesPerNode: *maxVolumesPerNode,
		SecretFile:        *secretFile,
	}
	d := nfs.NewDriver(&driverOptions)
	d.Run(false)
}
