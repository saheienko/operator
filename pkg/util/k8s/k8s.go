package k8s

import (
	"context"
	"io/ioutil"
	"reflect"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	corev1alpha1 "github.com/libopenstorage/operator/pkg/apis/core/v1alpha1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ParseObjectFromFile reads the given file and loads the object from the file in obj
func ParseObjectFromFile(
	filepath string,
	scheme *runtime.Scheme,
	obj runtime.Object,
) error {
	objBytes, err := ioutil.ReadFile(filepath)
	if err != nil {
		return err
	}
	codecs := serializer.NewCodecFactory(scheme)
	_, _, err = codecs.UniversalDeserializer().Decode([]byte(objBytes), nil, obj)
	return err
}

// CreateOrUpdateServiceAccount creates a service account if not present,
// else updates it if it has changed
func CreateOrUpdateServiceAccount(
	k8sClient client.Client,
	sa *v1.ServiceAccount,
	ownerRef *metav1.OwnerReference,
) error {
	existingSA := &v1.ServiceAccount{}
	err := k8sClient.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      sa.Name,
			Namespace: sa.Namespace,
		},
		existingSA,
	)
	if errors.IsNotFound(err) {
		logrus.Debugf("Creating %s service account", sa.Name)
		return k8sClient.Create(context.TODO(), sa)
	} else if err != nil {
		return err
	}

	for _, o := range existingSA.OwnerReferences {
		if o.UID != ownerRef.UID {
			sa.OwnerReferences = append(sa.OwnerReferences, o)
		}
	}

	if len(sa.OwnerReferences) > len(existingSA.OwnerReferences) {
		logrus.Debugf("Updating %s service account", sa.Name)
		return k8sClient.Update(context.TODO(), sa)
	}
	return nil
}

// DeleteServiceAccount deletes a service account if present and owned
func DeleteServiceAccount(
	k8sClient client.Client,
	name, namespace string,
	owners ...metav1.OwnerReference,
) error {
	resource := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	serviceAccount := &v1.ServiceAccount{}
	err := k8sClient.Get(context.TODO(), resource, serviceAccount)
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	newOwners := removeOwners(serviceAccount.OwnerReferences, owners)

	// Do not delete the object if it does not have the owner that was passed;
	// even if the object has no owner
	if (len(serviceAccount.OwnerReferences) == 0 && len(owners) > 0) ||
		(len(serviceAccount.OwnerReferences) > 0 && len(serviceAccount.OwnerReferences) == len(newOwners)) {
		logrus.Debugf("Cannot delete ServiceAccount %s/%s as it is not owned",
			namespace, name)
		return nil
	}

	if len(newOwners) == 0 {
		logrus.Debugf("Deleting %s/%s ServiceAccount", namespace, name)
		return k8sClient.Delete(context.TODO(), serviceAccount)
	}
	serviceAccount.OwnerReferences = newOwners
	logrus.Debugf("Disowning %s/%s ServiceAccount", namespace, name)
	return k8sClient.Update(context.TODO(), serviceAccount)
}

// CreateOrUpdateRole creates a role if not present,
// else updates it if it has changed
func CreateOrUpdateRole(
	k8sClient client.Client,
	role *rbacv1.Role,
	ownerRef *metav1.OwnerReference,
) error {
	existingRole := &rbacv1.Role{}
	err := k8sClient.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      role.Name,
			Namespace: role.Namespace,
		},
		existingRole,
	)
	if errors.IsNotFound(err) {
		logrus.Debugf("Creating %s role", role.Name)
		return k8sClient.Create(context.TODO(), role)
	} else if err != nil {
		return err
	}

	modified := !reflect.DeepEqual(role.Rules, existingRole.Rules)

	for _, o := range existingRole.OwnerReferences {
		if o.UID != ownerRef.UID {
			role.OwnerReferences = append(role.OwnerReferences, o)
		}
	}

	if modified || len(role.OwnerReferences) > len(existingRole.OwnerReferences) {
		logrus.Debugf("Updating %s role", role.Name)
		return k8sClient.Update(context.TODO(), role)
	}
	return nil
}

// DeleteRole deletes a role if present and owned
func DeleteRole(
	k8sClient client.Client,
	name, namespace string,
	owners ...metav1.OwnerReference,
) error {
	resource := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	role := &rbacv1.Role{}
	err := k8sClient.Get(context.TODO(), resource, role)
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	newOwners := removeOwners(role.OwnerReferences, owners)

	// Do not delete the object if it does not have the owner that was passed;
	// even if the object has no owner
	if (len(role.OwnerReferences) == 0 && len(owners) > 0) ||
		(len(role.OwnerReferences) > 0 && len(role.OwnerReferences) == len(newOwners)) {
		logrus.Debugf("Cannot delete Role %s/%s as it is not owned",
			namespace, name)
		return nil
	}

	if len(newOwners) == 0 {
		logrus.Debugf("Deleting %s/%s Role", namespace, name)
		return k8sClient.Delete(context.TODO(), role)
	}
	role.OwnerReferences = newOwners
	logrus.Debugf("Disowning %s/%s Role", namespace, name)
	return k8sClient.Update(context.TODO(), role)
}

