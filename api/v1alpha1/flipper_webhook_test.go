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

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Flipper Webhook", func() {

	Context("When creating Flipper under Defaulting Webhook", func() {
		It("Should fill in the default value if a required field is empty", func() {

			flipper := &Flipper{}
			flipper.Default()
			Expect(flipper.Spec.Interval).To(Equal("10m0s"))

		})
	})

	Context("When creating Flipper under Validating Webhook", func() {
		It("Should deny if a required field is empty", func() {

			flipper := &Flipper{}
			_, err := flipper.ValidateCreate()
			Expect(err).NotTo(BeNil())
			_, err = flipper.ValidateUpdate(flipper)
			Expect(err).NotTo(BeNil())

		})

		It("Should admit if all required fields are provided", func() {
			flipper := &Flipper{Spec: FlipperSpec{Interval: "15m", Match: MatchFilter{Labels: map[string]string{"a": "b"}}}}
			err := flipper.validateFlipper()
			Expect(err).To(BeNil())

		})
	})

})
