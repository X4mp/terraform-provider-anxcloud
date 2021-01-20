package anxcloud

import (
	"context"

	"github.com/anexia-it/go-anxcloud/pkg/client"
	"github.com/anexia-it/go-anxcloud/pkg/core/tags"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceTags() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceTagsRead,
		Schema:      schemaTags(),
	}
}

func dataSourceTagsRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(client.Client)
	tagsAPI := tags.NewAPI(c)

	page := d.Get("page").(int)
	limit := d.Get("limit").(int)
	query := d.Get("query").(string)
	serviceIdentifier := d.Get("service_identifier").(string)
	organizationIdentifier := d.Get("organization_identifier").(string)
	order := d.Get("order").(string)
	sortAscending := d.Get("sort_ascending").(bool)

	tags, err := tagsAPI.List(ctx, page, limit, query, serviceIdentifier, organizationIdentifier, order, sortAscending)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("tags", flattenTags(tags)); err != nil {
		return diag.FromErr(err)
	}

	if id := uuid.New().String(); id != "" {
		d.SetId(id)
		return nil
	}

	return diag.Errorf("unable to create uuid for tags data source")
}