// CreateOrUpdateRoleBinding creates a role binding if not present,
// else updates it if it has changed
func CreateOrUpdateRoleBinding(
	k8sClient client.Client,
	rb *rbacv1.RoleBinding,
	ownerRef *metav1.OwnerReference,
) error {
	existingRB := &rbacv1.RoleBinding{}
	err := k8sClient.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      rb.Name,
			Namespace: rb.Namespace,
		},
		existingRB,
	)
	if errors.IsNotFound(err) {
		logrus.Debugf("Creating %s role binding", rb.Name)
		return k8sClient.Create(context.TODO(), rb)
	} else if err != nil {
		return err
	}

	modified := !reflect.DeepEqual(rb.Subjects, existingRB.Subjects) ||
		!reflect.DeepEqual(rb.RoleRef, existingRB.RoleRef)

	for _, o := range existingRB.OwnerReferences {
		if o.UID != ownerRef.UID {
			rb.OwnerReferences = append(rb.OwnerReferences, o)
		}
	}

	if modified || len(rb.OwnerReferences) > len(existingRB.OwnerReferences) {
		logrus.Debugf("Updating %s role binding", rb.Name)
		return k8sClient.Update(context.TODO(), rb)
	}
	return nil
}

// DeleteRoleBinding deletes a role binding if present and owned
func DeleteRoleBinding(
	k8sClient client.Client,
	name, namespace string,
	owners ...metav1.OwnerReference,
) error {
	resource := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	roleBinding := &rbacv1.RoleBinding{}
	err := k8sClient.Get(context.TODO(), resource, roleBinding)
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	newOwners := removeOwners(roleBinding.OwnerReferences, owners)

	// Do not delete the object if it does not have the owner that was passed;
	// even if the object has no owner
	if (len(roleBinding.OwnerReferences) == 0 && len(owners) > 0) ||
		(len(roleBinding.OwnerReferences) > 0 && len(roleBinding.OwnerReferences) == len(newOwners)) {
		logrus.Debugf("Cannot delete RoleBinding %s/%s as it is not owned",
			namespace, name)
		return nil
	}

	if len(newOwners) == 0 {
		logrus.Debugf("Deleting %s/%s RoleBinding", namespace, name)
		return k8sClient.Delete(context.TODO(), roleBinding)
	}
	roleBinding.OwnerReferences = newOwners
	logrus.Debugf("Disowning %s/%s RoleBinding", namespace, name)
	return k8sClient.Update(context.TODO(), roleBinding)
}

// CreateOrUpdateClusterRole creates a cluster role if not present,
// else updates it if it has changed
func CreateOrUpdateClusterRole(
	k8sClient client.Client,
	cr *rbacv1.ClusterRole,
	ownerRef *metav1.OwnerReference,
) error {
	existingCR := &rbacv1.ClusterRole{}
	err := k8sClient.Get(
		context.TODO(),
		types.NamespacedName{Name: cr.Name},
		existingCR,
	)
	if errors.IsNotFound(err) {
		logrus.Debugf("Creating %s cluster role", cr.Name)
		return k8sClient.Create(context.TODO(), cr)
	} else if err != nil {
		return err
	}

	modified := !reflect.DeepEqual(cr.Rules, existingCR.Rules)

	for _, o := range existingCR.OwnerReferences {
		if o.UID != ownerRef.UID {
			cr.OwnerReferences = append(cr.OwnerReferences, o)
		}
	}

	if modified || len(cr.OwnerReferences) > len(existingCR.OwnerReferences) {
		logrus.Debugf("Updating %s cluster role", cr.Name)
		return k8sClient.Update(context.TODO(), cr)
	}
	return nil
}

// DeleteClusterRole deletes a cluster role if present and owned
func DeleteClusterRole(
	k8sClient client.Client,
	name string,
	owners ...metav1.OwnerReference,
) error {
	resource := types.NamespacedName{
		Name: name,
	}

	clusterRole := &rbacv1.ClusterRole{}
	err := k8sClient.Get(context.TODO(), resource, clusterRole)
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	newOwners := removeOwners(clusterRole.OwnerReferences, owners)

	// Do not delete the object if it does not have the owner that was passed;
	// even if the object has no owner
	if (len(clusterRole.OwnerReferences) == 0 && len(owners) > 0) ||
		(len(clusterRole.OwnerReferences) > 0 && len(clusterRole.OwnerReferences) == len(newOwners)) {
		logrus.Debugf("Cannot delete ClusterRole %s as it is not owned", name)
		return nil
	}

	if len(newOwners) == 0 {
		logrus.Debugf("Deleting %s ClusterRole", name)
		return k8sClient.Delete(context.TODO(), clusterRole)
	}
	clusterRole.OwnerReferences = newOwners
	logrus.Debugf("Disowning %s ClusterRole", name)
	return k8sClient.Update(context.TODO(), clusterRole)
}

