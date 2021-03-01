package anxcloud

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/anexia-it/go-anxcloud/pkg/client"
	"github.com/anexia-it/go-anxcloud/pkg/vsphere"
	"github.com/anexia-it/go-anxcloud/pkg/vsphere/provisioning/vm"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/lithammer/shortuuid"
)

func TestAccAnxCloudVirtualServer(t *testing.T) {
	resourceName := "acc_test_vm_test"
	resourcePath := "anxcloud_virtual_server." + resourceName

	vmDef := vm.Definition{
		Location:     "52b5f6b2fd3a4a7eaaedf1a7c019e9ea",
		TemplateType: "templates",
		TemplateID:   "12c28aa7-604d-47e9-83fb-5f1d1f1837b3",
		Hostname:     "acc-test-" + shortuuid.New(),
		Memory:       2048,
		CPUs:         2,
		Disk:         50,
		Network: []vm.Network{
			{
				VLAN:    "02f39d20ca0f4adfb5032f88dbc26c39",
				NICType: "vmxnet3",
			},
		},
		DNS1:     "8.8.8.8",
		Password: "flatcar#1234$%%",
	}

	// upscale resources
	vmDefUpscale := vmDef
	vmDefUpscale.CPUs = 4
	vmDefUpscale.Disk = 80
	vmDefUpscale.Memory = 4096
	vmDefUpscale.Network = append(vmDefUpscale.Network, vm.Network{
		VLAN:    "02f39d20ca0f4adfb5032f88dbc26c39",
		NICType: "vmxnet3",
	})

	// down scale resources which does not require recreation of the VM
	vmDefDownscale := vmDefUpscale
	vmDefDownscale.CPUs = 2
	vmDefDownscale.Memory = 2096

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckAnxCloudVirtualServerDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccConfigAnxCloudVirtualServer(resourceName, &vmDef),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAnxCloudVirtualServerExists(resourcePath, &vmDef),
					resource.TestCheckResourceAttr(resourcePath, "location_id", vmDef.Location),
					resource.TestCheckResourceAttr(resourcePath, "template_id", vmDef.TemplateID),
					resource.TestCheckResourceAttr(resourcePath, "cpus", strconv.Itoa(vmDef.CPUs)),
					resource.TestCheckResourceAttr(resourcePath, "memory", strconv.Itoa(vmDef.Memory)),
					resource.TestCheckResourceAttr(resourcePath, "disk", strconv.Itoa(vmDef.Disk)),
				),
			},
			{
				Config: testAccConfigAnxCloudVirtualServer(resourceName, &vmDefUpscale),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAnxCloudVirtualServerExists(resourcePath, &vmDefUpscale),
					resource.TestCheckResourceAttr(resourcePath, "location_id", vmDefUpscale.Location),
					resource.TestCheckResourceAttr(resourcePath, "template_id", vmDefUpscale.TemplateID),
					resource.TestCheckResourceAttr(resourcePath, "cpus", strconv.Itoa(vmDefUpscale.CPUs)),
					resource.TestCheckResourceAttr(resourcePath, "memory", strconv.Itoa(vmDefUpscale.Memory)),
					resource.TestCheckResourceAttr(resourcePath, "disk", strconv.Itoa(vmDefUpscale.Disk)),
				),
			},
			{
				Config: testAccConfigAnxCloudVirtualServer(resourceName, &vmDefDownscale),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAnxCloudVirtualServerExists(resourcePath, &vmDefDownscale),
					resource.TestCheckResourceAttr(resourcePath, "location_id", vmDefDownscale.Location),
					resource.TestCheckResourceAttr(resourcePath, "template_id", vmDefDownscale.TemplateID),
					resource.TestCheckResourceAttr(resourcePath, "cpus", strconv.Itoa(vmDefDownscale.CPUs)),
					resource.TestCheckResourceAttr(resourcePath, "memory", strconv.Itoa(vmDefDownscale.Memory)),
					resource.TestCheckResourceAttr(resourcePath, "disk", strconv.Itoa(vmDefDownscale.Disk)),
				),
			},
		},
	})
}

