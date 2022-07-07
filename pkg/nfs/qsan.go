package nfs

import (
	"io/ioutil"

	"github.com/QsanJohnson/goqsm"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"
)

type StorageArraySecret struct {
	Server   string `yaml:"server"`
	Port     int    `yaml:"port"`
	Https    string `yaml:"https"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type StorageArraySecrets struct {
	StorageArraySecrets []StorageArraySecret `yaml:"storageArrays"`
}

type QsanClient struct {
	secrets    *StorageArraySecrets
	authClient map[string]*goqsm.AuthClient
}

func NewQsanClient(secretFile string) *QsanClient {
	secretConfig, err := loadSecretFile(secretFile)
	if err != nil {
		klog.Errorf("Unable to open config file: %v", err)
	}

	return &QsanClient{
		secrets:    secretConfig,
		authClient: make(map[string]*goqsm.AuthClient),
	}
}

func loadSecretFile(secretFile string) (*StorageArraySecrets, error) {
	yamlFile, err := ioutil.ReadFile(secretFile)
	if err != nil {
		klog.Errorf("Unable to open secret file: %v", err)
		return nil, err
	}

	secretConfig := StorageArraySecrets{}
	err = yaml.Unmarshal(yamlFile, &secretConfig)
	if err != nil {
		klog.Errorf("Failed to parse secret: %v", err)
		return nil, err
	}

	// klog.Infof("[loadSecretFile] %s ==>\n%+v\n", secretFile, secretConfig)
	return &secretConfig, nil
}

func (c *QsanClient) GetSecret(ip string) (username, passwd string) {
	for _, client := range c.secrets.StorageArraySecrets {
		if client.Server == ip {
			return client.Username, client.Password
		}
	}

	return "", ""
}

func (c *QsanClient) GetnAddAuthClient(ctx context.Context, ip string) (*goqsm.AuthClient, error) {

	if authClient, ok := c.authClient[ip]; ok {
		return authClient, nil
	} else {
		username, password := c.GetSecret(ip)
		client := goqsm.NewClient(ip)
		klog.Infof("Add a Qsan client ip(%s) username(%s), password(%s)", ip, username, password)
		authClient, err := client.GetAuthClient(ctx, username, password)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		c.authClient[ip] = authClient
		return authClient, nil
	}
}