// CreateOrUpdateClusterRoleBinding creates a cluster role binding if not present,
// else updates it if it has changed
func CreateOrUpdateClusterRoleBinding(
	k8sClient client.Client,
	crb *rbacv1.ClusterRoleBinding,
	ownerRef *metav1.OwnerReference,
) error {
	existingCRB := &rbacv1.ClusterRoleBinding{}
	err := k8sClient.Get(
		context.TODO(),
		types.NamespacedName{Name: crb.Name},
		existingCRB,
	)
	if errors.IsNotFound(err) {
		logrus.Debugf("Creating %s cluster role binding", crb.Name)
		return k8sClient.Create(context.TODO(), crb)
	} else if err != nil {
		return err
	}

	modified := !reflect.DeepEqual(crb.Subjects, existingCRB.Subjects) ||
		!reflect.DeepEqual(crb.RoleRef, existingCRB.RoleRef)

	for _, o := range existingCRB.OwnerReferences {
		if o.UID != ownerRef.UID {
			crb.OwnerReferences = append(crb.OwnerReferences, o)
		}
	}

	if modified || len(crb.OwnerReferences) > len(existingCRB.OwnerReferences) {
		logrus.Debugf("Updating %s cluster role binding", crb.Name)
		return k8sClient.Update(context.TODO(), crb)
	}
	return nil
}

// DeleteClusterRoleBinding deletes a cluster role binding if present and owned
func DeleteClusterRoleBinding(
	k8sClient client.Client,
	name string,
	owners ...metav1.OwnerReference,
) error {
	resource := types.NamespacedName{
		Name: name,
	}

	crb := &rbacv1.ClusterRoleBinding{}
	err := k8sClient.Get(context.TODO(), resource, crb)
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	newOwners := removeOwners(crb.OwnerReferences, owners)

	// Do not delete the object if it does not have the owner that was passed;
	// even if the object has no owner
	if (len(crb.OwnerReferences) == 0 && len(owners) > 0) ||
		(len(crb.OwnerReferences) > 0 && len(crb.OwnerReferences) == len(newOwners)) {
		logrus.Debugf("Cannot delete ClusterRoleBinding %s as it is not owned", name)
		return nil
	}

	if len(newOwners) == 0 {
		logrus.Debugf("Deleting %s ClusterRoleBinding", name)
		return k8sClient.Delete(context.TODO(), crb)
	}
	crb.OwnerReferences = newOwners
	logrus.Debugf("Disowning %s ClusterRoleBinding", name)
	return k8sClient.Update(context.TODO(), crb)
}

// CreateOrUpdateConfigMap creates a config map if not present,
// else updates it if it has changed
func CreateOrUpdateConfigMap(
	k8sClient client.Client,
	configMap *v1.ConfigMap,
	ownerRef *metav1.OwnerReference,
) error {
	existingConfigMap := &v1.ConfigMap{}
	err := k8sClient.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      configMap.Name,
			Namespace: configMap.Namespace,
		},
		existingConfigMap,
	)
	if errors.IsNotFound(err) {
		logrus.Debugf("Creating %v config map", configMap.Name)
		return k8sClient.Create(context.TODO(), configMap)
	} else if err != nil {
		return err
	}

	modified := !reflect.DeepEqual(configMap.Data, existingConfigMap.Data) ||
		!reflect.DeepEqual(configMap.BinaryData, existingConfigMap.BinaryData)

	for _, o := range existingConfigMap.OwnerReferences {
		if o.UID != ownerRef.UID {
			configMap.OwnerReferences = append(configMap.OwnerReferences, o)
		}
	}

	if modified || len(configMap.OwnerReferences) > len(existingConfigMap.OwnerReferences) {
		logrus.Debugf("Updating %v config map", configMap.Name)
		return k8sClient.Update(context.TODO(), configMap)
	}
	return nil
}

