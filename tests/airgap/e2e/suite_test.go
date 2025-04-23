/*
Copyright © 2022 - 2025 SUSE LLC

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

package e2e_test

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/ele-testhelpers/kubectl"
	"github.com/rancher-sandbox/ele-testhelpers/rancher"
	"github.com/rancher-sandbox/ele-testhelpers/tools"
	"github.com/rancher/elemental/tests/e2e/helpers/elemental"
)

const (
	airgapBuildScript     = "../scripts/build-airgap"
	ciTokenYaml           = "../assets/local-kubeconfig-token-skel.yaml"
	configPrivateCAScript = "../scripts/config-private-ca"
	installConfigYaml     = "../../install-config.yaml"
	installHardenedScript = "../scripts/config-hardened"
	installVMScript       = "../scripts/install-vm"
	localKubeconfigYaml   = "../assets/local-kubeconfig-skel.yaml"
	localStorageYaml      = "../assets/local-storage.yaml"
	metallbRscYaml        = "../assets/metallb_rsc.yaml"
	sshConfigFile         = "../assets/ssh_config"
	upgradeSkelYaml       = "../assets/upgrade_skel.yaml"
	userName              = "root"
	userPassword          = "r0s@pwd1"
	vmNameRoot            = "node"
)

var (
	backupRestoreVersion      string
	caType                    string
	certManagerVersion        string
	clusterName               string
	clusterNS                 string
	clusterType               string
	clusterYaml               string
	elementalSupport          string
	emulateTPM                bool
	forceDowngrade            bool
	isoBoot                   bool
	k3sVersion                string
	netDefaultFileName        string
	numberOfVMs               int
	operatorRepo              string
	rancherChannel            string
	rancherHeadVersion        string
	rancherHostname           string
	rancherLogCollector       string
	rancherVersion            string
	rancherUpgrade            string
	rancherUpgradeChannel     string
	rancherUpgradeHeadVersion string
	rancherUpgradeVersion     string
	sequential                bool
	sshdConfigFile            string
	usedNodes                 int
	vmIndex                   int
	vmName                    string
)

func CheckBackupRestore(v string) {
	Eventually(func() string {
		out, _ := kubectl.RunWithoutErr("logs", "-l app.kubernetes.io/name=rancher-backup",
			"--tail=-1", "--since=5m",
			"--namespace", "cattle-resources-system")
		return out
	}, tools.SetTimeout(5*time.Minute), 10*time.Second).Should(ContainSubstring(v))
}

/*
Check that Cluster resource has been correctly created
  - @param ns Namespace where the cluster is deployed
  - @param cn Cluster resource name
  - @returns Nothing, the function will fail through Ginkgo in case of issue
*/
func CheckCreatedCluster(ns, cn string) {
	// Check that the cluster is correctly created
	Eventually(func() string {
		out, _ := kubectl.RunWithoutErr("get", "cluster.v1.provisioning.cattle.io",
			"--namespace", ns,
			cn, "-o", "jsonpath={.metadata.name}")
		return out
	}, tools.SetTimeout(3*time.Minute), 5*time.Second).Should(Equal(cn))
}

/*
Check that Registration resource has been correctly created
  - @param ns Namespace where the cluster is deployed
  - @param rn MachineRegistration resource name
  - @returns Nothing, the function will fail through Ginkgo in case of issue
*/
func CheckCreatedRegistration(ns, rn string) {
	Eventually(func() string {
		out, _ := kubectl.RunWithoutErr("get", "MachineRegistration",
			"--namespace", ns,
			"-o", "jsonpath={.items[*].metadata.name}")
		return out
	}, tools.SetTimeout(3*time.Minute), 5*time.Second).Should(ContainSubstring(rn))
}

/*
Check that a SelectorTemplate resource has been correctly created
  - @param ns Namespace where the cluster is deployed
  - @param sn Selector name
  - @returns Nothing, the function will fail through Ginkgo in case of issue
*/
func CheckCreatedSelectorTemplate(ns, sn string) {
	Eventually(func() string {
		out, _ := kubectl.RunWithoutErr("get", "MachineInventorySelectorTemplate",
			"--namespace", ns,
			"-o", "jsonpath={.items[*].metadata.name}")
		return out
	}, tools.SetTimeout(3*time.Minute), 5*time.Second).Should(ContainSubstring(sn))
}

/*
Check SSH connection
  - @param cl Client (node) informations
  - @returns Nothing, the function will fail through Ginkgo in case of issue
*/
func CheckSSH(cl *tools.Client) {
	Eventually(func() string {
		out, _ := cl.RunSSH("echo SSH_OK")
		return strings.Trim(out, "\n")
	}, tools.SetTimeout(10*time.Minute), 5*time.Second).Should(Equal("SSH_OK"))
}

