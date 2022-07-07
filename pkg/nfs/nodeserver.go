package nfs

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/QsanJohnson/goqsm"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	mount "k8s.io/mount-utils"
)

// NodeServer driver
type NodeServer struct {
	Driver  *Driver
	mounter mount.Interface
}

// NodePublishVolume mount the volume
func (ns *NodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	volCap := req.GetVolumeCapability()
	if volCap == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability missing in request")
	}
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path not provided")
	}
	// mountOptions := volCap.GetMount().GetMountFlags()
	// if req.GetReadonly() {
	// 	mountOptions = append(mountOptions, "ro")
	// }
	var mountOptions []string

	var server, scId, baseDir string
	mountPermissions := ns.Driver.mountPermissions
	performChmodOp := (mountPermissions > 0)
	for k, v := range req.GetVolumeContext() {
		switch strings.ToLower(k) {
		case paramServer:
			server = v
		case paramDatastoreId:
			scId = v
		case paramShare:
			baseDir = v
		case mountOptionsField:
			if v != "" {
				mountOptions = append(mountOptions, v)
			}
		case mountPermissionsField:
			if v != "" {
				var err error
				var perm uint64
				if perm, err = strconv.ParseUint(v, 8, 32); err != nil {
					return nil, status.Errorf(codes.InvalidArgument, fmt.Sprintf("invalid mountPermissions %s", v))
				}
				if perm == 0 {
					performChmodOp = false
				} else {
					mountPermissions = perm
				}
			}
		}
	}
	if server == "" {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("%v is a required parameter", paramServer))
	}
	if baseDir == "" {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("%v is a required parameter", paramShare))
	}
	klog.Infof("[NodePublishVolume] VolumeContext: server(%s), scId(%s), baseDir(%s), mountPermissions(%d), mountOptions(%v)", server, scId, baseDir, mountPermissions, mountOptions)

	volData, err := ns.Driver.GetContextDataFromVolumeContextID(volumeID)
	klog.Infof("[NodePublishVolume] volData: %+v", volData)
	if err != nil {
		klog.Warningf("[NodePublishVolume] failed to get volume context data for volume id %v publish: %v", volumeID, err)
		return &csi.NodePublishVolumeResponse{}, nil
	}

	server = getServerFromSource(volData.server)
	source := fmt.Sprintf("%s:%s", server, baseDir)

	authClient, err := ns.Driver.qsan.GetnAddAuthClient(ctx, server)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	volumeAPI := goqsm.NewVolume(authClient)
	err = volumeAPI.ExportVolume(ctx, volData.scId, volData.volId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	notMnt, err := ns.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(targetPath, os.FileMode(mountPermissions)); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			notMnt = true
		} else {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	if !notMnt {
		klog.Warning("NodePublishVolume: notMnt is false")
		return &csi.NodePublishVolumeResponse{}, nil
	}

	klog.Infof("[NodePublishVolume] volumeID(%v) source(%s) targetPath(%s) mountflags(%v)", volumeID, source, targetPath, mountOptions)
	err = ns.mounter.Mount(source, targetPath, "nfs", mountOptions)
	if err != nil {
		if os.IsPermission(err) {
			return nil, status.Error(codes.PermissionDenied, err.Error())
		}
		if strings.Contains(err.Error(), "invalid argument") {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	if performChmodOp {
		if err := chmodIfPermissionMismatch(targetPath, os.FileMode(mountPermissions)); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	} else {
		klog.Infof("skip chmod on targetPath(%s) since mountPermissions is set as 0", targetPath)
	}
	klog.Infof("volume(%s) mount %s on %s succeeded", volumeID, source, targetPath)
	return &csi.NodePublishVolumeResponse{}, nil
}

// NodeUnpublishVolume unmount the volume
func (ns *NodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	klog.Infof("[NodeUnpublishVolume] unmount volume %s on %s", volumeID, targetPath)
	err := mount.CleanupMountPoint(targetPath, ns.mounter, true /*extensiveMountPointCheck*/)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmount target %q: %v", targetPath, err)
	}

	volData, err := ns.Driver.GetContextDataFromVolumeContextID(volumeID)
	klog.Infof("[NodeUnpublishVolume] volData: %+v", volData)
	if err != nil {
		klog.Warningf("[NodeUnpublishVolume] failed to get volume context data for volume id %v unpublish: %v", volumeID, err)
		return &csi.NodeUnpublishVolumeResponse{}, nil
	}

	authClient, err := ns.Driver.qsan.GetnAddAuthClient(ctx, volData.server)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	volumeAPI := goqsm.NewVolume(authClient)
	err = volumeAPI.UnexportVolume(ctx, volData.scId, volData.volId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	klog.Infof("[NodeUnpublishVolume] Unexport volume %s successfully", volumeID)

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

// NodeGetInfo return info of the node on which this plugin is running
func (ns *NodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: ns.Driver.nodeID,
		// MaxVolumesPerNode: ns.Driver.maxVolumesPerNode,
	}, nil
}

// NodeGetCapabilities return the capabilities of the Node plugin
func (ns *NodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: ns.Driver.nscap,
	}, nil
}

// NodeGetVolumeStats get volume stats
func (ns *NodeServer) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	if len(req.VolumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "NodeGetVolumeStats volume ID was empty")
	}
	if len(req.VolumePath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "NodeGetVolumeStats volume path was empty")
	}

	klog.Infof("[NodeGetVolumeStats] get volume %s on %s", req.VolumeId, req.VolumePath)
	volData, err := ns.Driver.GetContextDataFromVolumeContextID(req.VolumeId)
	klog.Infof("[NodeGetVolumeStats] volData: %+v", volData)
	if err != nil {
		klog.Warningf("[NodeGetVolumeStats] failed to get volume context data for volume id %v status: %v", req.VolumeId, err)
		return &csi.NodeGetVolumeStatsResponse{}, nil
	}

	authClient, err := ns.Driver.qsan.GetnAddAuthClient(ctx, volData.server)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	volumeAPI := goqsm.NewVolume(authClient)
	v, err := volumeAPI.ListVolumes(ctx, volData.scId, volData.volId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if len(*v) != 1 {
		klog.Warningf("NodeGetVolumeStats: volume count(%d) != 1", len(*v))
	}

	vol := (*v)[0]
	return &csi.NodeGetVolumeStatsResponse{
		Usage: []*csi.VolumeUsage{
			&csi.VolumeUsage{
				Available: int64(vol.SizeMB - vol.UsedMB),
				Total:     int64(vol.SizeMB),
				Used:      int64(vol.UsedMB),
				Unit:      csi.VolumeUsage_BYTES,
			},
		},
		VolumeCondition: &csi.VolumeCondition{
			Abnormal: false,
			Message:  "",
		},
	}, nil
}

// NodeUnstageVolume unstage volume
func (ns *NodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// NodeStageVolume stage volume
func (ns *NodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// NodeExpandVolume node expand volume
func (ns *NodeServer) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
