package integration_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/occam"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/paketo-buildpacks/occam/matchers"
)

func testPipenv(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect     = NewWithT(t).Expect
		Eventually = NewWithT(t).Eventually

		pack   occam.Pack
		docker occam.Docker
	)

	it.Before(func() {
		pack = occam.NewPack()
		docker = occam.NewDocker()
	})

	context("when building a pipenv app", func() {
		var (
			image     occam.Image
			container occam.Container

			name   string
			source string
		)

		it.Before(func() {
			var err error
			name, err = occam.RandomName()
			Expect(err).NotTo(HaveOccurred())
		})

		it.After(func() {
			Expect(docker.Container.Remove.Execute(container.ID)).To(Succeed())
			Expect(docker.Image.Remove.Execute(image.ID)).To(Succeed())
			Expect(docker.Volume.Remove.Execute(occam.CacheVolumeNames(name))).To(Succeed())
			Expect(os.RemoveAll(source)).To(Succeed())
		})

		it("creates a working OCI image with a start command", func() {
			var err error
			source, err = occam.Source(filepath.Join("testdata", "pipenv"))
			Expect(err).NotTo(HaveOccurred())

			var logs fmt.Stringer
			image, logs, err = pack.WithNoColor().Build.
				WithBuildpacks(pythonBuildpack).
				WithPullPolicy("never").
				WithEnv(map[string]string{
					"BPE_SOME_VARIABLE": "some-value",
					"BP_IMAGE_LABELS":   "some-label=some-value",
				}).
				Execute(name, source)
			Expect(err).NotTo(HaveOccurred(), logs.String())

			container, err = docker.Container.Run.
				WithEnv(map[string]string{"PORT": "8080"}).
				WithPublish("8080").
				WithPublishAll().
				Execute(image.ID)
			Expect(err).NotTo(HaveOccurred())

			Eventually(container).Should(BeAvailable())

			response, err := http.Get(fmt.Sprintf("http://localhost:%s", container.HostPort("8080")))
			Expect(err).NotTo(HaveOccurred())
			defer response.Body.Close()

			Expect(response.StatusCode).To(Equal(http.StatusOK))

			content, err := ioutil.ReadAll(response.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("Hello, World with pipenv!"))

			Expect(logs).To(ContainLines(ContainSubstring("CPython Buildpack")))
			Expect(logs).To(ContainLines(ContainSubstring("Pip Buildpack")))
			Expect(logs).To(ContainLines(ContainSubstring("Pipenv Buildpack")))
			Expect(logs).To(ContainLines(ContainSubstring("Pipenv Install Buildpack")))
			Expect(logs).To(ContainLines(ContainSubstring("Python Start Buildpack")))
			Expect(logs).To(ContainLines(ContainSubstring("Procfile Buildpack")))
			Expect(logs).To(ContainLines(ContainSubstring("Environment Variables Buildpack")))
			Expect(logs).To(ContainLines(ContainSubstring("Image Labels Buildpack")))

			Expect(image.Buildpacks[6].Key).To(Equal("paketo-buildpacks/environment-variables"))
			Expect(image.Buildpacks[6].Layers["environment-variables"].Metadata["variables"]).To(Equal(map[string]interface{}{"SOME_VARIABLE": "some-value"}))
			Expect(image.Labels["some-label"]).To(Equal("some-value"))
		})
	})
}