/*
Get Elemental node information
  - @param hn Node hostname
  - @returns Client structure and MAC address
*/
func GetNodeInfo(hn string) (*tools.Client, string) {
	// Get network data
	data, err := rancher.GetHostNetConfig(".*name=\""+hn+"\".*", netDefaultFileName)
	Expect(err).To(Not(HaveOccurred()))

	// Set 'client' to be able to access the node through SSH
	c := &tools.Client{
		Host:     string(data.IP) + ":22",
		Username: userName,
		Password: userPassword,
	}

	return c, data.Mac
}

/*
Get Elemental node IP address
  - @param hn Node hostname
  - @returns IP address
*/
func GetNodeIP(hn string) string {
	// Get network data
	data, err := rancher.GetHostNetConfig(".*name=\""+hn+"\".*", netDefaultFileName)
	Expect(err).To(Not(HaveOccurred()))

	return data.IP
}

/*
Install CertManager
  - @param k kubectl structure
  - @returns Nothing, the function will fail through Ginkgo in case of issue
*/
func InstallCertManager(k *kubectl.Kubectl) {
	RunHelmCmdWithRetry("repo", "add", "jetstack", "https://charts.jetstack.io")
	RunHelmCmdWithRetry("repo", "update")

	// Set flags for cert-manager installation
	flags := []string{
		"upgrade", "--install", "cert-manager", "jetstack/cert-manager",
		"--namespace", "cert-manager",
		"--create-namespace",
		"--set", "installCRDs=true",
		"--wait", "--wait-for-jobs",
	}

	if clusterType == "hardened" {
		flags = append(flags, "--version", certManagerVersion)
	}

	RunHelmCmdWithRetry(flags...)

	checkList := [][]string{
		{"cert-manager", "app.kubernetes.io/component=controller"},
		{"cert-manager", "app.kubernetes.io/component=webhook"},
		{"cert-manager", "app.kubernetes.io/component=cainjector"},
	}
	Eventually(func() error {
		return rancher.CheckPod(k, checkList)
	}, tools.SetTimeout(4*time.Minute), 30*time.Second).Should(Not(HaveOccurred()))
}

/*
Install K3s
  - @returns Nothing, the function will fail through Ginkgo in case of issue
*/
func InstallK3s() {
	// Get K3s installation script
	fileName := "k3s-install.sh"
	Eventually(func() error {
		return tools.GetFileFromURL("https://get.k3s.io", fileName, true)
	}, tools.SetTimeout(2*time.Minute), 10*time.Second).ShouldNot(HaveOccurred())

	// Set command and arguments
	installCmd := exec.Command("sh", fileName)
	installCmd.Env = append(os.Environ(), "INSTALL_K3S_EXEC=--disable metrics-server")

	// Retry in case of (sporadic) failure...
	count := 1
	Eventually(func() error {
		// Execute K3s installation
		out, err := installCmd.CombinedOutput()
		GinkgoWriter.Printf("K3s installation loop %d:\n%s\n", count, out)
		count++
		return err
	}, tools.SetTimeout(2*time.Minute), 5*time.Second).Should(Not(HaveOccurred()))
}

/*
Install Rancher Manager
  - @param k kubectl structure
  - @returns Nothing, the function will fail through Ginkgo in case of issue
*/
func InstallRancher(k *kubectl.Kubectl) {
	Eventually(func() error {
		return rancher.DeployRancherManager(rancherHostname, rancherChannel, rancherVersion, rancherHeadVersion, caType, "proxy")
	}, tools.SetTimeout(5*time.Minute), 1*time.Minute).Should(Not(HaveOccurred()))

	checkList := [][]string{
		{"cattle-system", "app=rancher"},
		{"cattle-system", "app=rancher-webhook"},
		{"cattle-fleet-local-system", "app=fleet-agent"},
		{"cattle-provisioning-capi-system", "control-plane=controller-manager"},
	}
	Eventually(func() error {
		return rancher.CheckPod(k, checkList)
	}, tools.SetTimeout(10*time.Minute), 30*time.Second).Should(Not(HaveOccurred()))
}

/*
Execute RunHelmBinaryWithCustomErr within a loop with timeout
  - @param s options to pass to RunHelmBinaryWithCustomErr command
  - @returns Nothing, the function will fail through Ginkgo in case of issue
*/
func RunHelmCmdWithRetry(s ...string) {
	Eventually(func() error {
		return kubectl.RunHelmBinaryWithCustomErr(s...)
	}, tools.SetTimeout(2*time.Minute), 20*time.Second).Should(Not(HaveOccurred()))
}