// DeleteConfigMap deletes a config map if present and owned
func DeleteConfigMap(
	k8sClient client.Client,
	name, namespace string,
	owners ...metav1.OwnerReference,
) error {
	resource := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	configMap := &v1.ConfigMap{}
	err := k8sClient.Get(context.TODO(), resource, configMap)
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	newOwners := removeOwners(configMap.OwnerReferences, owners)

	// Do not delete the object if it does not have the owner that was passed;
	// even if the object has no owner
	if (len(configMap.OwnerReferences) == 0 && len(owners) > 0) ||
		(len(configMap.OwnerReferences) > 0 && len(configMap.OwnerReferences) == len(newOwners)) {
		logrus.Debugf("Cannot delete ConfigMap %s/%s as it is not owned",
			namespace, name)
		return nil
	}

	if len(newOwners) == 0 {
		logrus.Debugf("Deleting %s/%s ConfigMap", namespace, name)
		return k8sClient.Delete(context.TODO(), configMap)
	}
	configMap.OwnerReferences = newOwners
	logrus.Debugf("Disowning %s/%s ConfigMap", namespace, name)
	return k8sClient.Update(context.TODO(), configMap)
}

// CreateOrUpdateStorageClass creates a storage class if not present,
// else updates it if it has changed
func CreateOrUpdateStorageClass(
	k8sClient client.Client,
	sc *storagev1.StorageClass,
	ownerRef *metav1.OwnerReference,
) error {
	existingSC := &storagev1.StorageClass{}
	err := k8sClient.Get(
		context.TODO(),
		types.NamespacedName{Name: sc.Name},
		existingSC,
	)
	if errors.IsNotFound(err) {
		logrus.Debugf("Creating %s storage class", sc.Name)
		return k8sClient.Create(context.TODO(), sc)
	} else if err != nil {
		return err
	}

	modified := !reflect.DeepEqual(sc.Provisioner, existingSC.Provisioner)

	for _, o := range existingSC.OwnerReferences {
		if o.UID != ownerRef.UID {
			sc.OwnerReferences = append(sc.OwnerReferences, o)
		}
	}

	if modified || len(sc.OwnerReferences) > len(existingSC.OwnerReferences) {
		logrus.Debugf("Updating %s storage class", sc.Name)
		return k8sClient.Update(context.TODO(), sc)
	}
	return nil
}

// DeleteStorageClass deletes a storage class if present and owned
func DeleteStorageClass(
	k8sClient client.Client,
	name string,
	owners ...metav1.OwnerReference,
) error {
	resource := types.NamespacedName{
		Name: name,
	}

	storageClass := &storagev1.StorageClass{}
	err := k8sClient.Get(context.TODO(), resource, storageClass)
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	newOwners := removeOwners(storageClass.OwnerReferences, owners)

	// Do not delete the object if it does not have the owner that was passed;
	// even if the object has no owner
	if (len(storageClass.OwnerReferences) == 0 && len(owners) > 0) ||
		(len(storageClass.OwnerReferences) > 0 && len(storageClass.OwnerReferences) == len(newOwners)) {
		logrus.Debugf("Cannot delete StorageClass %s as it is not owned", name)
		return nil
	}

	if len(newOwners) == 0 {
		logrus.Debugf("Deleting %s StorageClass", name)
		return k8sClient.Delete(context.TODO(), storageClass)
	}
	storageClass.OwnerReferences = newOwners
	logrus.Debugf("Disowning %s StorageClass", name)
	return k8sClient.Update(context.TODO(), storageClass)
}

// CreateOrUpdateCSIDriver creates a CSIDriver if not present,
// else updates it if it has changed
func CreateOrUpdateCSIDriver(
	k8sClient client.Client,
	driver *storagev1beta1.CSIDriver,
	ownerRef *metav1.OwnerReference,
) error {
	existingDriver := &storagev1beta1.CSIDriver{}
	err := k8sClient.Get(
		context.TODO(),
		types.NamespacedName{Name: driver.Name},
		existingDriver,
	)
	if errors.IsNotFound(err) {
		logrus.Debugf("Creating %s CSIDriver", driver.Name)
		return k8sClient.Create(context.TODO(), driver)
	} else if err != nil {
		return err
	}

	modified := !reflect.DeepEqual(driver.Spec, existingDriver.Spec)

	for _, o := range existingDriver.OwnerReferences {
		if o.UID != ownerRef.UID {
			driver.OwnerReferences = append(driver.OwnerReferences, o)
		}
	}

	if modified || len(driver.OwnerReferences) > len(existingDriver.OwnerReferences) {
		logrus.Debugf("Updating %s CSIDriver", driver.Name)
		return k8sClient.Update(context.TODO(), driver)
	}
	return nil
}

