package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/cucumber/godog/colors"
	resty "github.com/go-resty/resty/v2"
	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/random"
	. "github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var namespace string
var repairId string
var opts = godog.Options{
	Output: colors.Colored(os.Stdout),
	Format: "progress", // can define default values
}

func init() {
	godog.BindFlags("godog.", flag.CommandLine, &opts) // godog v0.10.0 and earlier
	godog.BindCommandLineFlags("godog.", &opts)        // godog v0.11.0 (latest)
}

func TestMain(m *testing.M) {
	flag.Parse()
	opts.Paths = flag.Args()

	status := godog.TestSuite{
		Name:                "godogs",
		ScenarioInitializer: InitializeScenario,
		Options:             &opts,
	}.Run()

	// Optional: Run `testing` package's logic besides godog.
	if st := m.Run(); st > status {
		status = st
	}

	os.Exit(status)
}

func runShellCommand(command *exec.Cmd) error {
	err := command.Start()
	if err != nil {
		log.Fatal(err)
	}
	err = command.Wait()
	return err
}

func runShellCommandAndGetOutput(command *exec.Cmd) string {
	var outb bytes.Buffer
	command.Stdout = &outb
	err := command.Run()
	if err != nil {
		log.Fatal(err)
	}

	return string(outb.String())
}

func assertActual(a actualAssertion, actual interface{}, msgAndArgs ...interface{}) error {
	var t asserter
	a(&t, actual, msgAndArgs...)
	return t.err
}

type actualAssertion func(t assert.TestingT, actual interface{}, msgAndArgs ...interface{}) bool

// asserter is used to be able to retrieve the error reported by the called assertion
type asserter struct {
	err error
}

// assertExpectedAndActual is a helper function to allow the step function to call
// assertion functions where you want to compare an expected and an actual value.
func assertExpectedAndActual(a expectedAndActualAssertion, expected, actual interface{}, msgAndArgs ...interface{}) error {
	var t asserter
	a(&t, expected, actual, msgAndArgs...)
	return t.err
}

type expectedAndActualAssertion func(t assert.TestingT, expected, actual interface{}, msgAndArgs ...interface{}) bool

// Errorf is used by the called assertion to report an error
func (a *asserter) Errorf(format string, args ...interface{}) {
	a.err = fmt.Errorf(format, args...)
}

func iCanCheckThatResourceOfTypeWithLabelIsPresentInNamespace(resourceType, label string) error {
	if resourceType == "service" {
		services, _ := getServicesWithLabel(label)
		return assertExpectedAndActual(assert.Equal, 1, len(services.Items), "Couldn't find service with label "+label)
	}

	return errors.New("Resource of type " + namespace + " with label " + label + " was not found in namespace " + namespace)
}

func getServicesWithLabel(label string) (*v1.ServiceList, error) {
	kubectlOptions := k8s.NewKubectlOptions("", "", namespace)
	clientset, err := k8s.GetKubernetesClientFromOptionsE(GinkgoT(), kubectlOptions)
	err = assertActual(assert.Nil, err, "Couldn't get k8s client")
	if err != nil {
		return nil, err
	}
	services, err := clientset.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: label})
	return services, err
}

func iCanCheckThatResourceOfTypeWithNameIsPresentInNamespace(resourceType, name string) error {
	kubectlOptions := k8s.NewKubectlOptions("", "", namespace)
	switch resourceType {
	case "service":
		k8s.GetService(GinkgoT(), kubectlOptions, name)
		return nil
	default:
		return fmt.Errorf("Unsupported resource type: %s", resourceType)
	}
}

func aKindClusterIsRunningAndReachable(clusterType string) error {
	iCanDeleteTheKindCluster()
	var kindClusterShell string
	switch clusterType {
	case "one worker":
		kindClusterShell, _ = filepath.Abs("../scripts/cluster_one_worker.sh")
	case "three workers":
		kindClusterShell, _ = filepath.Abs("../scripts/cluster_three_workers.sh")
	default:
		return fmt.Errorf("Kind cluster creation shell script not found for %s", clusterType)
	}
	err := runShellCommand(exec.Command(kindClusterShell))

	return assertActual(assert.Nil, err, "Kind cluster creation failed")
}