func TestAccAnxCloudVirtualServerMultiDiskScaling(t *testing.T) {
	resourceName := "acc_test_vm_test_multi_disk"
	resourcePath := "anxcloud_virtual_server." + resourceName

	vmDef := vm.Definition{
		Location:     "52b5f6b2fd3a4a7eaaedf1a7c019e9ea",
		TemplateType: "templates",
		TemplateID:   "12c28aa7-604d-47e9-83fb-5f1d1f1837b3",
		Hostname:     "acc-test-" + shortuuid.New(),
		Memory:       2048,
		CPUs:         2,
		Network: []vm.Network{
			{
				VLAN:    "02f39d20ca0f4adfb5032f88dbc26c39",
				NICType: "vmxnet3",
			},
		},
		DNS1:     "8.8.8.8",
		Password: "flatcar#1234$%%",
	}

	disks := []vm.Disk{
		{
			Type:    "STD1",
			SizeGBs: 40,
		},
		{
			Type:    "STD1",
			SizeGBs: 40,
		},
	}

	disksUpscale := disks
	disksUpscale[1].Type = "ENT1"
	disksUpscale[1].SizeGBs = 50

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckAnxCloudVirtualServerDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccConfigAnxCloudVirtualServerMultiDiskSupport(resourceName, &vmDef, disks),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAnxCloudVirtualServerDisks(resourcePath, disks),
					resource.TestCheckResourceAttr(resourcePath, "disk", strconv.Itoa(vmDef.Disk)),
				),
			},
			{
				Config: testAccConfigAnxCloudVirtualServerMultiDiskSupport(resourceName, &vmDef, disksUpscale),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAnxCloudVirtualServerDisks(resourcePath, disksUpscale),
					resource.TestCheckResourceAttr(resourcePath, "disk", strconv.Itoa(vmDef.Disk)),
				),
			},
		},
	})
}

func testAccCheckAnxCloudVirtualServerDestroy(s *terraform.State) error {
	c := testAccProvider.Meta().(client.Client)
	v := vsphere.NewAPI(c)
	ctx := context.Background()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "anxcloud_virtual_server" {
			continue
		}

		if rs.Primary.ID == "" {
			return nil
		}

		info, err := v.Info().Get(ctx, rs.Primary.ID)
		if err != nil {
			if err := handleNotFoundError(err); err != nil {
				return err
			}
			return nil
		}
		if info.Identifier != "" {
			return fmt.Errorf("virtual machine '%s' exists", info.Identifier)
		}
	}

	return nil
}

func testAccConfigAnxCloudVirtualServer(resourceName string, def *vm.Definition) string {
	return fmt.Sprintf(`
	resource "anxcloud_virtual_server" "%s" {
		location_id   = "%s"
		template_id   = "%s"
		template_type = "%s"
		hostname      = "%s"
		cpus          = %d
		memory        = %d
		disk          = %d
		password      = "%s"

		// generated network string
		%s

		force_restart_if_needed = true
		critical_operation_confirmed = true
	}
	`, resourceName, def.Location, def.TemplateID, def.TemplateType, def.Hostname, def.CPUs, def.Memory, def.Disk,
		def.Password, generateNetworkSubResourceString(def.Network))
}

func testAccConfigAnxCloudVirtualServerMultiDiskSupport(resourceName string, def *vm.Definition, disks []vm.Disk) string {
	return fmt.Sprintf(`
	resource "anxcloud_virtual_server" "%s" {
		location_id   = "%s"
		template_id   = "%s"
		template_type = "%s"
		hostname      = "%s"
		cpus          = %d
		memory        = %d
		disk          = %d
		password      = "%s"

		// generated network string
		%s

		// generated disks string
		%s

		force_restart_if_needed = true
		critical_operation_confirmed = true
	}
	`, resourceName, def.Location, def.TemplateID, def.TemplateType, def.Hostname, def.CPUs, def.Memory, def.Disk,
		def.Password, generateNetworkSubResourceString(def.Network), generateDisksSubResourceString(disks))
}