// DeleteCSIDriver deletes the CSIDriver object if present and owned
func DeleteCSIDriver(
	k8sClient client.Client,
	name string,
	owners ...metav1.OwnerReference,
) error {
	resource := types.NamespacedName{
		Name: name,
	}

	csiDriver := &storagev1beta1.CSIDriver{}
	err := k8sClient.Get(context.TODO(), resource, csiDriver)
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	newOwners := removeOwners(csiDriver.OwnerReferences, owners)

	// Do not delete the object if it does not have the owner that was passed;
	// even if the object has no owner
	if (len(csiDriver.OwnerReferences) == 0 && len(owners) > 0) ||
		(len(csiDriver.OwnerReferences) > 0 && len(csiDriver.OwnerReferences) == len(newOwners)) {
		logrus.Debugf("Cannot delete CSIDriver %s as it is not owned", name)
		return nil
	}

	if len(newOwners) == 0 {
		logrus.Debugf("Deleting %s CSIDriver", name)
		return k8sClient.Delete(context.TODO(), csiDriver)
	}
	csiDriver.OwnerReferences = newOwners
	logrus.Debugf("Disowning %s CSIDriver", name)
	return k8sClient.Update(context.TODO(), csiDriver)
}

// CreateOrUpdateService creates a service if not present, else updates it
func CreateOrUpdateService(
	k8sClient client.Client,
	service *v1.Service,
	ownerRef *metav1.OwnerReference,
) error {
	existingService := &v1.Service{}
	err := k8sClient.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      service.Name,
			Namespace: service.Namespace,
		},
		existingService,
	)
	if errors.IsNotFound(err) {
		logrus.Debugf("Creating %s service", service.Name)
		return k8sClient.Create(context.TODO(), service)
	} else if err != nil {
		return err
	}

	ownerRefs := make([]metav1.OwnerReference, 0)
	if ownerRef != nil {
		ownerRefs = append(ownerRefs, *ownerRef)
		for _, o := range existingService.OwnerReferences {
			if o.UID != ownerRef.UID {
				ownerRefs = append(ownerRefs, o)
			}
		}
	}

	if service.Spec.Type == "" {
		service.Spec.Type = v1.ServiceTypeClusterIP
	}

	modified := existingService.Spec.Type != service.Spec.Type ||
		!reflect.DeepEqual(existingService.Labels, service.Labels) ||
		!reflect.DeepEqual(existingService.Spec.Selector, service.Spec.Selector)

	portMapping := make(map[string]v1.ServicePort)
	for _, port := range service.Spec.Ports {
		portMapping[port.Name] = port
	}

	servicePorts := make([]v1.ServicePort, 0)
	for _, existingPort := range existingService.Spec.Ports {
		newPort, exists := portMapping[existingPort.Name]
		if !exists {
			// The port is no longer present in the new ports list
			modified = true
			continue
		}
		if existingPort.Protocol == "" {
			existingPort.Protocol = v1.ProtocolTCP
		}
		if newPort.Protocol == "" {
			newPort.Protocol = v1.ProtocolTCP
		}
		if existingPort.Port != newPort.Port ||
			!reflect.DeepEqual(existingPort.TargetPort, newPort.TargetPort) ||
			existingPort.Protocol != newPort.Protocol {
			modified = true
		}
		toUpdate := existingPort.DeepCopy()
		toUpdate.Port = newPort.Port
		toUpdate.TargetPort = newPort.TargetPort
		toUpdate.Protocol = newPort.Protocol
		if service.Spec.Type != v1.ServiceTypeLoadBalancer &&
			service.Spec.Type != v1.ServiceTypeNodePort {
			toUpdate.NodePort = int32(0)
		}
		servicePorts = append(servicePorts, *toUpdate)
		delete(portMapping, newPort.Name)
	}

	// Copy new ports that were not present in the service before
	for _, port := range portMapping {
		modified = true
		toUpdate := port.DeepCopy()
		servicePorts = append(servicePorts, *toUpdate)
	}

	if modified || len(ownerRefs) > len(existingService.OwnerReferences) {
		existingService.OwnerReferences = ownerRefs
		existingService.Labels = service.Labels
		existingService.Spec.Selector = service.Spec.Selector
		existingService.Spec.Type = service.Spec.Type
		existingService.Spec.Ports = servicePorts
		logrus.Debugf("Updating %s service", service.Name)
		return k8sClient.Update(context.TODO(), existingService)
	}
	return nil
}

// DeleteService deletes a service if present and owned
func DeleteService(
	k8sClient client.Client,
	name, namespace string,
	owners ...metav1.OwnerReference,
) error {
	resource := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	service := &v1.Service{}
	err := k8sClient.Get(context.TODO(), resource, service)
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	newOwners := removeOwners(service.OwnerReferences, owners)

	// Do not delete the object if it does not have the owner that was passed;
	// even if the object has no owner
	if (len(service.OwnerReferences) == 0 && len(owners) > 0) ||
		(len(service.OwnerReferences) > 0 && len(service.OwnerReferences) == len(newOwners)) {
		logrus.Debugf("Cannot delete Service %s/%s as it is not owned",
			namespace, name)
		return nil
	}

	if len(newOwners) == 0 {
		logrus.Debugf("Deleting %s/%s Service", namespace, name)
		return k8sClient.Delete(context.TODO(), service)
	}
	service.OwnerReferences = newOwners
	logrus.Debugf("Disowning %s/%s Service", namespace, name)
	return k8sClient.Update(context.TODO(), service)
}

