/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"

	ktunnelsv1 "github.com/int128/ktunnels/api/v1"
)

const (
	tunnelOwnerKey = ".metadata.controller"
)

// AmazonAuroraClusterSetReconciler reconciles a AmazonAuroraClusterSet object
type AmazonAuroraClusterSetReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	rdsClient *rds.Client
}

//+kubebuilder:rbac:groups=ktunnels.int128.github.io,resources=amazonauroraclustersets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ktunnels.int128.github.io,resources=amazonauroraclustersets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ktunnels.int128.github.io,resources=amazonauroraclustersets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *AmazonAuroraClusterSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := crlog.FromContext(ctx)

	var clusterSet ktunnelsv1.AmazonAuroraClusterSet
	if err := r.Get(ctx, req.NamespacedName, &clusterSet); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !clusterSet.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// describe the clusters
	if r.rdsClient == nil {
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			log.Error(err, "unable to initialize AWS client")
			return ctrl.Result{}, err
		}
		r.rdsClient = rds.NewFromConfig(cfg)
	}
	var filters []types.Filter
	for _, f := range clusterSet.Spec.Filters {
		filters = append(filters, types.Filter{Name: aws.String(f.Name), Values: f.Values})
	}
	dbClustersOutput, err := r.rdsClient.DescribeDBClusters(ctx, &rds.DescribeDBClustersInput{Filters: filters})
	if err != nil {
		log.Error(err, "unable to describe clusters")
		return ctrl.Result{}, err
	}

	// compute the desired state
	desiredTunnelMap := make(map[string]ktunnelsv1.Tunnel)
	for _, dbCluster := range dbClustersOutput.DBClusters {
		name := aws.ToString(dbCluster.DBClusterIdentifier)
		tunnel := ktunnelsv1.Tunnel{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: clusterSet.Namespace,
			},
			Spec: ktunnelsv1.TunnelSpec{
				Host:  aws.ToString(dbCluster.Endpoint),
				Port:  aws.ToInt32(dbCluster.Port),
				Proxy: clusterSet.Spec.Proxy,
			},
		}
		desiredTunnelMap[name] = tunnel
	}

	var childTunnels ktunnelsv1.TunnelList
	if err := r.List(ctx, &childTunnels, client.InNamespace(req.Namespace), client.MatchingFields{tunnelOwnerKey: req.Name}); err != nil {
		log.Error(err, "unable to list child Tunnels")
		return ctrl.Result{}, err
	}
	childTunnelMap := make(map[string]ktunnelsv1.Tunnel)
	for _, item := range childTunnels.Items {
		childTunnelMap[item.Name] = item
	}

	for _, tunnel := range desiredTunnelMap {
		if _, ok := childTunnelMap[tunnel.Name]; !ok {
			if err := r.Create(ctx, &tunnel); err != nil {
				log.Error(err, "unable to create Tunnel")
				return ctrl.Result{}, err
			}
			log.Info("created Tunnel", "tunnel", tunnel.Name)
		}
	}

	for _, tunnel := range childTunnels.Items {
		if _, ok := desiredTunnelMap[tunnel.Name]; !ok {
			if err := r.Delete(ctx, &tunnel); err != nil {
				log.Error(err, "unable to delete Tunnel")
				return ctrl.Result{}, err
			}
			log.Info("deleted Tunnel", "tunnel", tunnel.Name)
		}
	}

	// TODO: update Tunnel if diff

	// TODO: update status

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AmazonAuroraClusterSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(),
		&ktunnelsv1.Tunnel{},
		tunnelOwnerKey,
		mapTunnelToOwnerName,
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&ktunnelsv1.AmazonAuroraClusterSet{}).
		Owns(&ktunnelsv1.Tunnel{}).
		Complete(r)
}

func mapTunnelToOwnerName(obj client.Object) []string {
	tunnel := obj.(*ktunnelsv1.Tunnel)
	owner := metav1.GetControllerOf(tunnel)
	if owner == nil {
		return nil
	}
	if owner.APIVersion != ktunnelsv1.GroupVersion.String() || owner.Kind != "AmazonAuroraClusterSet" {
		return nil
	}
	return []string{owner.Name}
}
