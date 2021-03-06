package anxcloud

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/anexia-it/go-anxcloud/pkg/client"
	"github.com/anexia-it/go-anxcloud/pkg/ipam/address"
	"github.com/anexia-it/go-anxcloud/pkg/vsphere"
	"github.com/anexia-it/go-anxcloud/pkg/vsphere/provisioning/nictype"
	"github.com/anexia-it/go-anxcloud/pkg/vsphere/provisioning/vm"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/customdiff"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const (
	maxDNSEntries = 4
	vmPoweredOn   = "poweredOn"
	vmPoweredOff  = "poweredOff"
)

func resourceVirtualServer() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceVirtualServerCreate,
		ReadContext:   resourceVirtualServerRead,
		UpdateContext: resourceVirtualServerUpdate,
		DeleteContext: resourceVirtualServerDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(60 * time.Minute),
			Read:   schema.DefaultTimeout(1 * time.Minute),
			Update: schema.DefaultTimeout(60 * time.Minute),
			Delete: schema.DefaultTimeout(5 * time.Minute),
		},
		Schema: schemaVirtualServer(),
		CustomizeDiff: customdiff.All(
			customdiff.ForceNewIf("network", func(ctx context.Context, d *schema.ResourceDiff, meta interface{}) bool {
				old, new := d.GetChange("network")
				oldNets := expandVirtualServerNetworks(old.([]interface{}))
				newNets := expandVirtualServerNetworks(new.([]interface{}))

				if len(oldNets) > len(newNets) {
					// some network has been deleted
					return true
				}

				for i, n := range newNets {
					if i+1 > len(oldNets) {
						// new networks were added
						break
					}

					if n.VLAN != oldNets[i].VLAN {
						key := fmt.Sprintf("network.%d.vlan_id", i)
						if err := d.ForceNew(key); err != nil {
							log.Fatalf("[ERROR] unable to force new '%s': %v", key, err)
						}
					}

					if n.NICType != oldNets[i].NICType {
						key := fmt.Sprintf("network.%d.nic_type", i)
						if err := d.ForceNew(key); err != nil {
							log.Fatalf("[ERROR] unable to force new '%s': %v", key, err)
						}
					}

					if len(n.IPs) != len(oldNets[i].IPs) {
						key := fmt.Sprintf("network.%d.ips", i)
						if err := d.ForceNew(key); err != nil {
							log.Fatalf("[ERROR] unable to force new '%s': %v", key, err)
						}
					} else {
						for j, ip := range n.IPs {
							if ip != oldNets[i].IPs[j] {
								key := fmt.Sprintf("network.%d.ips", i)
								if err := d.ForceNew(key); err != nil {
									log.Fatalf("[ERROR] unable to force new '%s': %v", key, err)
								}
							}
						}
					}
				}

				return false
			}),
			customdiff.ForceNewIfChange("disk", func(ctx context.Context, old, new, meta interface{}) bool {
				return old.(int) > new.(int)
			}),
		),
	}
}

func resourceVirtualServerCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var (
		diags    diag.Diagnostics
		networks []vm.Network
	)

	c := m.(client.Client)
	v := vsphere.NewAPI(c)
	a := address.NewAPI(c)
	locationID := d.Get("location_id").(string)

	networks = expandVirtualServerNetworks(d.Get("network").([]interface{}))
	for i, n := range networks {
		if len(n.IPs) > 0 {
			continue
		}

		res, err := a.ReserveRandom(ctx, address.ReserveRandom{
			LocationID: locationID,
			VlanID:     n.VLAN,
			Count:      1,
		})
		if err != nil {
			diags = append(diags, diag.FromErr(err)...)
		} else if len(res.Data) > 0 {
			networks[i].IPs = append(networks[i].IPs, res.Data[0].Address)
		} else {
			diags = append(diags, diag.Diagnostic{
				Severity:      diag.Error,
				Summary:       "Free IP not found",
				Detail:        fmt.Sprintf("Free IP not found for VLAN: '%s'", n.VLAN),
				AttributePath: cty.Path{cty.GetAttrStep{Name: "ips"}},
			})
		}
	}

	dns := expandVirtualServerDNS(d.Get("dns").([]interface{}))
	if len(dns) != maxDNSEntries {
		diags = append(diags, diag.Diagnostic{
			Severity:      diag.Error,
			Summary:       "DNS entries exceed limit",
			Detail:        "Number of DNS entries cannot exceed limit 4",
			AttributePath: cty.Path{cty.GetAttrStep{Name: "dns"}},
		})
	}

	if len(diags) > 0 {
		return diags
	}

	def := vm.Definition{
		Location:           locationID,
		TemplateType:       d.Get("template_type").(string),
		TemplateID:         d.Get("template_id").(string),
		Hostname:           d.Get("hostname").(string),
		Memory:             d.Get("memory").(int),
		CPUs:               d.Get("cpus").(int),
		Disk:               d.Get("disk").(int),
		DiskType:           d.Get("disk_type").(string),
		CPUPerformanceType: d.Get("cpu_performance_type").(string),
		Sockets:            d.Get("sockets").(int),
		Network:            networks,
		DNS1:               dns[0],
		DNS2:               dns[1],
		DNS3:               dns[2],
		DNS4:               dns[3],
		Password:           d.Get("password").(string),
		SSH:                d.Get("ssh_key").(string),
		Script:             d.Get("script").(string),
		BootDelay:          d.Get("boot_delay").(int),
		EnterBIOSSetup:     d.Get("enter_bios_setup").(bool),
	}

	base64Encoding := true
	provision, err := v.Provisioning().VM().Provision(ctx, def, base64Encoding)
	if err != nil {
		return diag.FromErr(err)
	}
	err = resource.RetryContext(ctx, d.Timeout(schema.TimeoutCreate), func() *resource.RetryError {
		if d.Id() == "" {
			p, err := v.Provisioning().Progress().Get(ctx, provision.Identifier)
			if err != nil {
				return resource.NonRetryableError(fmt.Errorf("unable to get vm progress by ID '%s', %w", provision.Identifier, err))
			}
			if p.VMIdentifier != "" {
				d.SetId(p.VMIdentifier)
			} else {
				return resource.RetryableError(fmt.Errorf("vm with provisioning ID '%s' is not ready yet: %d %%", provision.Identifier, p.Progress))
			}
		}

		info, err := v.Info().Get(ctx, d.Id())
		if err != nil {
			return resource.NonRetryableError(fmt.Errorf("unable to get vm  by ID '%s', %w", d.Id(), err))
		}
		if info.Status == vmPoweredOn {
			return nil
		}
		return resource.RetryableError(fmt.Errorf("vm with id '%s' is not %s yet: %s", d.Id(), vmPoweredOn, info.Status))
	})

	if err != nil {
		return diag.FromErr(err)
	}

	tags := expandTags(d.Get("tags").([]interface{}))
	for _, t := range tags {
		if err := attachTag(ctx, c, d.Id(), t); err != nil {
			return diag.FromErr(err)
		}
	}

	return resourceVirtualServerRead(ctx, d, m)
}

func resourceVirtualServerRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	c := m.(client.Client)
	v := vsphere.NewAPI(c)
	n := nictype.NewAPI(c)

	info, err := v.Info().Get(ctx, d.Id())
	if err != nil {
		if err := handleNotFoundError(err); err != nil {
			return diag.FromErr(err)
		}
		d.SetId("")
		return nil
	}

	nicTypes, err := n.List(ctx)
	if err != nil {
		return diag.FromErr(err)
	}

	// TODO: we miss information about:
	// * cpu_performance_type

	if err = d.Set("location_id", info.LocationID); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}
	if err = d.Set("template_id", info.TemplateID); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}
	if err = d.Set("template_type", info.TemplateType); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}
	if err = d.Set("cpus", info.CPU); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}
	if v := d.Get("sockets").(int); v != 0 {
		// TODO: API fix: info.Cores should be info.Sockets, there is info.Cpus which is info.Cores
		if err = d.Set("sockets", info.Cores); err != nil {
			diags = append(diags, diag.FromErr(err)...)
		}
	}

	if err = d.Set("memory", info.RAM); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}

	if len(info.DiskInfo) != 1 {
		return diag.Errorf("unsupported number of disks, currently only 1 disk is allowed, got %d", len(info.DiskInfo))
	}
	if err = d.Set("disk", info.DiskInfo[0].DiskGB); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}
	if v := d.Get("disk_type").(string); v != "" {
		if err = d.Set("disk_type", info.DiskInfo[0].DiskType); err != nil {
			diags = append(diags, diag.FromErr(err)...)
		}
	}

	specNetworks := expandVirtualServerNetworks(d.Get("network").([]interface{}))
	var networks []vm.Network
	for i, net := range info.Network {
		if len(nicTypes) < net.NIC {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "Requested invalid nic type",
				Detail:   fmt.Sprintf("NIC type index out of range, available %d, wanted %d", len(nicTypes), net.NIC),
			})
			continue
		}

		network := vm.Network{
			NICType: nicTypes[net.NIC-1],
			VLAN:    net.VLAN,
		}

		// in spec it's not required to set an IP address
		// however when it's set we have to reflect that in the state
		if i+1 < len(specNetworks) && len(specNetworks[i].IPs) > 0 {
			network.IPs = append(net.IPv4, net.IPv6...)
		}

		networks = append(networks, network)
	}

	fNetworks := flattenVirtualServerNetwork(networks)
	if err = d.Set("network", fNetworks); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}

	fInfo := flattenVirtualServerInfo(&info)
	if err = d.Set("info", fInfo); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}

	return diags
}

func resourceVirtualServerUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(client.Client)
	v := vsphere.NewAPI(c)
	ch := vm.Change{
		Reboot:          d.Get("force_restart_if_needed").(bool),
		EnableDangerous: d.Get("critical_operation_confirmed").(bool),
	}
	requiresReboot := false

	if d.HasChanges("sockets", "memory", "cpus") {
		ch.CPUs = d.Get("cpus").(int)
		ch.CPUSockets = d.Get("sockets").(int)
		ch.MemoryMBs = d.Get("memory").(int)

		requiresReboot = true
	}

	// must stay in a separate condition as any endpoint doesn't return info about the current state
	// thus we lose control over expected and current states
	if d.HasChange("cpu_performance_type") {
		ch.CPUPerformanceType = d.Get("cpu_performance_type").(string)
	}

	if d.HasChange("network") {
		old, new := d.GetChange("network")
		oldNets := expandVirtualServerNetworks(old.([]interface{}))
		newNets := expandVirtualServerNetworks(new.([]interface{}))

		if len(oldNets) < len(newNets) {
			ch.AddNICs = newNets[len(oldNets):]
		} else {
			return diag.Errorf(
				"unsupported update operation, cannot remove network or update its parameters",
			)
		}
	}

	if d.HasChanges("disk_type", "disk") {
		var disk vm.Disk

		info := expandVirtualServerInfo(d.Get("info").([]interface{}))
		if len(info.DiskInfo) != 1 {
			return diag.Errorf("unsupported number of disks, currently only 1 disk is allowed, got %d", len(info.DiskInfo))
		}

		disk.ID = info.DiskInfo[0].DiskID
		disk.Type = d.Get("disk_type").(string)
		disk.SizeGBs = d.Get("disk").(int)

		ch.ChangeDisks = append(ch.ChangeDisks, disk)

		requiresReboot = true
	}

	if d.HasChange("tags") {
		old, new := d.GetChange("tags")
		oldTags := expandTags(old.([]interface{}))
		newTags := expandTags(new.([]interface{}))

		dTags := getTagsDifferences(oldTags, newTags)
		cTags := getTagsDifferences(newTags, oldTags)

		for _, t := range dTags {
			if err := detachTag(ctx, c, d.Id(), t); err != nil {
				return diag.FromErr(err)
			}
		}

		for _, t := range cTags {
			if err := attachTag(ctx, c, d.Id(), t); err != nil {
				return diag.FromErr(err)
			}
		}
	}

	if _, err := v.Provisioning().VM().Update(ctx, d.Id(), ch); err != nil {
		return diag.FromErr(err)
	}

	delay := 10 * time.Second
	if requiresReboot {
		delay = 3 * time.Minute
	}

	vmState := resource.StateChangeConf{
		Delay:      delay,
		Timeout:    d.Timeout(schema.TimeoutUpdate),
		MinTimeout: 10 * time.Second,
		Pending: []string{
			vmPoweredOff,
		},
		Target: []string{
			vmPoweredOn,
		},
		Refresh: func() (interface{}, string, error) {
			info, err := v.Info().Get(ctx, d.Id())
			if err != nil {
				return "", "", err
			}
			return info, info.Status, nil
		},
	}
	_, err := vmState.WaitForStateContext(ctx)
	if err != nil {
		return diag.FromErr(err)
	}

	return resourceVirtualServerRead(ctx, d, m)
}

func resourceVirtualServerDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(client.Client)
	v := vsphere.NewAPI(c)

	delayedDeprovision := false
	err := v.Provisioning().VM().Deprovision(ctx, d.Id(), delayedDeprovision)
	if err != nil {
		if err := handleNotFoundError(err); err != nil {
			return diag.FromErr(err)
		}
		d.SetId("")
		return nil
	}

	err = resource.RetryContext(ctx, d.Timeout(schema.TimeoutDelete), func() *resource.RetryError {
		_, err := v.Info().Get(ctx, d.Id())
		if err != nil {
			if err := handleNotFoundError(err); err != nil {
				return resource.NonRetryableError(fmt.Errorf("unable to get vm with id '%s': %w", d.Id(), err))
			}
			d.SetId("")
			return nil
		}
		return resource.RetryableError(fmt.Errorf("waiting for vm with id '%s' to be deleted", d.Id()))
	})
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func getTagsDifferences(tagsA, tagsB []string) []string {
	var out []string

	for _, a := range tagsA {
		found := false
		for _, b := range tagsB {
			if a == b {
				found = true
			}
		}
		if !found {
			out = append(out, a)
		}
	}

	return out
}
