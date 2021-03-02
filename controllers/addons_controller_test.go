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
	"fmt"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	kraanv1alpha1 "github.com/fidelity/kraan/api/v1alpha1"
)

const (
	AddonsLayerName = "apps"

	timeout  = time.Second * 20
	interval = time.Millisecond * 250
)

// +kubebuilder:docs-gen:collapse=Imports

var _ = Describe("AddonsLayer controller", func() {
	createAddonsLayer := func(ctx context.Context, log logr.Logger, AddonsLayer *kraanv1alpha1.AddonsLayer) *kraanv1alpha1.AddonsLayer {
		Expect(k8sClient.Create(ctx, AddonsLayer)).Should(Succeed())
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

	verifyAddonsLayer := func(ctx context.Context, log logr.Logger, AddonsLayer, createdAddonsLayer *kraanv1alpha1.AddonsLayer, status string) {
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

		Expect(createdAddonsLayer.Spec.Hold).Should(Equal(AddonsLayer.Spec.Hold))

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

	Context("When creating AddonsLayer, wait for it to expire and then deleting it", func() {
		It("Should set AddonsLayer Status to Active, Create RoleBinding then set it to Expited and delete RoleBinding", func() {
			ctx := context.Background()
			logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter)))
			log := logf.Log.WithName("test-one")

			By("By creating a new AddonsLayer")
			AddonsLayer := &kraanv1alpha1.AddonsLayer{
				TypeMeta: metav1.TypeMeta{
					APIVersion: fmt.Sprintf("%s/%s", kraanv1alpha1.GroupVersion.Group, kraanv1alpha1.GroupVersion.Version),
					Kind:       kraanv1alpha1.AddonsLayerKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: AddonsLayerName,
				},
				Spec: kraanv1alpha1.AddonsLayerSpec{},
			}
			createdAddonsLayer := createAddonsLayer(ctx, log, AddonsLayer)
			verifyAddonsLayer(ctx, log, AddonsLayer, createdAddonsLayer, kraanv1alpha1.DeployedCondition)

		})
	})
})
