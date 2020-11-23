package image

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/system"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"net/url"
	"strings"
)

var (
	nbdkitExecFunction = system.ExecWithLimits
)

type NbdkitPlugin string
type NbdkitFilter string

// Nbdkit plugins
const (
	NbdkitCurlPlugin NbdkitPlugin = "curl"
)

// Nbdkit filters
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
	var pluginArgs []string
	args := []string{"-r"}
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

func (n *Nbdkit) Info(url *url.URL) (*ImgInfo, error) {
	n.source = url
	qemuImgArgs := []string{"--output=json"}
	output, err := n.startNbdkitWithQemuImg("info", qemuImgArgs)
	if err != nil {
		return nil, errors.Errorf("%s, %s", output, err.Error())
	}
	return checkOutputQemuImgInfo(output, url.String())
}

func (n *Nbdkit) Validate(url *url.URL, availableSize int64, filesystemOverhead float64) error {
	info, err := n.Info(url)
	if err != nil {
		return err
	}
	return checkIfUrlIsValid(info, availableSize, filesystemOverhead, url.String())
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
	klog.V(3).Infof("Start nbdkit with: %v \n", argsNbdkit)
	return nbdkitExecFunction(nil, reportProgress, "nbdkit", argsNbdkit...)
}
