package nbdkit

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/image"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

const (
	matcherString = "\\((\\d?\\d\\.\\d\\d)\\/100%\\)"
)

var (
	progress = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "import_progress",
			Help: "The import progress in percentage",
		},
		[]string{"ownerUID"},
	)
	ownerUID string
)

type NbdkitPlugin string
type NbdkitFilter string

const (
	NbdkitCurlPlugin NbdkitPlugin = "curl"
)

const (
	NbdkitXzFilter   NbdkitFilter = "xz"
	NbdkitTarFilter  NbdkitFilter = "tar"
	NbdkitGzipFilter NbdkitFilter = "gzip"
)

func (p NbdkitPlugin) String() string {
	return string(p)
}
func (f NbdkitFilter) String() string {
	return string(f)
}

func init() {
	if err := prometheus.Register(progress); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			// A counter for that metric has been registered before.
			// Use the old counter from now on.
			progress = are.ExistingCollector.(*prometheus.CounterVec)
		} else {
			klog.Errorf("Unable to create prometheus progress counter")
		}
	}
	ownerUID, _ = util.ParseEnvVar(common.OwnerUID, false)
}

type Nbdkit struct {
	NbdPidFile string
	nbdkitArgs []string
	plugin     NbdkitPlugin
	pluginArgs []string
	filters    []NbdkitFilter
	source     *url.URL
}

func NewNbdkit(plugin NbdkitPlugin, nbdkitPidFile string) *Nbdkit {
	return &Nbdkit{
		NbdPidFile: nbdkitPidFile,
		plugin:     plugin,
	}
}

func NewNbdkitCurl(nbdkitPidFile, certDir string) *Nbdkit {
	args := []string{"-r"}
	pluginArgs := []string{"--verbose"}
	if certDir != "" {
		pluginArgs = append(pluginArgs, fmt.Sprintf("cainfo=%s/%s", certDir, "tls.crt"))
	}

	return &Nbdkit{
		NbdPidFile: nbdkitPidFile,
		plugin:     NbdkitCurlPlugin,
		nbdkitArgs: args,
		pluginArgs: pluginArgs,
	}
}

// AddFilter adds a nbdkit filter if it doesn't already exist
func (n *Nbdkit) AddFilter(filter NbdkitFilter) {
	for _, f := range n.filters {
		if f == filter {
			return
		}
	}
	n.filters = append(n.filters, filter)
}

func (n *Nbdkit) Info(url *url.URL) (*image.ImgInfo, error) {
	n.source = url
	qemuImgArgs := []string{"--output=json"}
	output, err := n.startNbdkitWithQemuImg("info", qemuImgArgs)
	if err != nil {
		return nil, errors.Errorf("%s, %s", output, err.Error())
	}
	var info image.ImgInfo
	err = json.Unmarshal(output, &info)
	if err != nil {
		klog.Errorf("Invalid JSON:\n%s\n", string(output))
		return nil, errors.Wrapf(err, "Invalid json for image %s", url.String())
	}
	return &info, nil
}

func (n *Nbdkit) ConvertToRawStream(url *url.URL, dest string) error {
	n.source = url
	qemuImgArgs := []string{"-p", "-O", "raw", dest, "-t", "none"}
	_, err := n.startNbdkitWithQemuImg("convert", qemuImgArgs)
	return err
}

func (n *Nbdkit) getSource() string {
	var source string
	switch n.plugin {
	case NbdkitCurlPlugin:
		source = fmt.Sprintf("url=%s", n.source.String())
	default:
		source = ""
	}
	return source
}

func (n *Nbdkit) startNbdkitWithQemuImg(qemuImgCmd string, qemuImgArgs []string) ([]byte, error) {
	argsNbdkit := []string{
		"--foreground",
		"--readonly",
		"--exit-with-parent",
		"-U", "-",
		"--pidfile", n.NbdPidFile,
	}
	// set filters
	for _, f := range n.filters {
		argsNbdkit = append(argsNbdkit, fmt.Sprintf("--filter=%s", f))
	}
	// set additional arguments
	for _, a := range n.nbdkitArgs {
		argsNbdkit = append(argsNbdkit, a)
	}
	// append nbdkit plugin arguments
	argsNbdkit = append(argsNbdkit, n.plugin.String(), strings.Join(n.pluginArgs, " "), n.getSource())
	// append qemu-img command
	argsNbdkit = append(argsNbdkit, "--run", fmt.Sprintf("qemu-img %s $nbd %v", qemuImgCmd, strings.Join(qemuImgArgs, " ")))
	nbdkit := exec.Command("nbdkit", argsNbdkit...)
	nbdkit.Env = os.Environ()
	out, err := nbdkit.CombinedOutput()
	if err != nil {
		klog.Errorf(string(out))
	}

	return out, err
}
