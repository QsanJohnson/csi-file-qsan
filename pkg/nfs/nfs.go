package nfs

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"k8s.io/klog/v2"
	mount "k8s.io/mount-utils"
)

type volumeContextData struct {
	server   string
	username string
	password string
	scId     string
	volId    string
}

// DriverOptions defines driver parameters specified in driver deployment
type DriverOptions struct {
	NodeID            string
	DriverName        string
	Endpoint          string
	MountPermissions  uint64
	WorkingMountDir   string
	MaxVolumesPerNode int64
	SecretFile        string
}

type Driver struct {
	name              string
	nodeID            string
	version           string
	endpoint          string
	mountPermissions  uint64
	workingMountDir   string
	maxVolumesPerNode int64

	//ids *identityServer
	ns          *NodeServer
	cscap       []*csi.ControllerServiceCapability
	nscap       []*csi.NodeServiceCapability
	volumeLocks *VolumeLocks
	// secrets     *StorageArraySecrets
	// authClient  map[string]*goqsm.AuthClient
	qsan *QsanClient
}

const (
	DefaultDriverName = "file.csi.qsan.com"
	// Address of the NFS server
	paramServer = "server"
	// Base directory of the NFS server to create volumes under.
	// The base directory must be a direct child of the root directory.
	// The root directory is omitted from the string, for example:
	//     "base" instead of "/base"
	paramShare            = "share"
	paramDatastoreId      = "datastoreid"
	paramBlockSize        = "blocksize"
	paramProvision        = "provision"
	paramCompress         = "compress"
	mountOptionsField     = "mountoptions"
	mountPermissionsField = "mountpermissions"
)

func NewDriver(options *DriverOptions) *Driver {
	klog.Infof("Driver: %v version: %v (build %v)", options.DriverName, driverVersion, buildDate)

	n := &Driver{
		name:              options.DriverName,
		version:           driverVersion,
		nodeID:            options.NodeID,
		endpoint:          options.Endpoint,
		mountPermissions:  options.MountPermissions,
		workingMountDir:   options.WorkingMountDir,
		maxVolumesPerNode: options.MaxVolumesPerNode,
		qsan:              NewQsanClient(options.SecretFile),
	}

	n.AddControllerServiceCapabilities([]csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
		// csi.ControllerServiceCapability_RPC_GET_VOLUME,
		// csi.ControllerServiceCapability_RPC_GET_CAPACITY,
		// csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
		// csi.ControllerServiceCapability_RPC_VOLUME_CONDITION,
		// csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
	})

	n.AddNodeServiceCapabilities([]csi.NodeServiceCapability_RPC_Type{
		csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
		csi.NodeServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
		csi.NodeServiceCapability_RPC_UNKNOWN,
		// csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
	})
	n.volumeLocks = NewVolumeLocks()
	return n
}

func NewNodeServer(n *Driver, mounter mount.Interface) *NodeServer {
	return &NodeServer{
		Driver:  n,
		mounter: mounter,
	}
}

func (n *Driver) Run(testMode bool) {
	versionMeta, err := GetVersionYAML(n.name)
	if err != nil {
		klog.Fatalf("%v", err)
	}
	klog.V(2).Infof("\nDRIVER INFORMATION:\n-------------------\n%s\n\nStreaming logs below:", versionMeta)

	n.ns = NewNodeServer(n, mount.New(""))
	s := NewNonBlockingGRPCServer()
	s.Start(n.endpoint,
		NewDefaultIdentityServer(n),
		// NFS plugin has not implemented ControllerServer
		// using default controllerserver.
		NewControllerServer(n),
		n.ns,
		testMode)
	s.Wait()
}

func (n *Driver) AddControllerServiceCapabilities(cl []csi.ControllerServiceCapability_RPC_Type) {
	var csc []*csi.ControllerServiceCapability
	for _, c := range cl {
		csc = append(csc, NewControllerServiceCapability(c))
	}
	n.cscap = csc
}

func (n *Driver) AddNodeServiceCapabilities(nl []csi.NodeServiceCapability_RPC_Type) {
	var nsc []*csi.NodeServiceCapability
	for _, n := range nl {
		nsc = append(nsc, NewNodeServiceCapability(n))
	}
	n.nscap = nsc
}

func (n *Driver) GenerateVolumeContextID(ipv4, dsId, volId string) string {
	return fmt.Sprintf("%s-%s-%s", ipv4ToHexStr(ipv4), dsId, volId)
}

func (n *Driver) GetContextDataFromVolumeContextID(id string) (*volumeContextData, error) {
	tokens := strings.Split(id, "-")
	if len(tokens) != 3 {
		return nil, fmt.Errorf("Format error with volume context ID %s ", id)
	}

	a, _ := hex.DecodeString(tokens[0])
	ipv4 := fmt.Sprintf("%d.%d.%d.%d", a[0], a[1], a[2], a[3])

	var username, password string
	for _, client := range n.qsan.secrets.StorageArraySecrets {
		if client.Server == ipv4 {
			username, password = client.Username, client.Password
			break
		}
	}
	if username == "" && password == "" {
		return nil, fmt.Errorf("Can not get secret from server %s", ipv4)
	}

	return &volumeContextData{
		server:   ipv4,
		scId:     tokens[1],
		volId:    tokens[2],
		username: username,
		password: password,
	}, nil
}

func ipv4ToHexStr(ipv4 string) string {
	r := `^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})`
	reg, err := regexp.Compile(r)
	if err != nil {
		return ""
	}
	ips := reg.FindStringSubmatch(ipv4)
	if ips == nil {
		return ""
	}
	ip1, _ := strconv.Atoi(ips[1])
	ip2, _ := strconv.Atoi(ips[2])
	ip3, _ := strconv.Atoi(ips[3])
	ip4, _ := strconv.Atoi(ips[4])
	buf := []byte{byte(ip1), byte(ip2), byte(ip3), byte(ip4)}
	return hex.EncodeToString(buf)
}

func hexStrToIPv4(s string) string {
	b, err := hex.DecodeString(s)
	if err != nil {
		fmt.Println("HexStrToIPv4 failed:", s)
	}
	return fmt.Sprintf("%d.%d.%d.%d", b[0], b[1], b[2], b[3])
}

func IsCorruptedDir(dir string) bool {
	_, pathErr := mount.PathExists(dir)
	return pathErr != nil && mount.IsCorruptedMnt(pathErr)
}
