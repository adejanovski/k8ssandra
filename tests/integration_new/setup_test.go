package integrationnew

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	updateHelmDeps()
	os.Exit(m.Run())
}

func updateHelmDeps() {
	updateHelmDepsScriptPath, _ := filepath.Abs("../../scripts/update-helm-deps.sh")
	log.Println("Updating Helm dependencies...")
	err := runShellCommand(exec.Command("bash", "-c", updateHelmDepsScriptPath))
	if err != nil {
		log.Fatalf(fmt.Sprintf("Failed updating Helm dependencies: %s", err.Error()))
		os.Exit(1)
	}
}

// Reaper scenario:
// - Install Traefik
// - Create a namespace
// - Register a cluster with 3 nodes
// - Verify that Reaper correctly initializes
// - Start a repair on the reaper_db keyspace
// - Wait for at least one segment to be processed
// - Cancel the repair
// - Terminate the namespace and delete the cluster
func TestReaperDeploymentScenario(t *testing.T) {
	aKindClusterIsRunningAndReachableStep(t, "three workers")
	iInstallTraefikStep(t)
	iCreateTheNamespaceStep(t)
	iCanSeeTheNamespaceInTheListOfNamespacesStep(t)
	iDeployAClusterWithOptionsInTheNamespaceUsingTheValuesStep(t, "default", "three_nodes_cluster_with_reaper.yaml")
	iCanCheckThatResourceOfTypeWithLabelIsPresentInNamespaceStep(t, "service", "app.kubernetes.io/managed-by=reaper-operator")
	iCanCheckThatResourceOfTypeWithNameIsPresentInNamespaceStep(t, "service", "k8ssandra-dc1-all-pods-service")
	iCanCheckThatResourceOfTypeWithNameIsPresentInNamespaceStep(t, "service", "k8ssandra-dc1-service")
	iCanCheckThatResourceOfTypeWithNameIsPresentInNamespaceStep(t, "service", "k8ssandra-seed-service")
	iWaitForTheReaperPodToBeReadyInNamespaceStep(t)
	iCanSeeThatTheKeyspaceExistsInCassandraInNamespaceStep(t, "reaper_db")
	iCanCheckThatAClusterNamedWasRegisteredInReaperInNamespaceStep(t, "k8ssandra")
	iTriggerARepairOnTheKeyspaceStep(t, "reaper_db")
	iWaitForAtLeastOneSegmentToBeProcessedStep(t)
	iCanCancelTheRunningRepairStep(t)
	iDeleteTheNamespaceStep(t)
	iCannotSeeTheNamespaceInTheListOfNamespacesStep(t)
	iCanDeleteTheKindClusterStep(t)
}

