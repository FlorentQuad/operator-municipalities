/*
Copyright 2024.

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

package controller

import (
	"context"

	slices "golang.org/x/exp/slices"

	"strings"

	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"

	municipalityv1alpha1 "github.com/FlorentQuad/operator-municipalities/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var municipalities []string

// MunicipalitiesReconciler reconciles a Municipalities object
type MunicipalitiesReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=municipality.municipalities.rvig,resources=municipalities,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=municipality.municipalities.rvig,resources=municipalities/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=municipality.municipalities.rvig,resources=municipalities/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
func (r *MunicipalitiesReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("reconcile start")

	mResource := &municipalityv1alpha1.Municipalities{}
	err := r.Get(ctx, req.NamespacedName, mResource)
	if err != nil {
		logger.Info("could not get Municiaplities object in namespace: " + req.NamespacedName.Namespace)
		return ctrl.Result{}, err
	}

	updatedList := strings.Split(strings.ReplaceAll(mResource.Spec.Online, " ", ""), ",")
	for _, m := range updatedList {
		if m != "" {
			r.createMunicipality(ctx, m, req.NamespacedName.Namespace)
		}
	}

	for _, m := range municipalities {
		if m != "" && !slices.Contains(updatedList, m) {
			r.removeMunicipality(ctx, m, req.NamespacedName.Namespace)
		}
	}

	municipalities = updatedList
	logger.Info("reconcile end")
	return ctrl.Result{}, nil
}

func (r *MunicipalitiesReconciler) createMunicipality(ctx context.Context, municipality string, namespace string) {
	logger := log.FromContext(ctx)

	var depl appsv1.Deployment
	deplName := types.NamespacedName{Name: municipality, Namespace: namespace}
	if err := r.Get(ctx, deplName, &depl); err != nil && errors.IsNotFound(err) {
		if !errors.IsNotFound(err) {
			logger.Info("something went wrong while searching for deployment " + municipality)
			return
		}

		var numberOfReplicas int32 = 1
		depl = appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      municipality,
				Namespace: namespace,
				Labels:    map[string]string{"label": municipality, "municipality": municipality},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &numberOfReplicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"label": municipality},
				},
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"label": municipality, "app": municipality},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  municipality + "-container",
								Image: "bitnami/nginx",
								Ports: []v1.ContainerPort{
									{
										ContainerPort: 8080,
									},
								},
							},
						},
					},
				},
			},
		}

		err = r.Create(ctx, &depl)
		if err != nil {
			logger.Error(err, "unable to create deployment")
		} else {
			logger.Info("deployment created")
		}

		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      municipality,
				Namespace: namespace,
				Labels:    map[string]string{"label": municipality, "municipality": municipality},
			},
			Spec: corev1.ServiceSpec{
				Type:     corev1.ServiceTypeNodePort,
				Selector: map[string]string{"label": municipality},
				Ports: []corev1.ServicePort{
					{
						Port: 8080,
						TargetPort: intstr.IntOrString{
							IntVal: 8080,
						},
					},
				},
			},
		}
		err = r.Create(ctx, &svc)
		if err != nil {
			logger.Error(err, "unable to create service")
		} else {
			logger.Info("service created")
		}

		route := &routev1.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name:      municipality,
				Namespace: namespace,
				Labels:    map[string]string{"label": municipality, "municipality": municipality},
			},
			Spec: routev1.RouteSpec{
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: municipality,
				},
				TLS: &routev1.TLSConfig{
					InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
					Termination:                   routev1.TLSTerminationEdge,
				},
			},
		}

		err = r.Create(ctx, &route)
		if err != nil {
			logger.Error(err, "unable to create route")
		} else {
			logger.Info("route created")
		}

	} else {
		logger.Info("municipality," + municipality + ", is already created")
	}
}

func (r *MunicipalitiesReconciler) removeMunicipality(ctx context.Context, municipality string, namespace string) {
	logger := log.FromContext(ctx)

	logger.Info("deleted " + municipality)
}

// SetupWithManager sets up the controller with the Manager.
func (r *MunicipalitiesReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&municipalityv1alpha1.Municipalities{}).
		Complete(r)
}
