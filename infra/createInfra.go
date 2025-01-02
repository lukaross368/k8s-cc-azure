package infra

import (
	"fmt"
	"log"
	"os"

	"github.com/lukaross368/k8s-cc-azure/config"
	"github.com/lukaross368/k8s-cc-azure/loggers"
	"github.com/pulumi/pulumi-azure-native-sdk/network/v2"
	"github.com/pulumi/pulumi-azure-native/sdk/go/azure/compute"
	"github.com/pulumi/pulumi-azure-native/sdk/go/azure/resources"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func readSSHKey(sshKeyPath string) (string, error) {
	loggers.LogVerbose("Reading SSH Key data from path: %s", sshKeyPath)
	data, err := os.ReadFile(sshKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read SSH key from %s: %w", sshKeyPath, err)
	} else {
		loggers.LogVerbose("Read SSH Key Data")
	}
	return string(data), nil
}

func ReturnPulumiFunction(config config.Config) func(ctx *pulumi.Context) error {
	program := func(ctx *pulumi.Context) error {

		tags := pulumi.StringMap{
			"deployed_with": pulumi.String("pulumi"),
			"project":       pulumi.String(config.ProjectName),
			"environment":   pulumi.String(config.Environment),
		}

		resourceGroupName := fmt.Sprintf("k8s-%s-rg", config.Environment)
		resourceGroup, err := resources.NewResourceGroup(ctx, resourceGroupName, &resources.ResourceGroupArgs{
			ResourceGroupName: pulumi.String(resourceGroupName),
			Location:          pulumi.String(config.Region),
			Tags:              tags,
		})

		if err != nil {
			log.Fatalf("error creating resource group: %v", err)
		} else {
			loggers.LogVerbose("Successfully created resource group")
		}

		vnet, err := network.NewVirtualNetwork(ctx, "kubernetes-vnet", &network.VirtualNetworkArgs{
			ResourceGroupName:  resourceGroup.Name,
			Location:           resourceGroup.Location,
			VirtualNetworkName: pulumi.String("kubernetes-vnet"),
			Tags:               tags,
			AddressSpace: &network.AddressSpaceArgs{
				AddressPrefixes: pulumi.StringArray{
					pulumi.String(config.Networking.AddressPrefix),
				},
			},
		})

		if err != nil {
			log.Fatalf("error creating virtual network: %v", err)
		} else {
			loggers.LogVerbose("Successfully created virtual network")
		}

		networkSecurityGroup, err := network.NewNetworkSecurityGroup(ctx, "kubernetes-nsg", &network.NetworkSecurityGroupArgs{
			Location:                 resourceGroup.Location,
			ResourceGroupName:        resourceGroup.Name,
			NetworkSecurityGroupName: pulumi.String("kubernetes-nsg"),
			Tags:                     tags,
		})

		if err != nil {
			log.Fatalf("error creating nsg: %v", err)
		} else {
			loggers.LogVerbose("Successfully created nsg")
		}

		allowSSH, err := network.NewSecurityRule(ctx, "kubernetesAllowSSH", &network.SecurityRuleArgs{
			Access:                   pulumi.String("Allow"),
			DestinationAddressPrefix: pulumi.String("*"),
			DestinationPortRange:     pulumi.String("22"),
			Direction:                pulumi.String("Inbound"),
			Priority:                 pulumi.Int(1000),
			Protocol:                 pulumi.String("Tcp"),
			ResourceGroupName:        resourceGroup.Name,
			NetworkSecurityGroupName: networkSecurityGroup.Name,
			SourceAddressPrefix:      pulumi.String("*"),
			SourcePortRange:          pulumi.String("*"),
		})

		if err != nil {
			log.Fatalf("error creating %v rule: %v", allowSSH.Name, err)
		}

		allowApi, err := network.NewSecurityRule(ctx, "kubernetesAllowAPIServer", &network.SecurityRuleArgs{
			Access:                   pulumi.String("Allow"),
			DestinationAddressPrefix: pulumi.String("*"),
			DestinationPortRange:     pulumi.String("6443"),
			Direction:                pulumi.String("Inbound"),
			Priority:                 pulumi.Int(1001),
			Protocol:                 pulumi.String("Tcp"),
			ResourceGroupName:        resourceGroup.Name,
			NetworkSecurityGroupName: networkSecurityGroup.Name,
			SourceAddressPrefix:      pulumi.String("*"),
			SourcePortRange:          pulumi.String("*"),
		})

		if err != nil {
			log.Fatalf("error creating %v rule: %v", allowApi.Name, err)
		}

		subnet, err := network.NewSubnet(ctx, "kubernetes-subnet", &network.SubnetArgs{
			ResourceGroupName:  resourceGroup.Name,
			VirtualNetworkName: vnet.Name,
			SubnetName:         pulumi.String("kubernetes-subnet"),
			AddressPrefix:      pulumi.String(config.Networking.SubnetCIDR),
			NetworkSecurityGroup: &network.NetworkSecurityGroupTypeArgs{
				Id: networkSecurityGroup.ID(),
			},
		})

		if err != nil {
			log.Fatalf("error creating subnet: %v", err)
		} else {
			loggers.LogVerbose("Successfully created subnet")
		}

		publicIP, err := network.NewPublicIPAddress(ctx, "kubernetes-pip", &network.PublicIPAddressArgs{
			ResourceGroupName:        resourceGroup.Name,
			Location:                 resourceGroup.Location,
			PublicIpAddressName:      pulumi.String("kubernetes-pip"),
			PublicIPAllocationMethod: pulumi.String("Static"),
			PublicIPAddressVersion:   pulumi.String("IPv4"),
			Tags:                     tags,
			Sku: &network.PublicIPAddressSkuArgs{
				Name: pulumi.String("Standard"),
			},
		})

		if err != nil {
			log.Fatalf("error creating public IP address: %v", err)
		} else {
			loggers.LogVerbose("Successfully created public IP address")
		}

		lb, err := network.NewLoadBalancer(ctx, "kubernetes-lb", &network.LoadBalancerArgs{
			ResourceGroupName: resourceGroup.Name,
			Location:          resourceGroup.Location,
			LoadBalancerName:  pulumi.String("kubernetes-lb"),
			Tags:              tags,
			Sku: &network.LoadBalancerSkuArgs{
				Name: pulumi.String("Standard"),
			},

			FrontendIPConfigurations: network.FrontendIPConfigurationArray{
				&network.FrontendIPConfigurationArgs{
					Name: pulumi.String("kubernetes-pip-config"),
					PublicIPAddress: &network.PublicIPAddressTypeArgs{
						Id: publicIP.ID(),
					},
				},
			},

			BackendAddressPools: network.BackendAddressPoolArray{
				&network.BackendAddressPoolArgs{
					Name: pulumi.String("kubernetes-lb-pool"),
				},
			},
		})

		if err != nil {
			log.Fatalf("error creating load balancer: %v", err)
		} else {
			loggers.LogVerbose("Successfully created Load Balancer")
		}

		availabilitySet, err := compute.NewAvailabilitySet(ctx, "kubernetes-as", &compute.AvailabilitySetArgs{
			ResourceGroupName: resourceGroup.Name,
			Location:          resourceGroup.Location,
			Sku: &compute.SkuArgs{
				Name: pulumi.String("Aligned"),
			},
			PlatformFaultDomainCount:  pulumi.Int(2),
			PlatformUpdateDomainCount: pulumi.Int(5),
			AvailabilitySetName:       pulumi.String("kubernetes-as"),
			Tags:                      tags,
		})

		if err != nil {
			log.Fatalf("error creating availability set : %v", err)
		} else {
			loggers.LogVerbose("Successfully created availability set ")
		}

		loggers.LogVerbose("running createControllers function")

		err = CreateControllers(ctx, config, resourceGroup, subnet, lb, availabilitySet, tags)

		if err != nil {
			log.Fatalf("error creating control plane machines : %v", err)
		} else {
			loggers.LogVerbose("Successfully created control plane vms")
		}

		loggers.LogVerbose("running createWorkers function")

		err = CreateWorkers(ctx, config, resourceGroup, subnet, lb, availabilitySet, tags)

		if err != nil {
			log.Fatalf("error creating worker machines : %v", err)
		} else {
			loggers.LogVerbose("Successfully created worker vms")
		}

		return err
	}
	return program
}

func CreateControllers(ctx *pulumi.Context, config config.Config, resourceGroup *resources.ResourceGroup, subnet *network.Subnet, lb *network.LoadBalancer, availabilitySet *compute.AvailabilitySet, tags pulumi.StringMap) error {

	sshKey, err := readSSHKey(config.ControlPlane.SshKeyPath)
	if err != nil {
		return fmt.Errorf("error reading SSH key: %w", err)
	}

	for i := 0; i < config.ControlPlane.Nodes; i++ {

		fmt.Printf("creating controller-%d-pip", i)

		publicIP, err := network.NewPublicIPAddress(ctx, fmt.Sprintf("controller-%d-pip", i), &network.PublicIPAddressArgs{
			ResourceGroupName:   resourceGroup.Name,
			Location:            resourceGroup.Location,
			PublicIpAddressName: pulumi.String(fmt.Sprintf("controller-%d-pip", i)),
			Tags:                tags,
			Sku: &network.PublicIPAddressSkuArgs{
				Name: pulumi.String("Standard"),
			},
			PublicIPAllocationMethod: pulumi.String("Static"),
		})
		if err != nil {
			return fmt.Errorf("error creating public IP for controller-%d: %w", i, err)
		}

		fmt.Printf("creating controller-%d-nic", i)

		nic, err := network.NewNetworkInterface(ctx, fmt.Sprintf("controller-%d-nic", i), &network.NetworkInterfaceArgs{
			ResourceGroupName:    resourceGroup.Name,
			Location:             resourceGroup.Location,
			EnableIPForwarding:   pulumi.Bool(true),
			NetworkInterfaceName: pulumi.String(fmt.Sprintf("controller-%d-nic", i)),
			Tags:                 tags,
			IpConfigurations: network.NetworkInterfaceIPConfigurationArray{
				&network.NetworkInterfaceIPConfigurationArgs{
					Name:                      pulumi.String(fmt.Sprintf("ipconfig-%d", i)),
					PrivateIPAddress:          pulumi.String(fmt.Sprintf("10.0.0.1%d", i)),
					PrivateIPAllocationMethod: pulumi.String("Static"),
					PublicIPAddress: &network.PublicIPAddressTypeArgs{
						Id: publicIP.ID(),
					},
					Subnet: &network.SubnetTypeArgs{
						Id: subnet.ID(),
					},
					LoadBalancerBackendAddressPools: network.BackendAddressPoolArray{
						&network.BackendAddressPoolArgs{
							Id: lb.BackendAddressPools.Index(pulumi.Int(0)).Id(),
						},
					},
				},
			},
		})

		if err != nil {
			return fmt.Errorf("error creating NIC for controller-%d: %w", i, err)
		}

		fmt.Printf("creating controller-%d-vm", i)

		_, err = compute.NewVirtualMachine(ctx, fmt.Sprintf("controller-%d", i), &compute.VirtualMachineArgs{
			ResourceGroupName: resourceGroup.Name,
			Location:          resourceGroup.Location,
			VmName:            pulumi.String(fmt.Sprintf("controller-%d", i)),
			HardwareProfile: &compute.HardwareProfileArgs{
				VmSize: pulumi.String(config.ControlPlane.ControllerVMSize),
			},
			OsProfile: &compute.OSProfileArgs{
				AdminUsername: pulumi.String(config.ControlPlane.ControllerNodeVMAdminUser),
				ComputerName:  pulumi.String(fmt.Sprintf("controller-%d", i)),
				LinuxConfiguration: &compute.LinuxConfigurationArgs{
					DisablePasswordAuthentication: pulumi.Bool(true),
					Ssh: &compute.SshConfigurationArgs{
						PublicKeys: compute.SshPublicKeyTypeArray{
							compute.SshPublicKeyTypeArgs{
								KeyData: pulumi.String(sshKey),
								Path:    pulumi.String("/home/kuberoot/.ssh/authorized_keys"),
							},
						},
					},
				},
			},
			StorageProfile: &compute.StorageProfileArgs{
				ImageReference: &compute.ImageReferenceArgs{
					Publisher: pulumi.String("Canonical"),
					Offer:     pulumi.String("UbuntuServer"),
					Sku:       pulumi.String(config.ControlPlane.ControllerNodeMachineImageSku),
					Version:   pulumi.String("latest"),
				},
				OsDisk: &compute.OSDiskArgs{
					CreateOption: pulumi.String("FromImage"),
					DiskSizeGB:   pulumi.Int(config.ControlPlane.ControllerNodeDiskSize),
					ManagedDisk: &compute.ManagedDiskParametersArgs{
						StorageAccountType: pulumi.String(config.ControlPlane.ControllerNodeDiskStorageAccountType),
					},
				},
			},
			NetworkProfile: &compute.NetworkProfileArgs{
				NetworkInterfaces: compute.NetworkInterfaceReferenceArray{
					&compute.NetworkInterfaceReferenceArgs{
						Id: nic.ID(),
					},
				},
			},
			AvailabilitySet: &compute.SubResourceArgs{
				Id: availabilitySet.ID(),
			},
		})

		if err != nil {
			return fmt.Errorf("error creating VM for controller-%d: %w", i, err)
		}

	}
	return nil
}

func CreateWorkers(ctx *pulumi.Context, config config.Config, resourceGroup *resources.ResourceGroup, subnet *network.Subnet, lb *network.LoadBalancer, availabilitySet *compute.AvailabilitySet, tags pulumi.StringMap) error {

	sshKey, err := readSSHKey(config.Workers.SshKeyPath)
	if err != nil {
		return fmt.Errorf("error reading SSH key: %w", err)
	}

	for i := 0; i < config.Workers.Nodes; i++ {

		fmt.Printf("creating worker-%d-pip", i)

		publicIP, err := network.NewPublicIPAddress(ctx, fmt.Sprintf("worker-%d-pip", i), &network.PublicIPAddressArgs{
			ResourceGroupName:   resourceGroup.Name,
			Location:            resourceGroup.Location,
			PublicIpAddressName: pulumi.String(fmt.Sprintf("worker-%d-pip", i)),
			Tags:                tags,
			Sku: &network.PublicIPAddressSkuArgs{
				Name: pulumi.String("Standard"),
			},
			PublicIPAllocationMethod: pulumi.String("Static"),
		})
		if err != nil {
			return fmt.Errorf("error creating public IP for worker-%d: %w", i, err)
		}

		fmt.Printf("creating worker-%d-nic", i)

		nic, err := network.NewNetworkInterface(ctx, fmt.Sprintf("worker-%d-nic", i), &network.NetworkInterfaceArgs{
			ResourceGroupName:    resourceGroup.Name,
			Location:             resourceGroup.Location,
			EnableIPForwarding:   pulumi.Bool(true),
			NetworkInterfaceName: pulumi.String(fmt.Sprintf("worker-%d-nic", i)),
			Tags:                 tags,
			IpConfigurations: network.NetworkInterfaceIPConfigurationArray{
				&network.NetworkInterfaceIPConfigurationArgs{
					Name:                      pulumi.String(fmt.Sprintf("ipconfig-%d", i)),
					PrivateIPAddress:          pulumi.String(fmt.Sprintf("10.0.0.2%d", i)),
					PrivateIPAllocationMethod: pulumi.String("Static"),
					PublicIPAddress: &network.PublicIPAddressTypeArgs{
						Id: publicIP.ID(),
					},
					Subnet: &network.SubnetTypeArgs{
						Id: subnet.ID(),
					},
					LoadBalancerBackendAddressPools: network.BackendAddressPoolArray{
						&network.BackendAddressPoolArgs{
							Id: lb.BackendAddressPools.Index(pulumi.Int(0)).Id(),
						},
					},
				},
			},
		})

		if err != nil {
			return fmt.Errorf("error creating NIC for worker-%d: %w", i, err)
		}

		fmt.Printf("creating worker-%d-vm", i)

		_, err = compute.NewVirtualMachine(ctx, fmt.Sprintf("worker-%d", i), &compute.VirtualMachineArgs{
			ResourceGroupName: resourceGroup.Name,
			Location:          resourceGroup.Location,
			VmName:            pulumi.String(fmt.Sprintf("worker-%d", i)),
			HardwareProfile: &compute.HardwareProfileArgs{
				VmSize: pulumi.String(config.Workers.ControllerVMSize),
			},
			OsProfile: &compute.OSProfileArgs{
				AdminUsername: pulumi.String(config.Workers.ControllerNodeVMAdminUser),
				ComputerName:  pulumi.String(fmt.Sprintf("worker-%d", i)),
				LinuxConfiguration: &compute.LinuxConfigurationArgs{
					DisablePasswordAuthentication: pulumi.Bool(true),
					Ssh: &compute.SshConfigurationArgs{
						PublicKeys: compute.SshPublicKeyTypeArray{
							compute.SshPublicKeyTypeArgs{
								KeyData: pulumi.String(sshKey),
								Path:    pulumi.String("/home/kuberoot/.ssh/authorized_keys"),
							},
						},
					},
				},
			},
			StorageProfile: &compute.StorageProfileArgs{
				ImageReference: &compute.ImageReferenceArgs{
					Publisher: pulumi.String("Canonical"),
					Offer:     pulumi.String("UbuntuServer"),
					Sku:       pulumi.String(config.Workers.ControllerNodeMachineImageSku),
					Version:   pulumi.String("latest"),
				},
				OsDisk: &compute.OSDiskArgs{
					CreateOption: pulumi.String("FromImage"),
					DiskSizeGB:   pulumi.Int(config.Workers.ControllerNodeDiskSize),
					ManagedDisk: &compute.ManagedDiskParametersArgs{
						StorageAccountType: pulumi.String(config.Workers.ControllerNodeDiskStorageAccountType),
					},
				},
			},
			NetworkProfile: &compute.NetworkProfileArgs{
				NetworkInterfaces: compute.NetworkInterfaceReferenceArray{
					&compute.NetworkInterfaceReferenceArgs{
						Id: nic.ID(),
					},
				},
			},
			AvailabilitySet: &compute.SubResourceArgs{
				Id: availabilitySet.ID(),
			},
		})

		if err != nil {
			return fmt.Errorf("error creating VM for worker-%d: %w", i, err)
		}

	}
	return nil
}
