// +build integration

package controllers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	kraanv1alpha1 "github.com/fidelity/kraan/api/v1alpha1"
)

const (
	k8sList                    = "List"
	timeout                    = time.Second * 120
	interval                   = time.Second
	addonsLayersFileName       = "testdata/addons.json"
	addonsLayersOrphanFileName = "testdata/addons-orphan.json"
)

func getAddonsFromFiles(fileNames ...string) *kraanv1alpha1.AddonsLayerList {
	addonsLayersList := &kraanv1alpha1.AddonsLayerList{
		TypeMeta: metav1.TypeMeta{
			Kind:       k8sList,
			APIVersion: fmt.Sprintf("%s/%s", kraanv1alpha1.GroupVersion.Version, kraanv1alpha1.GroupVersion.Version),
		},
		Items: make([]kraanv1alpha1.AddonsLayer, 0, 10),
	}

	for _, fileName := range fileNames {
		buffer, err := ioutil.ReadFile(fileName)
		Expect(err).NotTo(HaveOccurred())

		addons := &kraanv1alpha1.AddonsLayerList{}

		err = json.Unmarshal(buffer, addons)
		Expect(err).NotTo(HaveOccurred())

		addonsLayersList.Items = append(addonsLayersList.Items, addons.Items...)
	}

	return addonsLayersList
}

func listAddonsLayers(ctx context.Context, log logr.Logger) []*kraanv1alpha1.AddonsLayer {
	listOptions := &client.ListOptions{}
	addonsLayerList := &kraanv1alpha1.AddonsLayerList{}
	Expect(k8sClient.List(ctx, addonsLayerList, listOptions)).Should(Succeed())
	log.Info("AddonsLayers list", "items", len(addonsLayerList.Items))
	addonsLayers := make([]*kraanv1alpha1.AddonsLayer, len(addonsLayerList.Items))
	for index, addonsLayer := range addonsLayerList.Items {
		addonsLayers[index] = &kraanv1alpha1.AddonsLayer{}
		addonsLayer.DeepCopyInto(addonsLayers[index])
		log.Info("AddonsLayer added to list", "name", addonsLayer.Name)
	}
	return addonsLayers
}

func deleteAddonsLayers(ctx context.Context, log logr.Logger, addonsLayers []*kraanv1alpha1.AddonsLayer) {
	//var deletionPolicy metav1.DeletionPropagation = metav1.DeletePropagationOrphan
	deleteOptions := &client.DeleteOptions{} // PropagationPolicy: &deletionPolicy
	for _, addonsLayer := range addonsLayers {
		Expect(k8sClient.Delete(ctx, addonsLayer, deleteOptions)).Should(Succeed())
		log.Info("AddonsLayer deleted", "name", addonsLayer.Name)
	}
	log.Info("waiting for AddonsLayers to be deleted")
	Eventually(func() bool {
		time.Sleep(time.Second)
		return len(listAddonsLayers(ctx, log)) == 0
	}, timeout, interval).Should(BeTrue())
	log.Info("AddonsLayers deleted")
}

func getAddonsLayer(ctx context.Context, log logr.Logger, addonsLayerName string) *kraanv1alpha1.AddonsLayer {
	addonsLayer := &kraanv1alpha1.AddonsLayer{}
	addonsLayerLookupKey := types.NamespacedName{Name: addonsLayerName}
	Expect(k8sClient.Get(ctx, addonsLayerLookupKey, addonsLayer)).Should(Succeed())
	log.Info("AddonsLayer retrieved")

	return addonsLayer
}

func createAddonsLayer(ctx context.Context, log logr.Logger, addonsLayer *kraanv1alpha1.AddonsLayer) *kraanv1alpha1.AddonsLayer {
	createOptions := &client.CreateOptions{}
	Expect(k8sClient.Create(ctx, addonsLayer, createOptions)).Should(Succeed())
	AddonsLayerLookupKey := types.NamespacedName{Name: addonsLayer.Name}
	createdAddonsLayer := &kraanv1alpha1.AddonsLayer{}

	log.Info("waiting for AddonsLayer to be created")
	Eventually(func() bool {
		err := k8sClient.Get(ctx, AddonsLayerLookupKey, createdAddonsLayer)

		return err == nil
	}, timeout, interval).Should(BeTrue())
	log.Info("AddonsLayer created")

	return createdAddonsLayer
}

func createAddonsLayers(ctx context.Context, log logr.Logger, dataFileNames ...string) []*kraanv1alpha1.AddonsLayer {
	addonsLayersItems := getAddonsFromFiles(dataFileNames...).Items
	addonsLayers := make([]*kraanv1alpha1.AddonsLayer, len(addonsLayersItems))

	for index, addonsLayer := range addonsLayersItems {
		addonsLayers[index] = createAddonsLayer(ctx, log, &addonsLayer) // nolint: scopelint // ok
	}

	return addonsLayers
}