// CreateOrUpdateDeployment creates a deployment if not present, else updates it
func CreateOrUpdateDeployment(
	k8sClient client.Client,
	deployment *appsv1.Deployment,
	ownerRef *metav1.OwnerReference,
) error {
	existingDeployment := &appsv1.Deployment{}
	err := k8sClient.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      deployment.Name,
			Namespace: deployment.Namespace,
		},
		existingDeployment,
	)
	if errors.IsNotFound(err) {
		logrus.Debugf("Creating %s deployment", deployment.Name)
		return k8sClient.Create(context.TODO(), deployment)
	} else if err != nil {
		return err
	}

	for _, o := range existingDeployment.OwnerReferences {
		if o.UID != ownerRef.UID {
			deployment.OwnerReferences = append(deployment.OwnerReferences, o)
		}
	}

	logrus.Debugf("Updating %s deployment", deployment.Name)
	return k8sClient.Update(context.TODO(), deployment)
}

// DeleteDeployment deletes a deployment if present and owned
func DeleteDeployment(
	k8sClient client.Client,
	name, namespace string,
	owners ...metav1.OwnerReference,
) error {
	resource := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	deployment := &appsv1.Deployment{}
	err := k8sClient.Get(context.TODO(), resource, deployment)
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	newOwners := removeOwners(deployment.OwnerReferences, owners)

	// Do not delete the object if it does not have the owner that was passed;
	// even if the object has no owner
	if (len(deployment.OwnerReferences) == 0 && len(owners) > 0) ||
		(len(deployment.OwnerReferences) > 0 && len(deployment.OwnerReferences) == len(newOwners)) {
		logrus.Debugf("Cannot delete Deployment %s/%s as it is not owned",
			namespace, name)
		return nil
	}

	if len(newOwners) == 0 {
		logrus.Debugf("Deleting %s/%s Deployment", namespace, name)
		return k8sClient.Delete(context.TODO(), deployment)
	}
	deployment.OwnerReferences = newOwners
	logrus.Debugf("Disowning %s/%s Deployment", namespace, name)
	return k8sClient.Update(context.TODO(), deployment)
}

// CreateOrUpdateStatefulSet creates a stateful set if not present, else updates it
func CreateOrUpdateStatefulSet(
	k8sClient client.Client,
	ss *appsv1.StatefulSet,
	ownerRef *metav1.OwnerReference,
) error {
	existingSS := &appsv1.StatefulSet{}
	err := k8sClient.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      ss.Name,
			Namespace: ss.Namespace,
		},
		existingSS,
	)
	if errors.IsNotFound(err) {
		logrus.Debugf("Creating %s stateful set", ss.Name)
		return k8sClient.Create(context.TODO(), ss)
	} else if err != nil {
		return err
	}

	for _, o := range existingSS.OwnerReferences {
		if o.UID != ownerRef.UID {
			ss.OwnerReferences = append(ss.OwnerReferences, o)
		}
	}

	logrus.Debugf("Updating %s stateful set", ss.Name)
	return k8sClient.Update(context.TODO(), ss)
}

// DeleteStatefulSet deletes a stateful set if present and owned
func DeleteStatefulSet(
	k8sClient client.Client,
	name, namespace string,
	owners ...metav1.OwnerReference,
) error {
	resource := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	statefulSet := &appsv1.StatefulSet{}
	err := k8sClient.Get(context.TODO(), resource, statefulSet)
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	newOwners := removeOwners(statefulSet.OwnerReferences, owners)

	// Do not delete the object if it does not have the owner that was passed;
	// even if the object has no owner
	if (len(statefulSet.OwnerReferences) == 0 && len(owners) > 0) ||
		(len(statefulSet.OwnerReferences) > 0 && len(statefulSet.OwnerReferences) == len(newOwners)) {
		logrus.Debugf("Cannot delete StatefulSet %s/%s as it is not owned",
			namespace, name)
		return nil
	}

	if len(newOwners) == 0 {
		logrus.Debugf("Deleting %s/%s StatefulSet", namespace, name)
		return k8sClient.Delete(context.TODO(), statefulSet)
	}
	statefulSet.OwnerReferences = newOwners
	logrus.Debugf("Disowning %s/%s StatefulSet", namespace, name)
	return k8sClient.Update(context.TODO(), statefulSet)
}

