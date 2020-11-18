package anxcloud

import (
	"context"
	"fmt"
	"time"

	"github.com/anexia-it/go-anxcloud/pkg/client"
	"github.com/anexia-it/go-anxcloud/pkg/vsphere"
	"github.com/anexia-it/go-anxcloud/pkg/vsphere/provisioning/vm"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const (
	maxDNSEntries = 4
)

func resourceVirtualServer() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceVirtualServerCreate,
		ReadContext:   resourceVirtualServerRead,
		UpdateContext: resourceVirtualServerUpdate,
		DeleteContext: resourceVirtualServerDelete,
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(15 * time.Minute),
			Read:   schema.DefaultTimeout(1 * time.Minute),
			Update: schema.DefaultTimeout(5 * time.Minute),
			Delete: schema.DefaultTimeout(5 * time.Minute),
		},
		Schema: schemaVirtualServer(),
	}
}

func resourceVirtualServerCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var (
		diags    diag.Diagnostics
		networks []vm.Network
	)

	c := m.(client.Client)
	v := vsphere.NewAPI(c)
	locationID := d.Get("location_id").(string)

	networks = expandVirtualServerNetworks(d.Get("network").([]interface{}))
	for i, n := range networks {
		if len(n.IPs) > 0 {
			continue
		}

		freeIPs, err := v.Provisioning().IPs().GetFree(ctx, locationID, n.VLAN)
		if err != nil {
			diags = append(diags, diag.FromErr(err)...)
		} else if len(freeIPs) > 0 {
			networks[i].IPs = append(networks[i].IPs, freeIPs[0].Identifier)
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
			Summary:       "Warning level message",
			Detail:        "This is a warning, a very detailed one",
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

	provision, err := v.Provisioning().VM().Provision(ctx, def)
	if err != nil {
		return diag.FromErr(err)
	}
	err = resource.RetryContext(ctx, d.Timeout(schema.TimeoutCreate), func() *resource.RetryError {
		const complete = 100

		p, err := v.Provisioning().Progress().Get(ctx, provision.Identifier)
		if err != nil {
			return resource.NonRetryableError(fmt.Errorf("unable to get VM progress by ID '%s', %w", provision.Identifier, err))
		}
		if p.Progress == complete && p.VMIdentifier != "" {
			d.SetId(p.VMIdentifier)
			return nil
		}
		return resource.RetryableError(fmt.Errorf("VM with provisioning id '%s' is not ready yet: %d %%", provision.Identifier, p.Progress))
	})

	if err != nil {
		return diag.FromErr(err)
	}

	return resourceVirtualServerRead(ctx, d, m)
}

func resourceVirtualServerRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	c := m.(client.Client)
	v := vsphere.NewAPI(c)

	info, err := v.Info().Get(ctx, d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	if err = d.Set("cpus", info.CPU); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}
	if err = d.Set("memory", info.RAM); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}

	fInfo := flattenVirtualServerInfo(&info)
	if err = d.Set("info", fInfo); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}

	return diags
}

func resourceVirtualServerUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	if len(diags) > 0 {
		return diags
	}

	return resourceVirtualServerRead(ctx, d, m)
}

func resourceVirtualServerDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	return diags
}
