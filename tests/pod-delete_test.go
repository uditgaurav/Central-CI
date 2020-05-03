package exp

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/litmuschaos/chaos-operator/pkg/apis/litmuschaos/v1alpha1"
	chaosClient "github.com/litmuschaos/chaos-operator/pkg/client/clientset/versioned/typed/litmuschaos/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/uditgaurav/central-ci/pkg/utils"
	chaosTypes "github.com/uditgaurav/central-ci/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	scheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

var (
	kubeconfig     string
	config         *restclient.Config
	client         *kubernetes.Clientset
	clientSet      *chaosClient.LitmuschaosV1alpha1Client
	err            error
	experimentName = "pod-delete"
	engineName     = "engine1"
	force          = os.Getenv("FORCE")
)

func TestChaos(t *testing.T) {

	RegisterFailHandler(Fail)
	RunSpecs(t, "BDD test")
}

var _ = BeforeSuite(func() {
	var err error
	kubeconfig = os.Getenv("HOME") + "/.kube/config"
	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)

	if err != nil {
		Expect(err).To(BeNil(), "failed to get config")
	}

	client, err = kubernetes.NewForConfig(config)

	if err != nil {
		Expect(err).To(BeNil(), "failed to get client")
	}

	clientSet, err = chaosClient.NewForConfig(config)

	if err != nil {
		Expect(err).To(BeNil(), "failed to get clientSet")
	}

	err = v1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		fmt.Println(err)
	}

})

//BDD Tests for pod-delete experiment
var _ = Describe("BDD of pod-delete experiment", func() {

	// BDD TEST CASE 1
	Context("Check for Litmus components", func() {

		It("Should check for creation of runner pod", func() {

			//Installing RBAC for the experiment
			err := utils.InstallRbac(chaosTypes.PodDeleteRbacPath, chaosTypes.ChaosNamespace, experimentName, client)
			Expect(err).To(BeNil(), "Fail to create RBAC")
			klog.Info("Rbac has been created successfully !!!")

			//Creating Chaos-Experiment
			By("Creating Experiment")
			err = utils.DownloadFile(experimentName+".yaml", chaosTypes.PodDeleteExperimentPath)
			Expect(err).To(BeNil(), "fail to fetch chaos experiment file")
			err = exec.Command("kubectl", "apply", "-f", experimentName+".yaml", "-n", chaosTypes.ChaosNamespace).Run()
			Expect(err).To(BeNil(), "fail to create chaos experiment")
			klog.Info("Chaos Experiment Created Successfully")

			//Installing chaos engine for the experiment
			//Fetching engine file
			By("Fetching engine file for the experiment")
			err = utils.DownloadFile(experimentName+"-ce.yaml", chaosTypes.PodDeleteEnginePath)
			Expect(err).To(BeNil(), "Fail to fetch engine file")

			//Modify chaos engine spec
			err = utils.EditFile(experimentName+"-ce.yaml", "namespace: default", "namespace: "+chaosTypes.ChaosNamespace)
			Expect(err).To(BeNil(), "Failed to update namespace in engine")
			err = utils.EditFile(experimentName+"-ce.yaml", "name: nginx-chaos", "name: "+engineName)
			Expect(err).To(BeNil(), "Failed to update engine name in engine")
			err = utils.EditFile(experimentName+"-ce.yaml", "appns: 'default'", "appns: "+chaosTypes.ChaosNamespace)
			Expect(err).To(BeNil(), "Failed to update application namespace in engine")
			err = utils.EditFile(experimentName+"-ce.yaml", "annotationCheck: 'true'", "annotationCheck: 'false'")
			Expect(err).To(BeNil(), "Failed to update AnnotationCheck in engine")
			err = utils.EditFile(experimentName+"-ce.yaml", "applabel: 'app=nginx'", "applabel: "+chaosTypes.ApplicationLabel)
			Expect(err).To(BeNil(), "Failed to update application label in engine")
			err = utils.EditKeyValue(experimentName+"-ce.yaml", "TOTAL_CHAOS_DURATION", "value: '30'", "value: '"+chaosTypes.TotalChaosDuration+"'")
			Expect(err).To(BeNil(), "Failed to update total chaos duration")
			err = utils.EditKeyValue(experimentName+"-ce.yaml", "CHAOS_INTERVAL", "value: '10'", "value: '"+chaosTypes.ChaosInterval+"'")
			Expect(err).To(BeNil(), "Failed to update chaos interval")
			err = utils.EditKeyValue(experimentName+"-ce.yaml", "FORCE", "value: 'false'", "value: '"+force+"'")
			Expect(err).To(BeNil(), "Failed to update chaos interval")

			//Creating ChaosEngine
			By("Creating ChaosEngine")
			err = exec.Command("kubectl", "apply", "-f", experimentName+"-ce.yaml", "-n", chaosTypes.ChaosNamespace).Run()
			Expect(err).To(BeNil(), "Fail to create ChaosEngine")
			klog.Info("ChaosEngine created successfully")
			time.Sleep(2 * time.Second)

			//Fetching the runner pod and Checking if it get in Running state or not
			By("Wait for runner pod to come in running sate")
			runnerNamespace := chaosTypes.ChaosNamespace
			runnerPodStatus, err := utils.RunnerPodStatus(runnerNamespace, engineName, client)
			Expect(runnerPodStatus).NotTo(Equal("1"), "Runner pod failed to get in running state")
			Expect(err).To(BeNil(), "Fail to get the runner pod")
			klog.Info("Runner pod for is in Running state")

			//Waiting for experiment job to get completed
			//Also Printing the logs of the experiment
			By("Waiting for job completion")
			jobNamespace := chaosTypes.ChaosNamespace
			jobPodLogs, err := utils.JobLogs(experimentName, jobNamespace, engineName, client)
			Expect(jobPodLogs).To(Equal(0), "Fail to print the logs of the experiment")
			Expect(err).To(BeNil(), "Fail to get the experiment job pod")

			//Checking the chaosresult
			By("Checking the chaosresult")
			app, err := clientSet.ChaosResults(chaosTypes.ChaosNamespace).Get(engineName+"-"+experimentName, metav1.GetOptions{})
			Expect(string(app.Status.ExperimentStatus.Verdict)).To(Equal("Pass"), "Verdict is not pass chaosresult")
			Expect(err).To(BeNil(), "Fail to get chaosresult")
		})
	})
})