func testAccCheckAnxCloudVirtualServerExists(n string, def *vm.Definition) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		c := testAccProvider.Meta().(client.Client)
		v := vsphere.NewAPI(c)
		ctx := context.Background()

		if !ok {
			return fmt.Errorf("virtual server not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("virtual server id not set")
		}

		info, err := v.Info().Get(ctx, rs.Primary.ID)
		if err != nil {
			return err
		}

		if info.Status != vmPoweredOn {
			return fmt.Errorf("virtual machine found but it is not in the expected state '%s': '%s'", vmPoweredOn, info.Status)
		}

		if info.CPU != def.CPUs {
			return fmt.Errorf("virtual machine cpu does not match, got %d - expected %d", info.CPU, def.CPUs)
		}
		if info.RAM != def.Memory {
			return fmt.Errorf("virtual machine memory does not match, got %d - expected %d", info.RAM, def.Memory)
		}

		if len(info.DiskInfo) != 1 {
			return fmt.Errorf("unspported number of attached disks, got %d - expected 1", len(info.DiskInfo))
		}
		if infoDiskGB := info.DiskInfo[0].DiskGB; infoDiskGB != def.Disk {
			return fmt.Errorf("virtual machine disk size does not match, got %d - expected %d", infoDiskGB, def.Disk)
		}

		if len(info.Network) != len(def.Network) {
			return fmt.Errorf("virtual machine networks number do not match, got %d - expected %d", len(info.Network), len(info.Network))
		}
		for i, n := range def.Network {
			if n.VLAN != info.Network[i].VLAN {
				return fmt.Errorf("virtual machine network[%d].vlan do not match, got %s - expected %s", i, info.Network[i].VLAN, n.VLAN)
			}
		}

		return nil
	}
}

func testAccCheckAnxCloudVirtualServerDisks(n string, expectedDisks []vm.Disk) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		c := testAccProvider.Meta().(client.Client)
		v := vsphere.NewAPI(c)
		ctx := context.Background()

		if !ok {
			return fmt.Errorf("virtual server not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("virtual server id not set")
		}

		info, err := v.Info().Get(ctx, rs.Primary.ID)
		if err != nil {
			return err
		}

		if len(info.DiskInfo) != len(expectedDisks) {
			return fmt.Errorf("virtual machine disk count do not match, got %d - expected %d", len(info.DiskInfo), len(expectedDisks))
		}

		for i, d := range info.DiskInfo {
			if d.DiskType != expectedDisks[i].Type {
				return fmt.Errorf("virtual machine disk with ID %d has incorrect type, got %s - expected %s", d.DiskID, d.DiskType, expectedDisks[i].Type)
			} else if d.DiskGB != expectedDisks[i].SizeGBs {
				return fmt.Errorf("virtual machine disk with ID %d has incorrect size, got %d - expected %d", d.DiskID, d.DiskGB, expectedDisks[i].SizeGBs)
			}
		}

		return nil
	}
}

func generateNetworkSubResourceString(networks []vm.Network) string {
	var output string
	template := "\nnetwork {\n\tvlan_id = \"%s\"\n\tnic_type = \"%s\"\n}\n"

	for _, n := range networks {
		output += fmt.Sprintf(template, n.VLAN, n.NICType)
	}

	return output
}

func generateDisksSubResourceString(disks []vm.Disk) string {
	var output string
	template := "\ndisks {\n\tdisk = %d\n\tdisk_type = \"%s\"\n}\n"

	for _, d := range disks {
		output += fmt.Sprintf(template, d.SizeGBs, d.Type)
	}

	return output
}