// CreateOrUpdateDaemonSet creates a daemon set if not present, else updates it
func CreateOrUpdateDaemonSet(
	k8sClient client.Client,
	ds *appsv1.DaemonSet,
	ownerRef *metav1.OwnerReference,
) error {
	existingDS := &appsv1.DaemonSet{}
	err := k8sClient.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      ds.Name,
			Namespace: ds.Namespace,
		},
		existingDS,
	)
	if errors.IsNotFound(err) {
		logrus.Debugf("Creating %s daemon set", ds.Name)
		return k8sClient.Create(context.TODO(), ds)
	} else if err != nil {
		return err
	}

	ownerRefPresent := false
	for _, o := range existingDS.OwnerReferences {
		if o.UID == ownerRef.UID {
			ownerRefPresent = true
			break
		}
	}
	if !ownerRefPresent {
		if existingDS.OwnerReferences == nil {
			existingDS.OwnerReferences = make([]metav1.OwnerReference, 0)
		}
		existingDS.OwnerReferences = append(existingDS.OwnerReferences, *ownerRef)
	}

	existingDS.Labels = ds.Labels
	existingDS.Spec = ds.Spec

	logrus.Debugf("Updating %s daemon set", ds.Name)
	return k8sClient.Update(context.TODO(), existingDS)
}

// CreateOrUpdateStorageNode creates a StorageNode if not present, else updates it
func CreateOrUpdateStorageNode(
	k8sClient client.Client,
	node *corev1alpha1.StorageNode,
	ownerRef *metav1.OwnerReference,
) error {
	existingNode := &corev1alpha1.StorageNode{}
	err := k8sClient.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      node.Name,
			Namespace: node.Namespace,
		},
		existingNode,
	)
	if errors.IsNotFound(err) {
		logrus.Debugf("Creating StorageNode %s/%s", node.Namespace, node.Name)
		return k8sClient.Create(context.TODO(), node)
	} else if err != nil {
		return err
	}

	ownerRefs := make([]metav1.OwnerReference, 0)
	if ownerRef != nil {
		ownerRefs = append(ownerRefs, *ownerRef)
		for _, o := range existingNode.OwnerReferences {
			if o.UID != ownerRef.UID {
				ownerRefs = append(ownerRefs, o)
			}
		}
	}

	modified := !reflect.DeepEqual(node.Status, existingNode.Status) ||
		!reflect.DeepEqual(node.Spec, existingNode.Spec)

	if modified || len(ownerRefs) > len(existingNode.OwnerReferences) {
		existingNode.Spec = node.Spec
		existingNode.Status = node.Status
		logrus.Debugf("Updating StorageNode %s/%s", node.Namespace, node.Name)
		if err := k8sClient.Update(context.TODO(), existingNode); err != nil {
			return err
		}
		return k8sClient.Status().Update(context.TODO(), existingNode)
	}
	return nil
}

// CreateOrUpdateServiceMonitor creates a ServiceMonitor if not present, else updates it
func CreateOrUpdateServiceMonitor(
	k8sClient client.Client,
	monitor *monitoringv1.ServiceMonitor,
	ownerRef *metav1.OwnerReference,
) error {
	existingMonitor := &monitoringv1.ServiceMonitor{}
	err := k8sClient.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      monitor.Name,
			Namespace: monitor.Namespace,
		},
		existingMonitor,
	)
	if errors.IsNotFound(err) {
		logrus.Debugf("Creating ServiceMonitor %s/%s", monitor.Namespace, monitor.Name)
		return k8sClient.Create(context.TODO(), monitor)
	} else if err != nil {
		return err
	}

	modified := !reflect.DeepEqual(monitor.Spec, existingMonitor.Spec)

	for _, o := range existingMonitor.OwnerReferences {
		if o.UID != ownerRef.UID {
			monitor.OwnerReferences = append(monitor.OwnerReferences, o)
		}
	}

	if modified || len(monitor.OwnerReferences) > len(existingMonitor.OwnerReferences) {
		monitor.ResourceVersion = existingMonitor.ResourceVersion
		logrus.Debugf("Updating ServiceMonitor %s/%s", monitor.Namespace, monitor.Name)
		return k8sClient.Update(context.TODO(), monitor)
	}
	return nil
}

