package civo

import (
	"fmt"
	"github.com/civo/civogo"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"log"
)

func resourceSnapshot() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				Description:  "This is a unqiue, alphanumerical, short, human readable code for the snapshot",
				ValidateFunc: validation.StringIsNotEmpty,
			},
			"instance_id": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				Description:  "The ID of the instance to snapshot",
				ValidateFunc: validation.StringIsNotEmpty,
			},
			"safe": {
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: true,
				Default:  false,
				Description: "If true the instance will be shut down during the snapshot to ensure all files" +
					"are in a consistent state (e.g. database tables aren't in the middle of being optimised" +
					"and hence risking corruption). The default is false so you experience no interruption" +
					"of service, but a small risk of corruption.",
			},
			"cron_timing": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Description: "If a valid cron string is passed, the snapshot will be saved as an automated snapshot," +
					"continuing to automatically update based on the schedule of the cron sequence provided." +
					"The default is nil meaning the snapshot will be saved as a one-off snapshot.",
				ValidateFunc: validation.StringIsNotEmpty,
			},
			// Computed resource
			"hostname": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"template_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"region": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"size_gb": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"state": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"requested_at": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"completed_at": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
		Create: resourceSnapshotCreate,
		Read:   resourceSnapshotRead,
		Delete: resourceSnapshotDelete,
	}
}

func resourceSnapshotCreate(d *schema.ResourceData, m interface{}) error {
	apiClient := m.(*civogo.Client)

	config := &civogo.SnapshotConfig{
		InstanceID: d.Get("instance_id").(string),
	}

	if attr, ok := d.GetOk("safe"); ok {
		config.Safe = attr.(bool)
	}

	if attr, ok := d.GetOk("cron_timing"); ok {
		config.Cron = attr.(string)
	}

	resp, err := apiClient.CreateSnapshot(d.Get("name").(string), config)
	if err != nil {
		fmt.Errorf("[WARN] failed to create snapshot: %s", err)
		return err
	}

	d.SetId(resp.ID)

	_, hasCronTiming := d.GetOk("cron_timing")

	if hasCronTiming {
		/*
			if hasCronTiming is declare them we no need to wait the state from the backend
		*/
		return resourceSnapshotRead(d, m)
	} else {
		/*
			if hasCronTiming is not declare them we need to wait the state from the backend
			and made a resource retry
		*/
		return resource.Retry(d.Timeout(schema.TimeoutCreate), func() *resource.RetryError {
			resp, err := apiClient.FindSnapshot(d.Id())
			if err != nil {
				return resource.NonRetryableError(fmt.Errorf("error geting snapshot: %s", err))
			}

			if resp.State != "complete" {
				return resource.RetryableError(fmt.Errorf("[WARN] expected snapshot to be created but was in state %s", resp.State))
			}

			return resource.NonRetryableError(resourceSnapshotRead(d, m))
		})
	}

}

func resourceSnapshotRead(d *schema.ResourceData, m interface{}) error {
	apiClient := m.(*civogo.Client)

	resp, err := apiClient.FindSnapshot(d.Id())
	if err != nil {
		fmt.Errorf("[WARN] failed to read snapshot: %s", err)
		return err
	}

	safeValue := false

	if resp.Safe == 1 {
		safeValue = true
	}

	d.Set("instance_id", resp.InstanceID)
	d.Set("hostname", resp.Hostname)
	d.Set("template_id", resp.Template)
	d.Set("region", resp.Region)
	d.Set("name", resp.Name)
	d.Set("safe", safeValue)
	d.Set("size_gb", resp.SizeGigabytes)
	d.Set("state", resp.State)
	d.Set("cron_timing", resp.Cron)
	d.Set("requested_at", resp.RequestedAt.String())
	d.Set("completed_at", resp.CompletedAt.String())

	return nil
}

func resourceSnapshotDelete(d *schema.ResourceData, m interface{}) error {
	apiClient := m.(*civogo.Client)

	_, err := apiClient.DeleteSnapshot(d.Id())
	if err != nil {
		log.Printf("[INFO] civo snapshot (%s) was delete", d.Id())
	}

	return nil
}