func iCanDeleteTheKindCluster() error {
	err := runShellCommand(exec.Command("kind", "delete", "cluster"))
	return assertActual(assert.Nil, err, "Kind cluster deletion failed")
}

func iDeployAClusterWithOptionsInTheNamespaceUsingTheValues(options, customValues string) error {
	helmValues := map[string]string{}
	if options == "default" {
		helmValues = map[string]string{
			"reaper.ingress.host": "repair.localhost",
		}
	}
	return deployCluster(customValues, helmValues)
}

func iDeployAClusterWithCassandraHeapAndMBStargateHeapUsingTheValues(cassandraHeap, stargateHeap, customValues string) error {
	newGenSize, _ := strconv.Atoi(strings.ReplaceAll(strings.ReplaceAll(cassandraHeap, "M", ""), "G", ""))
	helmValues := map[string]string{
		"cassandra.heap.size":       cassandraHeap,
		"cassandra.heap.newGenSize": strconv.Itoa(newGenSize/2) + "M",
		"stargate.heapMB":           strings.ReplaceAll(stargateHeap, "M", ""),
	}
	return deployCluster(customValues, helmValues)
}

func deployCluster(customValues string, helmValues map[string]string) error {
	clusterChartPath, err := filepath.Abs("../../charts/k8ssandra")
	err = assertActual(assert.Nil, err, "Couldn't find the absolute path for K8ssandra charts")
	if err != nil {
		log.Fatal("Couldn't find the absolute path for K8ssandra charts")
		return err
	}

	customChartPath, err := filepath.Abs("../charts/" + customValues)
	if err != nil {
		log.Fatal(fmt.Sprintf("Couldn't find the absolute path for custom charts: %s", customValues))
		return err
	}

	kubectlOptions := k8s.NewKubectlOptions("", "", namespace)
	helmOptions := &helm.Options{}

	helmOptions = &helm.Options{
		// Enable traefik to allow redirections for testing
		SetValues:      helmValues,
		KubectlOptions: k8s.NewKubectlOptions("", "", namespace),
		ValuesFiles:    []string{customChartPath},
	}

	releaseName := "k8ssandra"
	helm.Install(GinkgoT(), helmOptions, clusterChartPath, releaseName)

	// Wait for cass-operator pod to be ready
	attempts := 0
	for {
		attempts++
		clientset, err := k8s.GetKubernetesClientFromOptionsE(GinkgoT(), kubectlOptions)
		pods, _ := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: "app.kubernetes.io/name=cass-operator"})
		err = assertExpectedAndActual(assert.Equal, 1, len(pods.Items), "Couldn't find cass-operator pod")
		if err == nil || attempts > 10 {
			k8s.RunKubectl(GinkgoT(), kubectlOptions, "wait", "--for=condition=Ready", "pod", "-l", "app.kubernetes.io/name=cass-operator", "--timeout=1800s")
			break
		}
		time.Sleep(20 * time.Second)
	}

	// Wait for CassandraDatacenter to be ready..
	k8s.RunKubectl(GinkgoT(), kubectlOptions, "wait", "--for=condition=Ready", "cassandradatacenter/dc1", "--timeout=1800s")

	return nil
}

func iCanSeeTheNamespaceInTheListOfNamespaces() error {
	kubectlOptions := k8s.NewKubectlOptions("", "", "default")
	_, err := k8s.GetNamespaceE(GinkgoT(), kubectlOptions, namespace)
	if err != nil {
		log.Fatal("Couldn't find namespace " + namespace)
	}
	return err
}

func iCanSeeTheSecretInTheListOfSecretsInTheNamespace(secret string) error {
	kubectlOptions := k8s.NewKubectlOptions("", "", namespace)
	_, err := k8s.GetSecretE(GinkgoT(), kubectlOptions, secret)
	if err != nil {
		log.Fatal("Couldn't find secret " + secret)
	}
	return err
}

