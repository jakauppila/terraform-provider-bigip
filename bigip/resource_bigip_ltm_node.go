/*
Original work from https://github.com/DealerDotCom/terraform-provider-bigip
Modifications Copyright 2019 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file,You can obtain one at https://mozilla.org/MPL/2.0/.
*/
package bigip

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"context"
	bigip "github.com/f5devcentral/go-bigip"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceBigipLtmNode() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceBigipLtmNodeCreate,
		ReadContext:   resourceBigipLtmNodeRead,
		UpdateContext: resourceBigipLtmNodeUpdate,
		DeleteContext: resourceBigipLtmNodeDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  "Name of the node",
				ForceNew:     true,
				ValidateFunc: validateF5Name,
			},
			"address": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Address of the node",
				ForceNew:    true,
			},
			"rate_limit": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Specifies the maximum number of connections per second allowed for a node or node address. The default value is 'disabled'.",
				Computed:    true,
			},
			"connection_limit": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Specifies the maximum number of connections allowed for the node or node address.",
				Computed:    true,
			},
			"dynamic_ratio": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Sets the dynamic ratio number for the node. Used for dynamic ratio load balancing. ",
				Computed:    true,
			},
			"ratio": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Sets the ratio number for the node.",
				Computed:    true,
			},
			"monitor": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "/Common/icmp",
				Description: "Specifies the name of the monitor or monitor rule that you want to associate with the node.",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "User defined description of the node.",
			},
			"state": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Marks the node up or down. The default value is user-up.",
			},
			"session": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Enables or disables the node for new sessions. The default value is user-enabled.",
				Computed:    true,
			},
			"fqdn": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"address_family": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Specifies the node's address family. The default is 'unspecified', or IP-agnostic",
						},
						"name": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Specifies the fully qualified domain name of the node.",
						},
						"interval": {
							Type:        schema.TypeString,
							Optional:    true,
							Computed:    true,
							Description: "Specifies the amount of time before sending the next DNS query.",
						},
						"downinterval": {
							Type:        schema.TypeInt,
							Optional:    true,
							Computed:    true,
							Description: "Specifies the number of attempts to resolve a domain name. The default is 5.",
						},
						"autopopulate": {
							Type:        schema.TypeString,
							Optional:    true,
							Computed:    true,
							Description: "Specifies whether the node should scale to the IP address set returned by DNS.",
						},
					},
				},
			},
		},
	}
}

func resourceBigipLtmNodeCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)

	name := d.Get("name").(string)
	address := d.Get("address").(string)
	rateLimit := d.Get("rate_limit").(string)
	connectionLimit := d.Get("connection_limit").(int)
	dynamicRatio := d.Get("dynamic_ratio").(int)
	monitor := d.Get("monitor").(string)
	state := d.Get("state").(string)
	session := d.Get("session").(string)
	description := d.Get("description").(string)
	ratio := d.Get("ratio").(int)

	r := regexp.MustCompile("^((?:[0-9]{1,3}.){3}[0-9]{1,3})|(.*:[^%]*)$")

	log.Println("[INFO] Creating node " + name + "::" + address)

	nodeConfig := &bigip.Node{
		Name:            name,
		RateLimit:       rateLimit,
		ConnectionLimit: connectionLimit,
		DynamicRatio:    dynamicRatio,
		Monitor:         monitor,
		State:           state,
		Session:         session,
		Description:     description,
		Ratio:           ratio,
	}

	if r.MatchString(address) {
		nodeConfig.Address = address
	} else {
		interval := d.Get("fqdn.0.interval").(string)
		addressFamily := d.Get("fqdn.0.address_family").(string)
		autopopulate := d.Get("fqdn.0.autopopulate").(string)
		downinterval := d.Get("fqdn.0.downinterval").(int)

		nodeConfig.FQDN.Name = address
		nodeConfig.FQDN.Interval = interval
		nodeConfig.FQDN.AddressFamily = addressFamily
		nodeConfig.FQDN.AutoPopulate = autopopulate
		nodeConfig.FQDN.DownInterval = downinterval
	}

	log.Printf("[DEBUG] config of Node to be add :%+v", nodeConfig)
	d.SetId(name)

	exist, _ := resourceBigipLtmNodeExists(d, meta)
	if !exist {
		if err := client.AddNode(nodeConfig); err != nil {
			d.SetId("")
			return diag.FromErr(fmt.Errorf("error modifying node %s: %v", name, err))
		}
	}
	return resourceBigipLtmNodeRead(ctx, d, meta)
}

func resourceBigipLtmNodeRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)

	name := d.Id()

	log.Println("[INFO] Fetching node " + name)

	node, err := client.GetNode(name)
	if err != nil {
		log.Printf("[ERROR] Unable to retrieve node %s  %v :", name, err)
		return diag.FromErr(err)
	}
	if node == nil {
		log.Printf("[WARN] Node (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}
	if node.FQDN.Name != "" {
		if err := d.Set("address", node.FQDN.Name); err != nil {
			return diag.FromErr(fmt.Errorf("[DEBUG] Error saving address to state for Node (%s): %s", d.Id(), err))
		}
	} else {
		// xxx.xxx.xxx.xxx(%x)
		// x:x(%x)
		regex := regexp.MustCompile(`((?:(?:[0-9]{1,3}\.){3}[0-9]{1,3})|(?:.*:[^%]*))(?:\%\d+)?`)
		address := regex.FindStringSubmatch(node.Address)
		log.Println("[INFO] Address: " + address[1])
		if err := d.Set("address", node.Address); err != nil {
			return diag.FromErr(fmt.Errorf("[DEBUG] Error saving address to state for Node (%s): %s", d.Id(), err))
		}
	}
	_ = d.Set("name", name)

	if err := d.Set("rate_limit", node.RateLimit); err != nil {
		return diag.FromErr(fmt.Errorf("[DEBUG] Error saving Monitor to state for Node (%s): %s", d.Id(), err))
	}

	if (node.Session == "monitor-enabled") || (node.Session == "user-enabled") {
		log.Printf("[DEBUG] node session is :%s", node.Session)
		_ = d.Set("session", "user-enabled")
	} else {
		log.Printf("[DEBUG] node session is :%s", node.Session)
		_ = d.Set("session", "user-disabled")
	}

	_ = d.Set("connection_limit", node.ConnectionLimit)
	_ = d.Set("description", node.Description)
	_ = d.Set("dynamic_ratio", node.DynamicRatio)
	_ = d.Set("monitor", strings.TrimSpace(node.Monitor))
	_ = d.Set("ratio", node.Ratio)
	_ = d.Set("fqdn.0.interval", node.FQDN.Interval)
	_ = d.Set("fqdn.0.downinterval", node.FQDN.DownInterval)
	_ = d.Set("fqdn.0.autopopulate", node.FQDN.AutoPopulate)
	_ = d.Set("fqdn.0.address_family", node.FQDN.AddressFamily)

	return nil
}

func resourceBigipLtmNodeExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	client := meta.(*bigip.BigIP)

	name := d.Id()
	log.Println("[INFO] Fetching node " + name)

	node, err := client.GetNode(name)
	if err != nil {
		log.Printf("[ERROR] Unable to retrieve node %s  %v :", name, err)
		return false, err
	}

	if node == nil {
		log.Printf("[WARN] node (%s) not found, removing from state", d.Id())
		return false, nil
	}
	return true, nil
}

func resourceBigipLtmNodeUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)

	name := d.Id()
	address := d.Get("address").(string)
	r := regexp.MustCompile("^((?:[0-9]{1,3}.){3}[0-9]{1,3})|(.*:[^%]*)$")

	nodeConfig := &bigip.Node{
		ConnectionLimit: d.Get("connection_limit").(int),
		DynamicRatio:    d.Get("dynamic_ratio").(int),
		Monitor:         d.Get("monitor").(string),
		RateLimit:       d.Get("rate_limit").(string),
		State:           d.Get("state").(string),
		Session:         d.Get("session").(string),
		Description:     d.Get("description").(string),
		Ratio:           d.Get("ratio").(int),
	}

	if r.MatchString(address) {
		nodeConfig.Address = address
	}

	if err := client.ModifyNode(name, nodeConfig); err != nil {
		return diag.FromErr(fmt.Errorf("error modifying node %s: %v", name, err))
	}

	return resourceBigipLtmNodeRead(ctx, d, meta)
}

func resourceBigipLtmNodeDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)

	name := d.Id()
	log.Println("[INFO] Deleting node " + name)
	err := client.DeleteNode(name)

	if err != nil {
		log.Printf("[ERROR] Unable to Delete Node %s  %v : ", name, err)
		return diag.FromErr(err)
	}
	d.SetId("")
	return nil
}
