package nbdkit

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
)

const imagesDir = "../../tests/images"

var (
	cirrosFileName          = "cirros-qcow2.img"
	diskimageTarFileName    = "cirros.tar"
	cirrosQCow2TarFileName  = "cirros.qcow2.tar"
	tinyCoreGz              = "tinyCore.iso.gz"
	tinyCoreXz              = "tinyCore.iso.xz"
	cirrosData, _           = readFile(filepath.Join(imagesDir, cirrosFileName))
	diskimageArchiveData, _ = readFile(diskimageTarFileName)
)

func createTestServer(imageDir string) *httptest.Server {
	return httptest.NewServer(http.FileServer(http.Dir(imageDir)))
}

// Read the contents of the file into a byte array, don't use this on really huge files.
func readFile(fileName string) ([]byte, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	result, err := ioutil.ReadAll(f)
	return result, err
}

var _ = Describe("Nbdkit curl with qemu info", func() {
	var (
		ts     *httptest.Server
		err    error
		tmpDir string
	)

	BeforeEach(func() {
		By("[BeforeEach] Creating test server")
		ts = createTestServer(imagesDir)
		tmpDir, err = ioutil.TempDir("", "ndb")
		Expect(err).NotTo(HaveOccurred())
		By("tmpDir: " + tmpDir)
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
		By("[AfterEach] closing test server")
		ts.Close()
	})
	It("should not fail with curl plugin and qemu-img info", func() {
		n := NewNbdkit(NbdkitCurlPlugin, filepath.Join(tmpDir, "ndb.pid"))
		u, err := url.Parse(ts.URL + "/" + cirrosFileName)
		Expect(err).ToNot(HaveOccurred())
		out, err := n.Info(u)
		Expect(out).NotTo(BeNil())
		Expect(out.Format).Should(Equal("qcow2"))
		Expect(err).NotTo(HaveOccurred())
	})

	It("should not fail with curl plugin, gzip filter and qemu-img info", func() {
		n := NewNbdkit(NbdkitCurlPlugin, filepath.Join(tmpDir, "ndb.pid"))
		n.AddFilter(NbdkitGzipFilter)
		u, err := url.Parse(ts.URL + "/" + tinyCoreGz)
		Expect(err).ToNot(HaveOccurred())
		out, err := n.Info(u)
		Expect(out).NotTo(BeNil())
		Expect(out.Format).Should(Equal("raw"))
		Expect(err).NotTo(HaveOccurred())
	})

	It("should not fail with curl plugin, xz filter and qemu-img info", func() {
		n := NewNbdkit(NbdkitCurlPlugin, filepath.Join(tmpDir, "ndb.pid"))
		n.AddFilter(NbdkitXzFilter)
		u, err := url.Parse(ts.URL + "/" + tinyCoreXz)
		Expect(err).ToNot(HaveOccurred())
		out, err := n.Info(u)
		Expect(out).NotTo(BeNil())
		Expect(out.Format).Should(Equal("raw"))
		Expect(err).NotTo(HaveOccurred())
	})
})