func iCannotSeeTheNamespaceInTheListOfNamespaces() error {
	kubectlOptions := k8s.NewKubectlOptions("", "", "default")
	attempts := 0
	for {
		attempts++
		namespaceObject, _ := k8s.GetNamespaceE(GinkgoT(), kubectlOptions, namespace)
		err := assertActual(assert.Nil, namespaceObject, fmt.Sprintf("Namespace %s was supposed to be deleted but was found in the k8s cluster.", namespace))
		if err == nil {
			// The namespace cannot be found, all good
			return nil
		}
		if namespaceObject.Status.Phase == v1.NamespaceTerminating {
			return nil
		}
		time.Sleep(10 * time.Second)
		if attempts > 10 {
			break
		}
	}
	return fmt.Errorf("namespace %s was supposed to be deleted but was found in the k8s cluster", namespace)
}

func iCreateTheNamespace() error {
	t := time.Now()
	namespace = fmt.Sprintf("k8ssandra%s", t.Format("2006010215040507"))
	log.Println(fmt.Sprintf("Creating namespace %s", namespace))
	kubectlOptions := k8s.NewKubectlOptions("", "", "default")
	k8s.CreateNamespace(GinkgoT(), kubectlOptions, namespace)
	return nil
}

func iDeleteTheNamespace() error {
	kubectlOptions := k8s.NewKubectlOptions("", "", "default")
	k8s.DeleteNamespace(GinkgoT(), kubectlOptions, namespace)
	return nil
}

func iInstallTraefik() error {
	kubectlOptions := k8s.NewKubectlOptions("", "", "default")
	options := &helm.Options{KubectlOptions: kubectlOptions}

	// Add traefik repo and update repos
	helm.RunHelmCommandAndGetOutputE(GinkgoT(), options, "repo", "add", "traefik", "https://helm.traefik.io/traefik")
	helm.RunHelmCommandAndGetOutputE(GinkgoT(), options, "repo", "update")

	// Deploy traefik
	// helm install traefik traefik/traefik -n traefik --create-namespace -f docs/content/en/docs/topics/ingress/traefik/kind-deployment/traefik.values.yaml
	valuesPath, _ := filepath.Abs("../../docs/content/en/docs/topics/ingress/traefik/kind-deployment/traefik.values.yaml")
	helm.RunHelmCommandAndGetOutputE(GinkgoT(), options, "install", "traefik", "traefik/traefik", "-n", "traefik", "--create-namespace", "-f", valuesPath)
	return nil
}

type credentials struct {
	username string
	password string
}

func getUsernamePassword(secretName, ns string) credentials {
	kubectlOptions := k8s.NewKubectlOptions("", "", ns)
	secret := k8s.GetSecret(GinkgoT(), kubectlOptions, secretName)
	username := secret.Data["username"]
	password := secret.Data["password"]
	creds := credentials{string(username), string(password)}
	return creds
}

func runCassandraQueryAndGetOutput(query string) string {
	cqlCredentials := getUsernamePassword("k8ssandra-superuser", namespace)
	kubectlOptions := k8s.NewKubectlOptions("", "", namespace)
	// Get reaper service
	output, _ := k8s.RunKubectlAndGetOutputE(GinkgoT(), kubectlOptions, "exec", "-it", "k8ssandra-dc1-default-sts-0", "--", "/opt/cassandra/bin/cqlsh", "--username", cqlCredentials.username, "--password", cqlCredentials.password, "-e", query)
	return output
}

func iCanSeeThatTheKeyspaceExistsInCassandraInNamespace(keyspace string) error {
	reaperDbKeyspace := runCassandraQueryAndGetOutput("describe keyspaces")
	assertExpectedAndActual(assert.Contains, "reaper_db", reaperDbKeyspace, "Couldn't find the reaper_db keyspace in Cassandra")
	return nil
}

func iWaitForTheReaperPodToBeReadyInNamespace() error {
	err := waitForPodWithLabelToBeReady("app.kubernetes.io/managed-by=reaper-operator", 30*time.Second, 10)
	return err
}

func iCanReadRowsInTheTableInTheKeyspace(nbRows int, tableName, keyspaceName string) error {
	output := runCassandraQueryAndGetOutput(fmt.Sprintf("SELECT id FROM %s.%s", keyspaceName, tableName))
	err := assertExpectedAndActual(assert.Contains, output, fmt.Sprintf("(%d rows)", nbRows), "Wrong number of rows found in the table.")
	return err
}

