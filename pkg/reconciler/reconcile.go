package reconciler

import (
	"context"

	"github.com/red-hat-storage/managed-fusion-fleet-reconciler/pkg/db"
	"github.com/red-hat-storage/managed-fusion-fleet-reconciler/pkg/forman"

	"github.com/go-logr/logr"
)

func Reconcile(log logr.Logger, client *db.Database, req forman.Request) forman.Result {
	ctx := context.Background()
	log.Info("Processing request", "name", req.Name)
	provider, err := client.GetProviderCluster(ctx, req.Name)
	if err != nil {
		log.Error(err, "Failed to get provider cluster")
		return forman.Result{}
	}
	log.Info("Provider", "clusterId", provider.ClusterID)

	return forman.Result{}
}
