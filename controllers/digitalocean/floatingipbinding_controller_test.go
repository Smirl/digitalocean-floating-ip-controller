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
	"time"

	"github.com/digitalocean/godo"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	digitaloceanv1beta1 "github.com/smirl/digitalocean-floating-ip-controller/apis/digitalocean/v1beta1"
)

type floatingIPRoot struct {
	FloatingIP *godo.FloatingIP `json:"floating_ip"`
}
type actionRoot struct {
	Event *godo.Action `json:"action"`
}

const TestIP string = "1.2.3.4"

var (
	node1 = v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node1"},
		Spec:       v1.NodeSpec{ProviderID: "digitalocean://12345678"},
	}
	getResponseUnassigned = floatingIPRoot{
		FloatingIP: &godo.FloatingIP{IP: TestIP},
	}
	// getResponseAssigned = floatingIPRoot{
	// 	FloatingIP: &godo.FloatingIP{IP: TestIP, Droplet: &godo.Droplet{ID: 12345678}},
	// }
	// assignResponse = actionRoot{Event: &godo.Action{}}
)

var _ = Context("Floating IP Controller", func() {

	Describe("when a new resources is created", func() {
		It("should assign a floating ip to a node", func() {

			By("Adding Node")
			Expect(k8sClient.Create(ctx, &node1)).Should(Succeed(), "failed to create test binding")

			By("Adding httpmocks")
			httpmock.RegisterResponder(
				"GET",
				"/v2/floating_ips/1.2.3.4",
				httpmock.NewJsonResponderOrPanic(200, getResponseUnassigned),
			)
			httpmock.RegisterResponder(
				"POST",
				"/v2/floating_ips/1.2.3.4/actions",
				httpmock.NewJsonResponderOrPanic(200, getResponseUnassigned),
			)

			By("Creating a binding")
			key := client.ObjectKey{
				Name:      "floatingipbinding-sample",
				Namespace: "default",
			}
			binding := &digitaloceanv1beta1.FloatingIPBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: digitaloceanv1beta1.FloatingIPBindingSpec{
					FloatingIP: "1.2.3.4",
				},
			}
			Expect(k8sClient.Create(ctx, binding)).Should(Succeed(), "failed to create test binding")
			Expect(httpmock.GetCallCountInfo()).To(HaveLen(2))

			By("Checking the status has updated")
			Eventually(
				func() bool {
					binding := &digitaloceanv1beta1.FloatingIPBinding{}
					Expect(k8sClient.Get(ctx, key, binding)).Should(Succeed(), "failed to get binding")
					return binding.Status.AssignedDropletName == "node1" && binding.Status.AssignedDropletID == 12345678
				},
				time.Second*1, time.Millisecond*100,
			).Should(BeTrue(), "Certificate should be set")
		})

	})

})
