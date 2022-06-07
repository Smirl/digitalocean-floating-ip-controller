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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	digitaloceanv1beta1 "github.com/smirl/digitalocean-floating-ip-controller/apis/digitalocean/v1beta1"
)

var _ = Context("Floating IP Controller", func() {

	Describe("when a new resources is created", func() {
		It("should create a floating ip", func() {
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
		})
	})

})
