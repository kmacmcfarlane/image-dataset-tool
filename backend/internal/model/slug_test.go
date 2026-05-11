package model_test

import (
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
)

func TestModel(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Model Suite")
}

var _ = Describe("GenerateSlug", func() {
	DescribeTable("converts names to slugs",
		func(input, expected string) {
			Expect(model.GenerateSlug(input)).To(Equal(expected))
		},
		Entry("simple lowercase", "hello", "hello"),
		Entry("mixed case", "Hello World", "hello-world"),
		Entry("special chars", "Hello, World! (test)", "hello-world-test"),
		Entry("multiple spaces", "hello   world", "hello-world"),
		Entry("leading/trailing spaces", "  hello  ", "hello"),
		Entry("underscores", "hello_world_test", "hello-world-test"),
		Entry("hyphens preserved", "hello-world", "hello-world"),
		Entry("multiple hyphens collapsed", "hello---world", "hello-world"),
		Entry("numbers", "project 123", "project-123"),
		Entry("unicode accents", "café résumé", "cafe-resume"),
		Entry("unicode umlauts", "Über cool", "uber-cool"),
		Entry("pure unicode non-latin", "项目", ""),
		Entry("mixed unicode and ascii", "My 项目 Test", "my-test"),
		Entry("empty string", "", ""),
		Entry("only special chars", "!!!@@@", ""),
	)

	It("truncates to max 64 chars", func() {
		long := strings.Repeat("a", 100)
		slug := model.GenerateSlug(long)
		Expect(len(slug)).To(BeNumerically("<=", model.MaxSlugLength))
	})

	It("handles very long name with hyphens at truncation point", func() {
		// Build a name that will have a hyphen right at position 64
		name := strings.Repeat("ab ", 30) // "ab ab ab..." → slug "ab-ab-ab-..."
		slug := model.GenerateSlug(name)
		Expect(len(slug)).To(BeNumerically("<=", model.MaxSlugLength))
		Expect(slug).NotTo(HaveSuffix("-"))
	})
})
