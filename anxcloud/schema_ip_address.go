package anxcloud

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func schemaIPAddresses() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"page": {
			Type:        schema.TypeInt,
			Optional:    true,
			Default:     1,
			Description: "The page of the list.",
		},
		"limit": {
			Type:        schema.TypeInt,
			Optional:    true,
			Default:     1000,
			Description: "The records limit of the list.",
		},
		"search": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "An optional string allowing to search trough entities.",
		},
		"addresses": {
			Type:        schema.TypeList,
			Computed:    true,
			Description: "List of available addresses.",
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"identifier": {
						Type:        schema.TypeString,
						Computed:    true,
						Description: "Identifier of the address.",
					},
					"address": {
						Type:        schema.TypeString,
						Computed:    true,
						Description: "The IP address.",
					},
					"description_customer": {
						Type:        schema.TypeString,
						Computed:    true,
						Description: "Additional customer description.",
					},
					"role": {
						Type:        schema.TypeString,
						Computed:    true,
						Description: "Role of the IP address",
					},
				},
			},
		},
	}
}

func schemaIPAddress() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"network_prefix_id": {
			Type:        schema.TypeString,
			Required:    true,
			ForceNew:    true,
			Description: "Identifier of the related network prefix.",
		},
		"address": {
			Type:        schema.TypeString,
			Required:    true,
			ForceNew:    true,
			Description: "IP address that should be created",
		},
		"description_customer": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Additional customer description.",
		},
		"description_internal": {
			Type:        schema.TypeString,
			Computed:    true,
			Description: "Internal description.",
		},
		"role": {
			Type:        schema.TypeString,
			Optional:    true,
			Default:     "Default",
			Description: "Role of the IP address",
		},
		"organization": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Customer of yours. Reseller only.",
		},
		"version": {
			Type:        schema.TypeInt,
			Computed:    true,
			Description: "IP version.",
		},
		"vlan_id": {
			Type:        schema.TypeString,
			Computed:    true,
			Description: "The associated VLAN identifier.",
		},
		"status": {
			Type:        schema.TypeString,
			Computed:    true,
			Description: "Status of the IP address",
		},
	}
}
