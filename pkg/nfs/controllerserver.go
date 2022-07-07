package nfs

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/QsanJohnson/goqsm"
	"k8s.io/klog/v2"
)

// ControllerServer controller server setting
type ControllerServer struct {
	Driver *Driver
}

// CreateVolume create a volume
func (cs *ControllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	name := req.GetName()
	klog.Infof("[CreateVolume] volumeName: %s", name)
	if len(name) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume name must be provided")
	}

	if err := isValidVolumeCapabilities(req.GetVolumeCapabilities()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	mountPermissions := cs.Driver.mountPermissions
	reqCapacity := req.GetCapacityRange().GetRequiredBytes()
	parameters := req.GetParameters()
	klog.Infof("[CreateVolume] mountPermissions: %v, reqCapacity: %d, parameters: %v", mountPermissions, reqCapacity, parameters)
	if parameters == nil {
		parameters = make(map[string]string)
	}

	var server, scId, bkSize, provision, compress string
	// validate parameters (case-insensitive)
	for k, v := range parameters {
		switch strings.ToLower(k) {
		case paramServer:
			server = v
		case paramDatastoreId:
			scId = v
		case paramBlockSize:
			bkSize = v
		case paramProvision:
			provision = v
		case paramCompress:
			compress = v
		case mountPermissionsField:
			if v != "" {
				var err error
				if mountPermissions, err = strconv.ParseUint(v, 8, 32); err != nil {
					return nil, status.Errorf(codes.InvalidArgument, fmt.Sprintf("invalid mountPermissions %s in storage class", v))
				}
			}
		default:
			// return nil, status.Errorf(codes.InvalidArgument, fmt.Sprintf("invalid parameter %q in storage class", k))
			klog.Warningf("[CreateVolume] invalid parameter %q in storage class", k)
		}
	}

	if server == "" {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("%v is a required parameter", paramServer))
	}
	if scId == "" {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("%v is a required parameter", paramDatastoreId))
	}

	klog.Infof("[CreateVolume] parameters: server(%s), scId(%s), mountPermissions(%v)", server, scId, mountPermissions)

	volSizeMB := reqCapacity >> 20
	block, _ := strconv.ParseUint(bkSize, 10, 32)
	authClient, err := cs.Driver.qsan.GetnAddAuthClient(ctx, server)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	volumeAPI := goqsm.NewVolume(authClient)
	vol, err := volumeAPI.CreateVolume(ctx, scId, name, uint64(volSizeMB), &goqsm.VolumeCreateOptions{
		BlockSize: uint(block),
		Provision: provision,
		Compress:  compress,
		Dedup:     false,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	volumeContextID := cs.Driver.GenerateVolumeContextID(server, scId, vol.ID)
	context := map[string]string{
		paramServer:      server,
		paramDatastoreId: scId,
		paramShare:       "/nfs-share/" + name,
	}

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      volumeContextID,
			CapacityBytes: int64(vol.SizeMB) << 20,
			VolumeContext: context,
		},
	}, nil
}

// DeleteVolume delete a volume
func (cs *ControllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}

	volData, err := cs.Driver.GetContextDataFromVolumeContextID(volumeID)
	klog.Infof("[DeleteVolume] volData: %+v", volData)
	if err != nil {
		klog.Warningf("[DeleteVolume] failed to get volume context data for volume id %v deletion: %v", volumeID, err)
		return &csi.DeleteVolumeResponse{}, nil
	}

	authClient, err := cs.Driver.qsan.GetnAddAuthClient(ctx, volData.server)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	volumeAPI := goqsm.NewVolume(authClient)
	err = volumeAPI.DeleteVolume(ctx, volData.scId, volData.volId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	klog.Infof("[DeleteVolume] Delete volumeID %s successfully", volumeID)

	return &csi.DeleteVolumeResponse{}, nil
}

func (cs *ControllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *ControllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *ControllerServer) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *ControllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if err := isValidVolumeCapabilities(req.GetVolumeCapabilities()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: req.GetVolumeCapabilities(),
		},
		Message: "",
	}, nil
}

func (cs *ControllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *ControllerServer) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerGetCapabilities implements the default GRPC callout.
// Default supports all capabilities
func (cs *ControllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: cs.Driver.cscap,
	}, nil
}

func (cs *ControllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *ControllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *ControllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *ControllerServer) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// isValidVolumeCapabilities validates the given VolumeCapability array is valid
func isValidVolumeCapabilities(volCaps []*csi.VolumeCapability) error {
	if len(volCaps) == 0 {
		return fmt.Errorf("volume capabilities missing in request")
	}
	for _, c := range volCaps {
		if c.GetBlock() != nil {
			return fmt.Errorf("block volume capability not supported")
		}

	}
	return nil
}
