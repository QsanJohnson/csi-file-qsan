// @2022 QSAN Inc. All rights reserved

package goqsm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

// VolumeOp handles volume related methods of the QSM storage.
type VolumeOp struct {
	client *AuthClient
}

// The response data of volume related methods, ex ListVolumes and CreateVolume.
type VolumeData struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	IntName string `json:"intName"`
	NaaID   string `json:"naaId"`
	VMPath  string `json:"vmPath"`
	SizeMB  uint64 `json:"sizeMB"`
	UsedMB  uint64 `json:"usedMB"`
}

type VolumeCreateOptions struct {
	BlockSize uint   // recordsize: 1024, 2048 ..., 65536
	Provision string // "thin" or "thick"
	Compress  string // "on", "off", "genericzero", "empty" or "lz4"
	Dedup     bool   // true: enable dedup, otherwise disable
}

// NewVolume returns volume operation
func NewVolume(client *AuthClient) *VolumeOp {
	return &VolumeOp{client}
}

// ListVolumes list all volumes or a dedicated volume with volId
func (v *VolumeOp) ListVolumes(ctx context.Context, scId, volId string) (*[]VolumeData, error) {
	req, err := v.client.NewRequest(ctx, http.MethodGet, "/rest/internal/cloud/containers/"+scId+"/vols/"+volId, nil)

	if err != nil {
		return nil, err
	}

	res := []VolumeData{}
	if err := v.client.sendRequest(ctx, req, &res); err != nil {
		return nil, err
	}

	return &res, nil
}

// CreateVolume create a volume on a storage container
func (v *VolumeOp) CreateVolume(ctx context.Context, scId, name string, size uint64, options *VolumeCreateOptions) (*VolumeData, error) {
	params := url.Values{}
	params.Add("name", name)
	params.Add("sizeMB", strconv.FormatUint(size, 10))

	var optionMap map[string]interface{}
	data, _ := json.Marshal(options)
	json.Unmarshal(data, &optionMap)
	// 'int' type will become 'float64' type after struct to map[string]interface{} conversion
	if optionMap["BlockSize"] != float64(0) {
		v := uint64(optionMap["BlockSize"].(float64))
		params.Add("blockSize", strconv.FormatUint(v, 10))
	}
	if optionMap["Provision"] != "" {
		params.Add("provision", optionMap["Provision"].(string))
	}
	if optionMap["Compress"] != "" {
		params.Add("compress", optionMap["Compress"].(string))
	}
	if optionMap["Dedup"] == true {
		params.Add("dedup", "on")
	}

	req, err := v.client.NewRequest(ctx, http.MethodPost, "/rest/internal/cloud/containers/"+scId+"/vols", params)
	if err != nil {
		return nil, err
	}

	res := VolumeData{}
	if err := v.client.sendRequest(ctx, req, &res); err != nil {
		return nil, err
	}

	return &res, nil
}

// DeleteVolume delete a volume from a storage container
func (v *VolumeOp) DeleteVolume(ctx context.Context, scId, volId string) error {
	req, err := v.client.NewRequest(ctx, http.MethodDelete, "/rest/internal/cloud/containers/"+scId+"/vols/"+volId, nil)
	if err != nil {
		return err
	}

	res := EmptyData{}
	if err := v.client.sendRequest(ctx, req, &res); err != nil {
		return err
	}

	return nil
}

// ExportVolume export a NFS volume
func (v *VolumeOp) ExportVolume(ctx context.Context, scId, volId string) error {
	req, err := v.client.NewRequest(ctx, http.MethodPost, "/rest/internal/cloud/containers/"+scId+"/vols/"+volId+"/share", nil)
	if err != nil {
		return err
	}

	res := EmptyData{}
	if err := v.client.sendRequest(ctx, req, &res); err != nil {
		return err
	}

	return nil
}

// UnexportVolume unexport a NFS volume
func (v *VolumeOp) UnexportVolume(ctx context.Context, scId, volId string) error {
	req, err := v.client.NewRequest(ctx, http.MethodDelete, "/rest/internal/cloud/containers/"+scId+"/vols/"+volId+"/share", nil)
	if err != nil {
		return err
	}

	res := EmptyData{}
	if err := v.client.sendRequest(ctx, req, &res); err != nil {
		return err
	}

	return nil
}
