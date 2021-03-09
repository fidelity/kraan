/*
Ideally, we should have one `<kind>_conroller_test.go` for each controller scaffolded and called in the `test_suite.go`.
So, let's write our example test for the AddonsLayer controller (`AddonsLayer_controller_test.go.`)
*/

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
// +kubebuilder:docs-gen:collapse=Apache License

/*
As usual, we start with the necessary imports. We also define some utility variables.
*/
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
	AddonsLayerName      = "apps"
	k8sList              = "List"
	timeout              = time.Second * 120
	interval             = time.Millisecond * 250
	addonsLayersFileName = "testdata/addons.json"
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

func listAddonsLayers(ctx context.Context, log logr.Logger) []kraanv1alpha1.AddonsLayer {
	listOptions := &client.ListOptions{}
	addonsLayerList := &kraanv1alpha1.AddonsLayerList{}
	Expect(k8sClient.List(ctx, addonsLayerList, listOptions)).Should(Succeed())
	log.Info("AddonsLayers list", "items", len(addonsLayerList.Items))

	return addonsLayerList.Items
}

func deleteAddonsLayers(ctx context.Context, log logr.Logger, addonsLayersItems []kraanv1alpha1.AddonsLayer) {
	var deletionPolicy metav1.DeletionPropagation = metav1.DeletePropagationOrphan //DeletePropagationForeground
	deleteOptions := &client.DeleteOptions{PropagationPolicy: &deletionPolicy}
	for _, addonsLayer := range addonsLayersItems {
		Expect(k8sClient.Delete(ctx, &addonsLayer, deleteOptions)).Should(Succeed()) // nolint: scopelint // ok
		log.Info("AddonsLayer deleted", "name", addonsLayer.Name)
	}
	log.Info("waiting for AddonsLayer to be deleted")
	Eventually(func() bool {
		time.Sleep(time.Second)
		return len(listAddonsLayers(ctx, log)) == 0
	}, timeout, interval).Should(BeTrue())
	log.Info("AddonsLayer created")
}

func createAddonsLayer(ctx context.Context, log logr.Logger, AddonsLayer *kraanv1alpha1.AddonsLayer) *kraanv1alpha1.AddonsLayer {
	createOptions := &client.CreateOptions{}
	Expect(k8sClient.Create(ctx, AddonsLayer, createOptions)).Should(Succeed())
	AddonsLayerLookupKey := types.NamespacedName{Name: AddonsLayerName}
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

func verifyAddonsLayer(ctx context.Context, log logr.Logger, addonsLayer *kraanv1alpha1.AddonsLayer, status string) {
	createdAddonsLayer := &kraanv1alpha1.AddonsLayer{}
	AddonsLayerLookupKey := types.NamespacedName{Name: AddonsLayerName}
	log.Info("waiting for AddonsLayer status to be expected value", "expected", status)
	Eventually(func() bool {
		err := k8sClient.Get(ctx, AddonsLayerLookupKey, createdAddonsLayer)
		if err != nil {
			return false
		}
		log.Info("AddonsLayer status", "actual", createdAddonsLayer.Status.State, "expected", status)

		return createdAddonsLayer.Status.State == status
	}, timeout, interval).Should(BeTrue())
	log.Info("AddonsLayer status achieved expected value", "expected", status)

	Expect(createdAddonsLayer.Spec.Hold).Should(Equal(addonsLayer.Spec.Hold))

	Expect(len(createdAddonsLayer.Status.Conditions)).Should(Equal(1))

	message := kraanv1alpha1.AddonsLayerDeployedMsg
	if status == kraanv1alpha1.HoldCondition {
		message = kraanv1alpha1.AddonsLayerHoldMsg
	}
	Expect(createdAddonsLayer.Status.Conditions).Should(Equal([]metav1.Condition{{
		Type:               status,
		Reason:             status,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: createdAddonsLayer.Status.Conditions[0].LastTransitionTime,
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

			By("By creating a new AddonsLayers")
			createdAddonsLayers := createAddonsLayers(ctx, log, addonsLayersFileName)
			verifyAddonsLayers(ctx, log, createdAddonsLayers)

		})
	})
})
