/*
Copyright 2021 Alex Williams.

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

package digitalocean

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/digitalocean/godo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	digitaloceanv1beta1 "github.com/smirl/digitalocean-floating-ip-controller/apis/digitalocean/v1beta1"
)

const RequeueAfter = time.Minute * 5

// Hold information about a droplet
type Droplet struct {
	ID   int
	Name string
}

// FloatingIPBindingReconciler reconciles a FloatingIPBinding object
type FloatingIPBindingReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	DigitaloceanClient *godo.Client
}

// SetupWithManager sets up the controller with the Manager.
func (r *FloatingIPBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&digitaloceanv1beta1.FloatingIPBinding{}).
		Watches(
			&v1.Node{},
			handler.EnqueueRequestsFromMapFunc(r.nodeToRequests),
		).
		Complete(r)
}

//+kubebuilder:rbac:groups=digitalocean.smirlwebs.com,resources=floatingipbindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=digitalocean.smirlwebs.com,resources=floatingipbindings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=digitalocean.smirlwebs.com,resources=floatingipbindings/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;watch;list

func (r *FloatingIPBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get the FloatingIPBinding from Kubernetes
	binding, err := r.GetFloatingIPBinding(ctx, req.NamespacedName)
	if err != nil {
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	// Get the best node/droplet to assign to the floating IP
	droplet, err := r.GetDroplet(ctx, binding)
	if err != nil {
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}
	if droplet == nil {
		logger.Info("No dropletID found. Requeuing.")
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	// Assign the droplet to the floating IP if required
	err = r.AssignFloatingIP(ctx, binding, droplet)
	if err != nil {
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	// Update status
	binding.Status.AssignedDropletID = droplet.ID
	binding.Status.AssignedDropletName = droplet.Name
	err = r.Status().Update(ctx, binding)
	if err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	return ctrl.Result{}, nil
}

func (r *FloatingIPBindingReconciler) nodeToRequests(ctx context.Context, node client.Object) []reconcile.Request {
	// Whenever any node event happens, reconcile ALL FloatingIPBindings
	var bindings digitaloceanv1beta1.FloatingIPBindingList
	if err := r.List(ctx, &bindings); err != nil {
		log.FromContext(ctx).Error(err, "Failed to list floating IP bindings")
		return []reconcile.Request{}
	}

	var reconcileRequests []reconcile.Request
	for _, binding := range bindings.Items {
		reconcileRequests = append(reconcileRequests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      binding.GetName(),
				Namespace: binding.GetNamespace(),
			},
		})
	}
	return reconcileRequests
}

func (r *FloatingIPBindingReconciler) GetFloatingIPBinding(
	ctx context.Context,
	name types.NamespacedName,
) (*digitaloceanv1beta1.FloatingIPBinding, error) {
	logger := log.FromContext(ctx)
	binding := &digitaloceanv1beta1.FloatingIPBinding{}
	if err := r.Get(ctx, name, binding); err != nil {
		err = client.IgnoreNotFound(err)
		if err != nil {
			logger.Info("unable to fetch FloatingIPBinding object")
		} else {
			logger.Info("unable to fetch FloatingIPBinding object because it has been deleted")
		}
		return nil, err
	}
	return binding, nil
}

func (r *FloatingIPBindingReconciler) GetDroplet(
	ctx context.Context,
	binding *digitaloceanv1beta1.FloatingIPBinding,
) (*Droplet, error) {
	logger := log.FromContext(ctx)
	var err error

	// Get NodeSelector or default to everything
	var selector labels.Selector
	if binding.Spec.NodeSelector == nil {
		selector = labels.Everything()
	} else {
		selector, err = metav1.LabelSelectorAsSelector(binding.Spec.NodeSelector)
		if err != nil {
			logger.Error(err, "Could not parse NodeSelector")
			return nil, err
		}
	}

	// Get list of nodes
	var nodes v1.NodeList
	err = r.Client.List(ctx, &nodes, client.MatchingLabelsSelector{Selector: selector})
	if err != nil {
		logger.Error(err, "Could not list nodes")
		return nil, err
	}
	if len(nodes.Items) == 0 {
		logger.Info("No nodes matching NodeSelector")
		return nil, nil
	}

	// Sort nodes by Age
	sort.SliceStable(nodes.Items, func(i, j int) bool {
		return nodes.Items[i].CreationTimestamp.Before(&nodes.Items[j].CreationTimestamp)
	})

	// Choose node based on NodeSelectorPolicy
	var node *v1.Node
	switch binding.Spec.NodeSelectorPolicy {
	case digitaloceanv1beta1.Newest:
		node = &nodes.Items[len(nodes.Items)-1]
	case digitaloceanv1beta1.Oldest:
		node = &nodes.Items[0]
	case digitaloceanv1beta1.Random:
		// If already randomly assigned, keep the same node
		for _, n := range nodes.Items {
			if n.GetName() == binding.Status.AssignedDropletName {
				node = &n
				logger.Info("Randomly assigned droplet still exists. Skipping.")
				break
			}
		}
		if node == nil {
			i := rand.IntnRange(0, len(nodes.Items))
			node = &nodes.Items[i]
		}
	default:
		return nil, fmt.Errorf("Invalid NodeSelectorPolicy: %s", binding.Spec.NodeSelectorPolicy)
	}

	// Get dropletID from providerId
	providerIdParts := strings.Split(node.Spec.ProviderID, "//")
	providerIdStr := providerIdParts[len(providerIdParts)-1]
	dropletID, err := strconv.Atoi(providerIdStr)
	if err != nil {
		logger.Error(err, "Could not convert providerId to int")
		return nil, err
	}
	return &Droplet{ID: dropletID, Name: node.Name}, nil
}

func (r *FloatingIPBindingReconciler) AssignFloatingIP(
	ctx context.Context,
	binding *digitaloceanv1beta1.FloatingIPBinding,
	droplet *Droplet,
) error {
	logger := log.FromContext(ctx).WithValues(
		"dropletID", droplet.ID,
		"dropletName", droplet.Name,
		"floatingIP", binding.Spec.FloatingIP,
	)
	// Get IP to see if it is already assigned
	ip, _, err := r.DigitaloceanClient.FloatingIPs.Get(ctx, binding.Spec.FloatingIP)
	if err != nil {
		logger.Error(err, "Failed to get floatingIP")
		return err
	}

	// Assign droplet to floating IP if not already assigned
	if ip.Droplet != nil && ip.Droplet.ID == droplet.ID {
		logger.Info("Droplet is already assigned to floatingIP. Skipping.")
	} else {
		_, _, err = r.DigitaloceanClient.FloatingIPActions.Assign(ctx, binding.Spec.FloatingIP, droplet.ID)
		if err != nil {
			doError, ok := err.(*godo.ErrorResponse)
			if ok && doError.Response.StatusCode == 422 {
				logger.Info("FloatingIP is in pending state. Skipping.")
				return nil
			}
			logger.Error(err, "Failed update floatingIP")
			return err
		}
		logger.Info("Assigned droplet to FloatingIP")
	}

	return nil
}
