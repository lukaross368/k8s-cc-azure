package config

import (
	"io"
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	ProjectName string `yaml:"projectName"`
	Environment string `yaml:"environment"`
	Region      string `yaml:"region"`
	DryRun      bool   `yaml:"dryRun"`

	Networking struct {
		AddressPrefix string `yaml:"addressPrefix"`
		SubnetCIDR    string `yaml:"subnetCIDR"`
	}

	ControlPlane struct {
		Nodes                                int    `yaml:"nodes"`
		ControllerVMSize                     string `yaml:"controllerVMSize"`
		SshKeyPath                           string `yaml:"sshKeyPath"`
		ControllerNodeDiskSize               int    `yaml:"controllerNodeDiskSize"`
		ControllerNodeDiskStorageAccountType string `yaml:"controllerNodeDiskStorageAccountType"`
		ControllerNodeMachineImageSku        string `yaml:"controllerNodeMachineImageSku"`
		ControllerNodeVMAdminUser            string `yaml:"controllerNodeVMAdminUser"`
	} `yaml:"controlPlane"`

	Workers struct {
		Nodes                                int    `yaml:"nodes"`
		ControllerVMSize                     string `yaml:"workerVMSize"`
		SshKeyPath                           string `yaml:"sshKeyPath"`
		ControllerNodeDiskSize               int    `yaml:"workerNodeDiskSize"`
		ControllerNodeDiskStorageAccountType string `yaml:"workerNodeDiskStorageAccountType"`
		ControllerNodeMachineImageSku        string `yaml:"workerNodeMachineImageSku"`
		ControllerNodeVMAdminUser            string `yaml:"workerNodeVMAdminUser"`
	} `yaml:"workers"`
}

func LoadConfig(filename string) (*Config, error) {
	var config Config

	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(bytes, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
