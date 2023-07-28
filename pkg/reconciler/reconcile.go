package reconciler

import (
	"context"
	"time"

	"github.com/red-hat-storage/managed-fusion-fleet-reconciler/pkg/db"
	"github.com/red-hat-storage/managed-fusion-fleet-reconciler/pkg/forman"
	"github.com/red-hat-storage/managed-fusion-fleet-reconciler/pkg/types"

	"github.com/go-logr/logr"
)

type fleetReconciler struct {
	ctx      context.Context
	log      logr.Logger
	provider *types.ProviderCluster
}

func (r *fleetReconciler) reconcile() (forman.Result, error) {
	// Reconciles the desired state
	// Example for an installation flow:
	// 	1. Ensures the infra via AWS Cloudformation
	// 	2. Ensures ROKS Cluster installation on existing infra
	// 	3. Ensures ManagedFusionAgent Operator on ROKS Cluster
	// 	4. Ensures presence of config for MFAO monitoring stack
	// 	5. Ensures Data Foundation Config is present on ROKS Cluster
	r.log.Info("Provider %q reconciled", r.provider.ClusterID)
	return forman.Result{}, nil
}

func Reconcile(log logr.Logger, client *db.Database, req forman.Request) forman.Result {
	ctx := context.Background()
	log.Info("Processing request", "name", req.Name)
	provider, err := client.GetProviderCluster(ctx, req.Name)
	if err != nil {
		log.Error(err, "Failed to get provider cluster")
		return forman.Result{}
	}
	log.Info("Provider", "clusterId", provider.ClusterID)

	r := &fleetReconciler{
		ctx:      ctx,
		log:      log,
		provider: provider,
	}
	result, err := r.reconcile()
	if err != nil {
		log.Error(err, "Failed to reconcile provider cluster")
		return forman.Result{Requeue: true, After: 1 * time.Minute}
	}

	return result
}
