package cluster

import (
	"context"
	"fmt"
	"time"

	sdk "github.com/openshift-online/ocm-sdk-go"
	v1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	machinepool "github.com/openshift-online/ocm-support-cli/pkg/machine_pool"
)

type Cluster struct {
	ID                string                    `json:"id"`
	HREF              string                    `json:"href"`
	Name              string                    `json:"name"`
	ExternalID        string                    `json:"external_id"`
	DisplayName       string                    `json:"display_name"`
	SubscriptionID    string                    `json:"subscription_id"`
	State             string                    `json:"state"`
	CloudProvider     string                    `json:"cloud_provider"`
	Version           string                    `json:"version"`
	RegionID          string                    `json:"region_id"`
	MultiAZ           bool                      `json:"multi_az"`
	MachinePools      []machinepool.MachinePool `json:"machine_pools,omitempty"`
	ProductID         string                    `json:"product_id"`
	Managed           bool                      `json:"managed"`
	ConsoleURL        string                    `json:"console_url"`
	CreationTimestamp time.Time                 `json:"creation_timestamp"`
}

func GetClusters(key string, searchStr string, limit int, fetchMachinePools bool, connection *sdk.Connection) ([]*v1.Cluster, error) {
	// Validate key
	if key == "" {
		return nil, fmt.Errorf("organization ID cannot be empty")
	}

	var search string

	search = fmt.Sprintf("(id = '%s'", key)              // cluster_id
	search += fmt.Sprintf(" or external_id = '%s'", key) //external_cluster_id
	search += fmt.Sprintf(" or organization.id = '%s'", key)
	search += fmt.Sprintf(" or subscription.id = '%s')", key)
	if searchStr != "" {
		search += fmt.Sprintf(" and %s", searchStr)
	}

	clusters, err := connection.ClustersMgmt().V1().Clusters().List().
		Size(limit).
		Search(search).
		SendContext(context.Background())
	if err != nil {
		return []*v1.Cluster{}, fmt.Errorf("failed to retrieve clusters: %w", err)
	}

	return clusters.Items().Slice(), nil
}

func PresentClusters(cluster *v1.Cluster, mp []*v1.MachinePool) Cluster {
	return Cluster{
		ID:                cluster.ID(),
		HREF:              cluster.HREF(),
		Name:              cluster.Name(),
		ExternalID:        cluster.ExternalID(),
		DisplayName:       cluster.Name(), // Display Name is the same as Name
		SubscriptionID:    cluster.Subscription().ID(),
		State:             string(cluster.State()),
		CloudProvider:     cluster.CloudProvider().ID(),
		Version:           cluster.OpenshiftVersion(),
		RegionID:          cluster.Region().ID(),
		MultiAZ:           cluster.MultiAZ(),
		MachinePools:      machinepool.PresentMachinePool(mp),
		ProductID:         cluster.Product().ID(),
		Managed:           cluster.Managed(),
		ConsoleURL:        cluster.Console().URL(),
		CreationTimestamp: cluster.CreationTimestamp(),
	}
}