func updateAddonsLayer(ctx context.Context, log logr.Logger, updatedAddonsLayer *kraanv1alpha1.AddonsLayer) *kraanv1alpha1.AddonsLayer {
	addonsLayer := getAddonsLayer(ctx, log, updatedAddonsLayer.Name)
	updatedAddonsLayer.Spec.DeepCopyInto(&addonsLayer.Spec)
	updateOptions := &client.UpdateOptions{}
	Expect(k8sClient.Update(ctx, addonsLayer, updateOptions)).Should(Succeed())
	addonsLayerLookupKey := types.NamespacedName{Name: addonsLayer.Name}

	log.Info("Checking AddonsLayer to is updated")
	Eventually(func() bool {
		err := k8sClient.Get(ctx, addonsLayerLookupKey, updatedAddonsLayer)

		return err == nil && updatedAddonsLayer.Spec.Source.Path == addonsLayer.Spec.Source.Path
	}, timeout, interval).Should(BeTrue())
	log.Info("AddonsLayer created")

	return updatedAddonsLayer
}

func updateAddonsLayers(ctx context.Context, log logr.Logger, dataFileNames ...string) []*kraanv1alpha1.AddonsLayer {
	addonsLayersItems := getAddonsFromFiles(dataFileNames...).Items
	addonsLayers := make([]*kraanv1alpha1.AddonsLayer, len(addonsLayersItems))

	for index, addonsLayer := range addonsLayersItems {
		addonsLayers[index] = updateAddonsLayer(ctx, log, &addonsLayer) // nolint: scopelint // ok
	}

	return addonsLayers
}

func verifyAddonsLayer(ctx context.Context, log logr.Logger, addonsLayer *kraanv1alpha1.AddonsLayer, status string) {
	retrievedAddonsLayer := &kraanv1alpha1.AddonsLayer{}
	addonsLayerLookupKey := types.NamespacedName{Name: addonsLayer.Name}
	log.Info("waiting for AddonsLayer status to be expected value", "expected", status)
	Eventually(func() bool {
		err := k8sClient.Get(ctx, addonsLayerLookupKey, retrievedAddonsLayer)
		if err != nil {
			return false
		}
		log.Info("AddonsLayer status", "name", retrievedAddonsLayer.Name, "actual", retrievedAddonsLayer.Status.State, "expected", status)

		return retrievedAddonsLayer.Status.State == status
	}, timeout, interval).Should(BeTrue())
	log.Info("AddonsLayer status achieved expected value", "expected", status)

	Expect(retrievedAddonsLayer.Spec.Hold).Should(Equal(addonsLayer.Spec.Hold))

	Expect(len(retrievedAddonsLayer.Status.Conditions)).Should(Equal(1))

	message := "AddonsLayer version 0.1.01 is Deployed, All HelmReleases deployed"
	if status == kraanv1alpha1.HoldCondition {
		message = kraanv1alpha1.AddonsLayerHoldMsg
	}
	Expect(retrievedAddonsLayer.Status.Conditions).Should(Equal([]metav1.Condition{{
		Type:               status,
		Reason:             status,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: retrievedAddonsLayer.Status.Conditions[0].LastTransitionTime,
		Message:            message,
	}}))
}

func verifyAddonsLayers(ctx context.Context, log logr.Logger, addonsLayers []*kraanv1alpha1.AddonsLayer) {
	for _, addonsLayer := range addonsLayers {
		verifyAddonsLayer(ctx, log, addonsLayer, "Deployed")
	}
}

// +kubebuilder:docs-gen:collapse=Imports

var _ = Describe("AddonsLayer controller", func() {
	Context("When creating AddonsLayers, wait for them to be deployed state", func() {
		It("Should set AddonsLayers Status to Deployed and deploy the HelmReleases defined by each AddonsLayer", func() {
			ctx := context.Background()
			logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter)))
			log := logf.Log.WithName("test-one")

			By("By deleting any existing addons")
			deleteAddonsLayers(ctx, log, listAddonsLayers(ctx, log))

			By("By creating new AddonsLayers")
			createAddonsLayers(ctx, log, addonsLayersFileName)
			verifyAddonsLayers(ctx, log, listAddonsLayers(ctx, log))

		})
	})
	Context("When updating AddonsLayers source to move a HelmRelease from one layer to another, wait for them to be deployed state", func() {
		It("Should not redeploy the HelmReleases moved between layers", func() {
			ctx := context.Background()
			logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter)))
			log := logf.Log.WithName("test-two")

			By("By updating AddonsLayers to use different path in repository")
			updateAddonsLayers(ctx, log, addonsLayersOrphanFileName)
			time.Sleep(time.Second * 10)
			verifyAddonsLayers(ctx, log, listAddonsLayers(ctx, log))
			//verifyHelmReleaseNotRedeployed(ctx, log, "bootstrap", "microservice-2")

		})
	})
})