func iCreateTheTableInTheKeyspace(tableName, keyspaceName string) error {
	runCassandraQueryAndGetOutput(fmt.Sprintf("CREATE KEYSPACE IF NOT EXISTS %s with replication = {'class':'SimpleStrategy', 'replication_factor':1};", keyspaceName))
	runCassandraQueryAndGetOutput(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s(id timeuuid PRIMARY KEY, val text);", keyspaceName, tableName))
	return nil
}

func iLoadRowsInTheTableInTheKeyspace(nbRows int, tableName, keyspaceName string) error {
	for i := 0; i < nbRows; i++ {
		runCassandraQueryAndGetOutput(fmt.Sprintf("INSERT INTO %s.%s(id,val) values(now(), '%d');", keyspaceName, tableName, i))
	}
	return nil
}

func waitForPodWithLabelToBeReady(label string, waitTime time.Duration, maxAttempts int) error {
	kubectlOptions := k8s.NewKubectlOptions("", "", namespace)
	attempts := 0
	for {
		attempts++
		getCassandraPodOutput, err := k8s.RunKubectlAndGetOutputE(GinkgoT(), kubectlOptions, "get", "pods", "-l", label)
		if err == nil && !strings.HasPrefix(getCassandraPodOutput, "No resources found") {
			break
		}
		if attempts > maxAttempts {
			return fmt.Errorf("Pod with label '%s' didn't start within timeout", label)
		}
		time.Sleep(waitTime)
	}
	k8s.RunKubectl(GinkgoT(), kubectlOptions, "wait", "--for=condition=Ready", "pod", "-l", label, "--timeout=1800s")
	return nil
}

// Medusa related functions
func iCreateTheMedusaSecretInTheNamespaceApplyingTheFile(secretFile string) error {
	home, _ := os.UserHomeDir()
	medusaSecretPath, _ := filepath.Abs(strings.Replace(secretFile, "~", home, 1))
	kubectlOptions := k8s.NewKubectlOptions("", "", namespace)
	k8s.KubectlApply(GinkgoT(), kubectlOptions, medusaSecretPath)
	return nil
}

func iPerformABackupWithMedusaNamed(backupName string) error {
	kubectlOptions := k8s.NewKubectlOptions("", "", namespace)
	backupChartPath, err := filepath.Abs("../../charts/backup")
	if err != nil {
		log.Fatal(fmt.Sprintf("Couldn't find the absolute path for backup charts"))
		return err
	}

	helmOptions := &helm.Options{
		SetValues: map[string]string{
			"name":                     backupName,
			"cassandraDatacenter.name": "dc1",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", namespace),
	}
	helm.Install(GinkgoT(), helmOptions, backupChartPath, "test")

	// Wait for the backup to be finished
	// kubectl get cassandrabackup backup1 -o jsonpath='{.status.finished}' -n k8ssandra2021021306435807
	attempts := 0
	for {
		attempts++
		output, err := k8s.RunKubectlAndGetOutputE(GinkgoT(), kubectlOptions, "get", "cassandrabackup", backupName, "-o", "jsonpath='{.status.finished}'")
		if err == nil && len(output) > 0 {
			var nodes []string
			json.Unmarshal([]byte(strings.ReplaceAll(output, "'", "")), &nodes)
			if len(nodes) == 1 {
				return nil
			}
		}
		if attempts > 12 {
			return fmt.Errorf("Backup didn't succeed within timeout: %s", err)
		}
		time.Sleep(10 * time.Second)
	}
}

func iRestoreTheBackupNamedUsingMedusa(backupName string) error {
	restoreChartPath, err := filepath.Abs("../../charts/restore")
	if err != nil {
		log.Fatal(fmt.Sprintf("Couldn't find the absolute path for restore charts"))
		return err
	}

	helmOptions := &helm.Options{
		SetValues: map[string]string{
			"backup.name":              backupName,
			"cassandraDatacenter.name": "dc1",
			"name":                     "restore-test",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", namespace),
	}
	helm.Install(GinkgoT(), helmOptions, restoreChartPath, "restore-test")
	// Give a little time for the cassandraDatacenter resource to be recreated
	time.Sleep(60 * time.Second)
	err = waitForPodWithLabelToBeReady("app.kubernetes.io/managed-by=cass-operator", 30*time.Second, 10)

	return err
}

// Reaper related functions
func iCanCheckThatAClusterNamedWasRegisteredInReaperInNamespace(clusterName string) error {
	restClient := resty.New()
	attempts := 0
	for {
		attempts++
		response, err := restClient.R().Get("http://repair.localhost:8080/cluster")
		if err != nil {
			log.Println(fmt.Sprintf("The HTTP request failed with error %s", err))
		} else {
			data := response.Body()
			log.Println(fmt.Sprintf("Reaper response: %s", data))
			var clusters []string
			json.Unmarshal([]byte(data), &clusters)
			if len(clusters) > 0 {
				assertErr := assertExpectedAndActual(assert.Equal, clusterName, clusters[0], fmt.Sprintf("%s cluster wasn't properly registered in Reaper", clusterName))
				return assertErr
			}
		}
		time.Sleep(30 * time.Second)
		if attempts >= 10 {
			break
		}
	}
	return errors.New("Cluster wasn't properly registered in Reaper")
}

func iCanCancelTheRunningRepair() error {
	restClient := resty.New()
	// Start the previously created repair run
	response, err := restClient.R().
		SetHeader("Content-Type", "application/json").
		Put(fmt.Sprintf("http://repair.localhost:8080/repair_run/%s/state/ABORTED", repairId))

	log.Println(fmt.Sprintf("Reaper response: %s", response.Body()))
	log.Println(fmt.Sprintf("Reaper status code: %d", response.StatusCode()))

	if err != nil || response.StatusCode() != 200 {
		return fmt.Errorf("Failed aborting repair %s: %s / %s", repairId, err, response.Body())
	}

	return nil
}

func iTriggerARepairOnTheKeyspace(arg1 string) error {
	restClient := resty.New()

	// Create the repair run
	response, err := restClient.R().
		SetHeader("Content-Type", "application/json").
		SetQueryParams(map[string]string{
			"clusterName":  "k8ssandra",
			"keyspace":     "reaper_db",
			"owner":        "k8ssandra",
			"segmentCount": "5",
		}).
		Post("http://repair.localhost:8080/repair_run")

	data := response.Body()
	log.Println(fmt.Sprintf("Reaper response: %s", data))
	var reaperResponse interface{}
	err2 := json.Unmarshal(data, &reaperResponse)

	if err != nil || err2 != nil {
		return fmt.Errorf("The REST request or response parsing failed with error %s %s: %s", err, err2, data)
	}

	reaperResponseMap := reaperResponse.(map[string]interface{})
	repairId = fmt.Sprintf("%s", reaperResponseMap["id"])
	// Start the previously created repair run
	response, err = restClient.R().
		SetHeader("Content-Type", "application/json").
		Put(fmt.Sprintf("http://repair.localhost:8080/repair_run/%s/state/RUNNING", repairId))

	log.Println(fmt.Sprintf("Reaper response: %s", response.Body()))
	log.Println(fmt.Sprintf("Reaper status code: %d", response.StatusCode()))

	if err != nil || response.StatusCode() != 200 {
		return fmt.Errorf("Failed starting repair %s: %s / %s", repairId, err, response.Body())
	}

	return nil
}

func iWaitForAtLeastOneSegmentToBeProcessed() error {
	restClient := resty.New()
	attempts := 0
	for {
		attempts++
		response, err := restClient.R().Get(fmt.Sprintf("http://repair.localhost:8080/repair_run/%s/segments", repairId))
		if err != nil {
			log.Println(fmt.Sprintf("The HTTP request failed with error %s", err))
		}

		if strings.Contains(fmt.Sprintf("%s", response.Body()), "\"state\":\"DONE\"") {
			// We have at least one completed segment
			return nil
		}

		time.Sleep(30 * time.Second)
		if attempts >= 10 {
			// Too many attempts, failed test.
			log.Println(fmt.Sprintf("Latest segment list from Reaper: %s", response.Body()))
			break
		}
	}
	return errors.New("No repair segment was fully processed within timeout")
}

func getStargateService() (v1.Service, error) {
	kubectlOptions := k8s.NewKubectlOptions("", "", namespace)
	clientset, err := k8s.GetKubernetesClientFromOptionsE(GinkgoT(), kubectlOptions)
	err = assertActual(assert.Nil, err, "Couldn't get k8s client")
	if err != nil {
		return v1.Service{}, err
	}
	services, err := clientset.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
	for _, service := range services.Items {
		if strings.HasSuffix(service.ObjectMeta.Name, "-stargate-service") {
			return service, nil
		}
	}
	return v1.Service{}, fmt.Errorf("Couldn't find the Stargate service")
}

func iCanRunACyclesStressTestWithReadsAndAOpssRateWithinTimeout(stressCycles, percentRead string, rate, timeout int) error {
	kubectlOptions := k8s.NewKubectlOptions("", "", namespace)
	cqlCredentials := getUsernamePassword("k8ssandra-superuser", namespace)
	parsedReadRatio, _ := strconv.Atoi(strings.ReplaceAll(percentRead, "%", ""))
	readRatio := parsedReadRatio / 10
	writeRatio := 10 - readRatio

	jobName := fmt.Sprintf("nosqlbench-%s", strings.ToLower(random.UniqueId()))
	k8s.RunKubectl(GinkgoT(), kubectlOptions, "create", "job", "--image=nosqlbench/nosqlbench", jobName,
		"--", "java", "-jar", "nb.jar", "cql-iot", "rampup-cycles=1", fmt.Sprintf("cyclerate=%d", rate),
		fmt.Sprintf("username=%s", cqlCredentials.username), fmt.Sprintf("password=%s", cqlCredentials.password),
		fmt.Sprintf("main-cycles=%s", stressCycles), "hosts=k8ssandra-dc1-stargate-service", "--progress", "console:1s", "-v",
		fmt.Sprintf("write_ratio=%d", writeRatio), fmt.Sprintf("read_ratio=%d", readRatio))

	defer timeTrack(time.Now(), "nosqlbench stress test")
	k8s.RunKubectl(GinkgoT(), kubectlOptions, "wait", "--for=condition=complete", fmt.Sprintf("--timeout=%ds", timeout), fmt.Sprintf("job/%s", jobName))

	output := runShellCommandAndGetOutput(
		exec.Command("bash", "-c", fmt.Sprintf("kubectl logs job/%s -n %s | grep cqliot_default_main.cycles.servicetime", jobName, namespace)))
	log.Println(Outline(output))

	return nil
}

func iWaitForTheStargatePodsToBeReady() error {
	kubectlOptions := k8s.NewKubectlOptions("", "", namespace)

	attempts := 0
	for {
		attempts++
		output, err := k8s.RunKubectlAndGetOutputE(GinkgoT(), kubectlOptions, "rollout", "status", "deployment", "k8ssandra-dc1-stargate")
		if err == nil {
			if strings.HasSuffix(output, "successfully rolled out") {
				return nil
			}
		}
		time.Sleep(30 * time.Second)
		if attempts >= 10 {
			// Too many attempts, failed test.
			break
		}
	}
	return fmt.Errorf("Stargate deployment didn't roll out within timeout")
}

var (
	Info    = Yellow
	Outline = Purple
)

var (
	Black   = Color("\033[1;30m%s\033[0m")
	Red     = Color("\033[1;31m%s\033[0m")
	Green   = Color("\033[1;32m%s\033[0m")
	Yellow  = Color("\033[1;33m%s\033[0m")
	Purple  = Color("\033[1;34m%s\033[0m")
	Magenta = Color("\033[1;35m%s\033[0m")
	Teal    = Color("\033[1;36m%s\033[0m")
	White   = Color("\033[1;37m%s\033[0m")
)

func Color(colorString string) func(...interface{}) string {
	sprint := func(args ...interface{}) string {
		return fmt.Sprintf(colorString,
			fmt.Sprint(args...))
	}
	return sprint
}

func timeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Println(Info(fmt.Sprintf("%s took %s", name, elapsed)))
}

func InitializeScenario(ctx *godog.ScenarioContext) {
	ctx.Step(`^a kind cluster with "([^"]*)" is running and reachable$`, aKindClusterIsRunningAndReachable)
	ctx.Step(`^I deploy a cluster with "([^"]*)" options using the "([^"]*)" values$`, iDeployAClusterWithOptionsInTheNamespaceUsingTheValues)
	ctx.Step(`^I can see the namespace in the list of namespaces$`, iCanSeeTheNamespaceInTheListOfNamespaces)
	ctx.Step(`^I can see the "([^"]*)" secret in the list of secrets of the namespace$`, iCanSeeTheSecretInTheListOfSecretsInTheNamespace)
	ctx.Step(`^I cannot see the namespace in the list of namespaces$`, iCannotSeeTheNamespaceInTheListOfNamespaces)
	ctx.Step(`^I create the Medusa secret applying the "([^"]*)" file$`, iCreateTheMedusaSecretInTheNamespaceApplyingTheFile)
	ctx.Step(`^I create a namespace that will be used throughout the scenario$`, iCreateTheNamespace)
	ctx.Step(`^I delete the namespace$`, iDeleteTheNamespace)
	ctx.Step(`^I install Traefik$`, iInstallTraefik)
	ctx.Step(`^I can delete the kind cluster$`, iCanDeleteTheKindCluster)
	ctx.Step(`^I can check that resource of type "([^"]*)" with label "([^"]*)" is present$`, iCanCheckThatResourceOfTypeWithLabelIsPresentInNamespace)
	ctx.Step(`^I can check that resource of type "([^"]*)" with name "([^"]*)" is present$`, iCanCheckThatResourceOfTypeWithNameIsPresentInNamespace)
	ctx.Step(`^I can check that a cluster named "([^"]*)" was registered in Reaper$`, iCanCheckThatAClusterNamedWasRegisteredInReaperInNamespace)
	ctx.Step(`^I can see that the "([^"]*)" keyspace exists in Cassandra$`, iCanSeeThatTheKeyspaceExistsInCassandraInNamespace)
	ctx.Step(`^I wait for the Reaper pod to be ready$`, iWaitForTheReaperPodToBeReadyInNamespace)
	ctx.Step(`^I can read (\d+) rows in the "([^"]*)" table in the "([^"]*)" keyspace$`, iCanReadRowsInTheTableInTheKeyspace)
	ctx.Step(`^I create the "([^"]*)" table in the "([^"]*)" keyspace$`, iCreateTheTableInTheKeyspace)
	ctx.Step(`^I load (\d+) rows in the "([^"]*)" table in the "([^"]*)" keyspace$`, iLoadRowsInTheTableInTheKeyspace)
	ctx.Step(`^I perform a backup with Medusa named "([^"]*)"$`, iPerformABackupWithMedusaNamed)
	ctx.Step(`^I restore the backup named "([^"]*)" using Medusa$`, iRestoreTheBackupNamedUsingMedusa)
	ctx.Step(`^I can cancel the running repair$`, iCanCancelTheRunningRepair)
	ctx.Step(`^I trigger a repair on the "([^"]*)" keyspace$`, iTriggerARepairOnTheKeyspace)
	ctx.Step(`^I wait for at least one segment to be processed$`, iWaitForAtLeastOneSegmentToBeProcessed)
	ctx.Step(`^I can run a "([^"]*)" cycles stress test with "([^"]*)" reads and a (\d+) ops\/s rate within (\d+) seconds$`, iCanRunACyclesStressTestWithReadsAndAOpssRateWithinTimeout)
	ctx.Step(`^I deploy a cluster with "([^"]*)" Cassandra heap and "([^"]*)" Stargate heap using the "([^"]*)" values$`, iDeployAClusterWithCassandraHeapAndMBStargateHeapUsingTheValues)
	ctx.Step(`^I wait for the Stargate pods to be ready$`, iWaitForTheStargatePodsToBeReady)
}
