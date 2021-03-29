/*


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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/digitalocean/godo"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	digitaloceanv1beta1 "github.com/smirl/digitalocean-floating-ip-controller/api/v1beta1"
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
	Log               logr.Logger
	Scheme            *runtime.Scheme
	DigitaloceanToken string
}

func (r *FloatingIPBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&digitaloceanv1beta1.FloatingIPBinding{}).
		Watches(
			&source.Kind{Type: &v1.Node{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: handler.ToRequestsFunc(r.nodeToRequest),
			},
		).
		Complete(r)
}

func (r *FloatingIPBindingReconciler) nodeToRequest(nodeMapObject handler.MapObject) []reconcile.Request {
	// Whenever any node every happens reconcile ALL FloatingIPBindings
	// List all bindings
	var bindings digitaloceanv1beta1.FloatingIPBindingList
	err := r.List(context.Background(), &bindings)
	if err != nil {
		r.Log.Error(err, "Failed to list floating IP bindings")
		return []reconcile.Request{}
	}

	// Convert FloatingIPBindingList to []reconcile.Request
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

// +kubebuilder:rbac:groups=digitalocean.smirlwebs.com,resources=floatingipbindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=digitalocean.smirlwebs.com,resources=floatingipbindings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;watch;list
func (r *FloatingIPBindingReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("floatingipbinding", req.NamespacedName)

	// Create a digitalocean client
	// TODO: Move to controller set up
	client := godo.NewFromToken(r.DigitaloceanToken)

	// Get the FloatingIPBinding from Kubernetes
	binding, err := r.GetFloatingIPBinding(ctx, log, req.NamespacedName)
	if err != nil {
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	// Get the best node/droplet to assign to the floating IP
	droplet, err := r.GetDroplet(ctx, log, client, binding)
	if err != nil {
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}
	if droplet == nil {
		log.Info("No dropletID found. Requeuing.")
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	// Assign the droplet to the floating IP if required
	err = r.AssignFloatingIP(ctx, log, client, binding, droplet)
	if err != nil {
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	// Update status
	binding.Status.AssignedDropletID = droplet.ID
	binding.Status.AssignedDropletName = droplet.Name
	err = r.Status().Update(ctx, binding)
	if err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	return ctrl.Result{}, nil
}

func (r *FloatingIPBindingReconciler) GetFloatingIPBinding(
	ctx context.Context,
	log logr.Logger,
	name types.NamespacedName,
) (*digitaloceanv1beta1.FloatingIPBinding, error) {
	// Get the FloatingIPBinding from Kubernetes
	binding := &digitaloceanv1beta1.FloatingIPBinding{}
	if err := r.Get(ctx, name, binding); err != nil {
		err = client.IgnoreNotFound(err)
		if err != nil {
			log.Info("unable to fetch FloatingIPBinding object")
		} else {
			log.Info("unable to fetch FloatingIPBinding object because it has been deleted")
		}
		return nil, err
	}
	return binding, nil
}

func (r *FloatingIPBindingReconciler) GetDroplet(
	ctx context.Context,
	log logr.Logger,
	doClient *godo.Client,
	binding *digitaloceanv1beta1.FloatingIPBinding,
) (*Droplet, error) {
	var err error

	// Get NodeSelector or default to everything
	var selector labels.Selector
	if binding.Spec.NodeSelector == nil {
		selector = labels.Everything()
	} else {
		selector, err = metav1.LabelSelectorAsSelector(binding.Spec.NodeSelector)
		if err != nil {
			log.Error(err, "Could not parse NodeSelector")
			return nil, err
		}
	}

	// Get list of nodes
	var nodes v1.NodeList
	err = r.Client.List(ctx, &nodes, client.MatchingLabelsSelector{Selector: selector})
	if err != nil {
		log.Error(err, "Could not list nodes")
		return nil, err
	}
	if len(nodes.Items) == 0 {
		log.Info("No nodes matching NodeSelector")
		return nil, nil
	}

	// Sort nodes by Age
	sort.Slice(nodes.Items, func(i, j int) bool {
		return nodes.Items[i].CreationTimestamp.Before(&nodes.Items[j].CreationTimestamp)
	})

	// Choose node based on NodeSelectorPolicy
	var node v1.Node
	switch binding.Spec.NodeSelectorPolicy {
	case digitaloceanv1beta1.Newest:
		node = nodes.Items[len(nodes.Items)-1]
	case digitaloceanv1beta1.Oldest:
		node = nodes.Items[0]
	case digitaloceanv1beta1.Random:
		i := rand.IntnRange(0, len(nodes.Items))
		node = nodes.Items[i]
	default:
		// Default to Newest
		node = nodes.Items[len(nodes.Items)-1]
	}

	// Get dropletID int ID from providerId
	providerIdParts := strings.Split(node.Spec.ProviderID, "//")
	providerIdStr := providerIdParts[len(providerIdParts)-1]
	dropletID, err := strconv.Atoi(providerIdStr)
	if err != nil {
		log.Error(err, "Could not convert providerId to int")
		return nil, err
	}
	return &Droplet{ID: dropletID, Name: node.Name}, nil
}

func (r *FloatingIPBindingReconciler) AssignFloatingIP(
	ctx context.Context,
	log logr.Logger,
	doClient *godo.Client,
	binding *digitaloceanv1beta1.FloatingIPBinding,
	droplet *Droplet,
) error {
	// Use digitalocean API to assign floating IP
	log = log.WithValues(
		"dropletID", droplet.ID,
		"dropletName", droplet.Name,
		"floatingIP", binding.Spec.FloatingIP,
	)
	// Get IP to see if it is already assigned
	ip, _, err := doClient.FloatingIPs.Get(ctx, binding.Spec.FloatingIP)
	if err != nil {
		log.Error(err, "Failed to get floatingIP")
		return err
	}

	// Assign droplet to floating IP if not already assigned
	if ip.Droplet != nil && ip.Droplet.ID == droplet.ID {
		log.Info("Droplet is already assigned to floatingIP. Skipping.")
	} else {
		// Assign IP if not already assigned
		_, _, err = doClient.FloatingIPActions.Assign(ctx, binding.Spec.FloatingIP, droplet.ID)
		if err != nil {
			log.Error(err, "Failed update floatingIP")
			return err
		}
		log.Info("Assigned droplet to FloatingIP")
	}

	return nil
}