// Medusa scenario (invoked with a specific backend name):
// - Register a cluster with 1 node
// - Potentially install backend specific dependencies (such as Minio)
// - Create the backend credentials secret
// - Create a keyspace and a table
// - Load 10 rows and check that we can read 10 rows
// - Perform a backup with Medusa
// - Load 10 rows and check that we can read 20 rows
// - Restore the backup
// - Verify that we can read 10 rows
// - Cancel the repair
// - Terminate the namespace and delete the cluster
func MedusaDeploymentScenario(storageBackend string, t *testing.T) {
	medusaTestTable := "medusa_test"
	medusaTestKeyspace := "medusa"

	aKindClusterIsRunningAndReachableStep(t, "one worker")
	iCreateTheNamespaceStep(t)
	iCanSeeTheNamespaceInTheListOfNamespacesStep(t)

	if storageBackend == "minio" {
		iDeployMinIOUsingHelmAndCreateTheBucketStep(t, "k8ssandra-medusa")
		iCreateTheMedusaSecretInTheNamespaceApplyingTheFileStep(t, "../secret/medusa_minio_secret.yaml")
	} else {
		// S3
		iCreateTheMedusaSecretInTheNamespaceApplyingTheFileStep(t, "~/medusa_secret.yaml")
	}
	iCanSeeTheSecretInTheListOfSecretsInTheNamespaceStep(t, "medusa-bucket-key")

	if storageBackend == "minio" {
		iDeployAClusterWithOptionsInTheNamespaceUsingTheValuesStep(t, "minio", "one_node_cluster_with_medusa_minio.yaml")
	} else {
		// S3
		iDeployAClusterWithOptionsInTheNamespaceUsingTheValuesStep(t, "no Traefik", "one_node_cluster_with_medusa_s3.yaml")
	}

	iCanCheckThatResourceOfTypeWithNameIsPresentInNamespaceStep(t, "service", "k8ssandra-dc1-all-pods-service")
	iCanCheckThatResourceOfTypeWithNameIsPresentInNamespaceStep(t, "service", "k8ssandra-dc1-service")
	iCanCheckThatResourceOfTypeWithNameIsPresentInNamespaceStep(t, "service", "k8ssandra-seed-service")

	iCreateTheTableInTheKeyspaceStep(t, medusaTestTable, medusaTestKeyspace)
	// Load 10 rows and check that we can read that exact number of rows
	iLoadRowsInTheTableInTheKeyspaceStep(t, 10, medusaTestTable, medusaTestKeyspace)
	iCanReadRowsInTheTableInTheKeyspaceStep(t, 10, medusaTestTable, medusaTestKeyspace)
	iPerformABackupWithMedusaNamedStep(t, "backup1")

	// Load 10 additional rows and check that we can read 20 rows now
	iLoadRowsInTheTableInTheKeyspaceStep(t, 10, medusaTestTable, medusaTestKeyspace)
	iCanReadRowsInTheTableInTheKeyspaceStep(t, 20, medusaTestTable, medusaTestKeyspace)

	// Restore the backup with 10 rows
	iRestoreTheBackupNamedUsingMedusaStep(t, "backup1")

	// Check that we're back to 10 rows
	iCanReadRowsInTheTableInTheKeyspaceStep(t, 10, medusaTestTable, medusaTestKeyspace)

	iDeleteTheNamespaceStep(t)
	iCannotSeeTheNamespaceInTheListOfNamespacesStep(t)
	iCanDeleteTheKindClusterStep(t)
}

func TestMedusaS3Scenario(t *testing.T) {
	MedusaDeploymentScenario("s3", t)
}

func TestMedusaMinioScenario(t *testing.T) {
	MedusaDeploymentScenario("minio", t)
}

// Stress test
func TestStressLoadOne(t *testing.T) {
	aKindClusterIsRunningAndReachableStep(t, "three workers")
	iInstallTraefikStep(t)
	iCreateTheNamespaceStep(t)
	iCanSeeTheNamespaceInTheListOfNamespacesStep(t)
	iCreateTheMedusaSecretInTheNamespaceApplyingTheFileStep(t, "~/medusa_secret.yaml")
	iDeployAClusterWithCassandraHeapAndMBStargateHeapUsingTheValuesStep(t, "reaper-medusa-monitoring", "500M", "500M", "three_nodes_cluster_with_stargate.yaml")
	iWaitForTheStargatePodsToBeReadyStep(t)
	iCanRunACyclesStressTestWithReadsAndAOpssRateWithinTimeoutStep(t, "10k", "30%", 100, 900)
	iCanRunACyclesStressTestWithReadsAndAOpssRateWithinTimeoutStep(t, "50k", "30%", 500, 900)
	iCanRunACyclesStressTestWithReadsAndAOpssRateWithinTimeoutStep(t, "100k", "30%", 1000, 900)
	iCanRunACyclesStressTestWithReadsAndAOpssRateWithinTimeoutStep(t, "150k", "30%", 1500, 900)
	iDeleteTheNamespaceStep(t)
	iCannotSeeTheNamespaceInTheListOfNamespacesStep(t)
	iCanDeleteTheKindClusterStep(t)
}