// DeleteServiceMonitor deletes a storage class if present and owned
func DeleteServiceMonitor(
	k8sClient client.Client,
	name, namespace string,
	owners ...metav1.OwnerReference,
) error {
	resource := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	monitor := &monitoringv1.ServiceMonitor{}
	err := k8sClient.Get(context.TODO(), resource, monitor)
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	newOwners := removeOwners(monitor.OwnerReferences, owners)

	// Do not delete the object if it does not have the owner that was passed;
	// even if the object has no owner
	if (len(monitor.OwnerReferences) == 0 && len(owners) > 0) ||
		(len(monitor.OwnerReferences) > 0 && len(monitor.OwnerReferences) == len(newOwners)) {
		logrus.Debugf("Cannot delete ServiceMonitor %s/%s as it is not owned", namespace, name)
		return nil
	}

	if len(newOwners) == 0 {
		logrus.Debugf("Deleting %s/%s ServiceMonitor", namespace, name)
		return k8sClient.Delete(context.TODO(), monitor)
	}
	monitor.OwnerReferences = newOwners
	logrus.Debugf("Disowning %s/%s ServiceMonitor", namespace, name)
	return k8sClient.Update(context.TODO(), monitor)
}

// CreateOrUpdatePrometheusRule creates a PrometheusRule if not present, else updates it
func CreateOrUpdatePrometheusRule(
	k8sClient client.Client,
	rule *monitoringv1.PrometheusRule,
	ownerRef *metav1.OwnerReference,
) error {
	existingRule := &monitoringv1.PrometheusRule{}
	err := k8sClient.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      rule.Name,
			Namespace: rule.Namespace,
		},
		existingRule,
	)
	if errors.IsNotFound(err) {
		logrus.Debugf("Creating PrometheusRule %s/%s", rule.Namespace, rule.Name)
		return k8sClient.Create(context.TODO(), rule)
	} else if err != nil {
		return err
	}

	modified := !reflect.DeepEqual(rule.Spec, existingRule.Spec)

	for _, o := range existingRule.OwnerReferences {
		if o.UID != ownerRef.UID {
			rule.OwnerReferences = append(rule.OwnerReferences, o)
		}
	}

	if modified || len(rule.OwnerReferences) > len(existingRule.OwnerReferences) {
		rule.ResourceVersion = existingRule.ResourceVersion
		logrus.Debugf("Updating PrometheusRule %s/%s", rule.Namespace, rule.Name)
		return k8sClient.Update(context.TODO(), rule)
	}
	return nil
}

// DeletePrometheusRule deletes a storage class if present and owned
func DeletePrometheusRule(
	k8sClient client.Client,
	name, namespace string,
	owners ...metav1.OwnerReference,
) error {
	resource := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	rule := &monitoringv1.PrometheusRule{}
	err := k8sClient.Get(context.TODO(), resource, rule)
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	newOwners := removeOwners(rule.OwnerReferences, owners)

	// Do not delete the object if it does not have the owner that was passed;
	// even if the object has no owner
	if (len(rule.OwnerReferences) == 0 && len(owners) > 0) ||
		(len(rule.OwnerReferences) > 0 && len(rule.OwnerReferences) == len(newOwners)) {
		logrus.Debugf("Cannot delete PrometheusRule %s/%s as it is not owned", namespace, name)
		return nil
	}

	if len(newOwners) == 0 {
		logrus.Debugf("Deleting %s/%s PrometheusRule", namespace, name)
		return k8sClient.Delete(context.TODO(), rule)
	}
	rule.OwnerReferences = newOwners
	logrus.Debugf("Disowning %s/%s PrometheusRule", namespace, name)
	return k8sClient.Update(context.TODO(), rule)
}

// GetDaemonSetPods returns a list of pods for the given daemon set
func GetDaemonSetPods(
	k8sClient client.Client,
	ds *appsv1.DaemonSet,
) ([]*v1.Pod, error) {
	return GetPodsByOwner(k8sClient, ds.UID, ds.Namespace)
}

// GetPodsByOwner returns pods for a given owner and namespace
func GetPodsByOwner(
	k8sClient client.Client,
	ownerUID types.UID,
	namespace string,
) ([]*v1.Pod, error) {
	podList := &v1.PodList{}
	err := k8sClient.List(
		context.TODO(),
		podList,
		&client.ListOptions{
			Namespace: namespace,
		},
	)
	if err != nil {
		return nil, err
	}

	result := make([]*v1.Pod, 0)
	for _, pod := range podList.Items {
		for _, owner := range pod.OwnerReferences {
			if owner.UID == ownerUID {
				result = append(result, pod.DeepCopy())
			}
		}
	}
	return result, nil
}

func removeOwners(current, toBeDeleted []metav1.OwnerReference) []metav1.OwnerReference {
	toBeDeletedOwnerMap := make(map[types.UID]bool)
	for _, owner := range toBeDeleted {
		toBeDeletedOwnerMap[owner.UID] = true
	}
	newOwners := make([]metav1.OwnerReference, 0)
	for _, currOwner := range current {
		if _, exists := toBeDeletedOwnerMap[currOwner.UID]; !exists {
			newOwners = append(newOwners, currOwner)
		}
	}
	return newOwners
}