/*
Execute SSH command with retry
  - @param cl Client (node) informations
  - @param cmd Command to execute
  - @returns result of the executed command
*/
func RunSSHWithRetry(cl *tools.Client, cmd string) string {
	var err error
	var out string

	Eventually(func() error {
		out, err = cl.RunSSH(cmd)
		return err
	}, tools.SetTimeout(2*time.Minute), 20*time.Second).Should(Not(HaveOccurred()))

	return out
}

/*
Start K3s
  - @returns Nothing, the function will fail through Ginkgo in case of issue
*/
func StartK3s() {
	err := exec.Command("sudo", "systemctl", "start", "k3s").Run()
	Expect(err).To(Not(HaveOccurred()))
}

/*
Wait for K3s to start
  - @param k kubectl structure
  - @returns Nothing, the function will fail through Ginkgo in case of issue
*/
func WaitForK3s(k *kubectl.Kubectl) {
	// Check Pod(s)
	checkList := [][]string{
		{"kube-system", "app=local-path-provisioner"},
		{"kube-system", "k8s-app=kube-dns"},
		{"kube-system", "app.kubernetes.io/name=traefik"},
		{"kube-system", "svccontroller.k3s.cattle.io/svcname=traefik"},
	}
	Eventually(func() error {
		return rancher.CheckPod(k, checkList)
	}, tools.SetTimeout(4*time.Minute), 30*time.Second).Should(Not(HaveOccurred()))

	// Check DaemonSet(s)
	checkList = [][]string{
		{"kube-system", "svccontroller.k3s.cattle.io/svcname=traefik"},
	}
	Eventually(func() error {
		return rancher.CheckDaemonSet(k, checkList)
	}, tools.SetTimeout(4*time.Minute), 30*time.Second).Should(Not(HaveOccurred()))
}

func FailWithReport(message string, callerSkip ...int) {
	// Ensures the correct line numbers are reported
	Fail(message, callerSkip[0]+1)
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(FailWithReport)
	RunSpecs(t, "Elemental End-To-End Test Suite")
}

var _ = BeforeSuite(func() {
	caType = os.Getenv("CA_TYPE")
	certManagerVersion = os.Getenv("CERT_MANAGER_VERSION")
	index := os.Getenv("VM_INDEX")
	k3sVersion = os.Getenv("K3S_VERSION")
	number := os.Getenv("VM_NUMBERS")
	netDefaultFileName = "../assets/net-default-airgap.xml"
	operatorRepo = os.Getenv("OPERATOR_REPO")
	rancherHostname = os.Getenv("PUBLIC_FQDN")
	rancherLogCollector = os.Getenv("RANCHER_LOG_COLLECTOR")
	rancherVersion = os.Getenv("RANCHER_VERSION")
	rancherUpgrade = os.Getenv("RANCHER_UPGRADE")
	seqString := os.Getenv("SEQUENTIAL")

	// Only if VM_INDEX is set
	if index != "" {
		var err error
		vmIndex, err = strconv.Atoi(index)
		Expect(err).To(Not(HaveOccurred()))

		// Set default hostname
		vmName = elemental.SetHostname(vmNameRoot, vmIndex)
	} else {
		// Default value for vmIndex
		vmIndex = 0
	}

	// Only if VM_NUMBER is set
	if number != "" {
		var err error
		numberOfVMs, err = strconv.Atoi(number)
		Expect(err).To(Not(HaveOccurred()))
	} else {
		// By default set to vmIndex
		numberOfVMs = vmIndex
	}

	// Extract Rancher Manager channel/version to upgrade
	if rancherUpgrade != "" {
		// Split rancherUpgrade and reset it
		s := strings.Split(rancherUpgrade, "/")

		// Get needed informations
		rancherUpgradeChannel = s[0]
		if len(s) > 1 {
			rancherUpgradeVersion = s[1]
		}
		if len(s) > 2 {
			rancherUpgradeHeadVersion = s[2]
		}
	}

	// Extract Rancher Manager channel/version to install
	if rancherVersion != "" {
		// Split rancherVersion and reset it
		s := strings.Split(rancherVersion, "/")
		rancherVersion = ""

		// Get needed informations
		rancherChannel = s[0]
		if len(s) > 1 {
			rancherVersion = s[1]
		}
		if len(s) > 2 {
			rancherHeadVersion = s[2]
		}
	}

	// Force correct value for sequential
	switch seqString {
	case "true":
		sequential = true
	default:
		sequential = false
	}

	// Set number of "used" nodes
	// NOTE: could be the number of added nodes or the number of nodes to use/upgrade
	usedNodes = (numberOfVMs - vmIndex) + 1
})
