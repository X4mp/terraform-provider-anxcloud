package anxcloud

import (
	"context"

	"github.com/anexia-it/go-anxcloud/pkg/client"
	"github.com/anexia-it/go-anxcloud/pkg/vsphere/provisioning/templates"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceTemplate() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceTemplateRead,
		Schema:      schemaTemplate(),
	}
}

func dataSourceTemplateRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(client.Client)
	t := templates.NewAPI(c)
	locationID := d.Get("location_id").(string)
	templateType := d.Get("template_type").(string)
	templates, err := t.List(ctx, locationID, templateType, 1, 1000)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("templates", flattenTemplates(templates)); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(locationID)
	return nil
}
