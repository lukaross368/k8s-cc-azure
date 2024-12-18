package infra

import (
	"fmt"
	"log"
	"os"

	"github.com/pulumi/pulumi-azure-native-sdk/network/v2"
	"github.com/pulumi/pulumi-azure-native/sdk/go/azure/compute"
	"github.com/pulumi/pulumi-azure-native/sdk/go/azure/resources"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const projectName = "k8s-cc"
const environment = "dev"
const region = "uksouth"

const addressPrefix = "10.0.0.0/16"
const subnetCIDR = "10.0.0.0/24"

const controllerVMSize = "Standard_DS1_v2"
const sshKeyPath = "*****"
const controllerNodeDiskSize = 30
const controllerNodeDiskStorageAccountType = "Standard_LRS"
const controllerNodeMachineImageSku = "18.04-LTS"
const controllerNodeVMAdminUser = "kuberoot"

const controlNodes = 2
const workerNodes = 2

func readSSHKey() (string, error) {
	data, err := os.ReadFile(sshKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read SSH key from %s: %w", sshKeyPath, err)
	}
	return string(data), nil
}

func ReturnPulumiFunction() func(ctx *pulumi.Context) error {
	program := func(ctx *pulumi.Context) error {

		tags := pulumi.StringMap{
			"deployed_with": pulumi.String("pulumi"),
			"project":       pulumi.String(projectName),
			"environment":   pulumi.String(environment),
		}

		resourceGroup, err := resources.NewResourceGroup(ctx, "k8s-dev-rg", &resources.ResourceGroupArgs{
			ResourceGroupName: pulumi.String("k8s-dev-rg"),
			Location:          pulumi.String(region),
			Tags:              tags,
		})

		if err != nil {
			log.Fatalf("error creating resource group: %v", err)
		}

		vnet, err := network.NewVirtualNetwork(ctx, "kubernetes-vnet", &network.VirtualNetworkArgs{
			ResourceGroupName:  resourceGroup.Name,
			Location:           resourceGroup.Location,
			VirtualNetworkName: pulumi.String("kubernetes-vnet"),
			Tags:               tags,
			AddressSpace: &network.AddressSpaceArgs{
				AddressPrefixes: pulumi.StringArray{
					pulumi.String(addressPrefix),
				},
			},
		})

		if err != nil {
			log.Fatalf("error creating virtual network: %v", err)
		}

		networkSecurityGroup, err := network.NewNetworkSecurityGroup(ctx, "kubernetes-nsg", &network.NetworkSecurityGroupArgs{
			Location:                 resourceGroup.Location,
			ResourceGroupName:        resourceGroup.Name,
			NetworkSecurityGroupName: pulumi.String("kubernetes-nsg"),
			Tags:                     tags,
		})

		if err != nil {
			log.Fatalf("error creating nsg: %v", err)
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
			AddressPrefix:      pulumi.String(subnetCIDR),
			NetworkSecurityGroup: &network.NetworkSecurityGroupTypeArgs{
				Id: networkSecurityGroup.ID(),
			},
		})

		if err != nil {
			log.Fatalf("error creating subnet: %v", err)
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
		}

		availabilitySet, err := compute.NewAvailabilitySet(ctx, "kubernetes-as", &compute.AvailabilitySetArgs{
			ResourceGroupName:   resourceGroup.Name,
			Location:            resourceGroup.Location,
			AvailabilitySetName: pulumi.String("kubernetes-as"),
			Tags:                tags,
		})

		if err != nil {
			log.Fatalf("error creating availability set : %v", err)
		}

		log.Println("running createControllers function")

		err = CreateControllers(ctx, resourceGroup, subnet, lb, availabilitySet, controlNodes, tags)

		if err != nil {
			log.Fatalf("error creating control plane machines : %v", err)
		}

		return err
	}
	return program
}

func CreateControllers(ctx *pulumi.Context, resourceGroup *resources.ResourceGroup, subnet *network.Subnet, lb *network.LoadBalancer, availabilitySet *compute.AvailabilitySet, end int, tags pulumi.StringMap) error {

	sshKey, err := readSSHKey()
	if err != nil {
		return fmt.Errorf("error reading SSH key: %w", err)
	}

	for i := 0; i < end; i++ {

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
				VmSize: pulumi.String(controllerVMSize),
			},
			OsProfile: &compute.OSProfileArgs{
				AdminUsername: pulumi.String(controllerNodeVMAdminUser),
				ComputerName:  pulumi.String(fmt.Sprintf("controller-%d", i)),
				LinuxConfiguration: &compute.LinuxConfigurationArgs{
					DisablePasswordAuthentication: pulumi.Bool(false),
					Ssh: &compute.SshConfigurationArgs{
						PublicKeys: pulumi.ToOutput([]compute.SshPublicKeyTypeArgs{
							{
								KeyData: pulumi.String(sshKey),
								Path:    pulumi.String("/home/kuberoot/.ssh/authorized_keys"),
							},
						}).(compute.SshPublicKeyTypeArrayInput),
					},
				},
			},
			StorageProfile: &compute.StorageProfileArgs{
				ImageReference: &compute.ImageReferenceArgs{
					Publisher: pulumi.String("Canonical"),
					Offer:     pulumi.String("UbuntuServer"),
					Sku:       pulumi.String(controllerNodeMachineImageSku),
					Version:   pulumi.String("latest"),
				},
				OsDisk: &compute.OSDiskArgs{
					CreateOption: pulumi.String("FromImage"),
					DiskSizeGB:   pulumi.Int(controllerNodeDiskSize),
					ManagedDisk: &compute.ManagedDiskParametersArgs{
						StorageAccountType: pulumi.String(controllerNodeDiskStorageAccountType),
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
