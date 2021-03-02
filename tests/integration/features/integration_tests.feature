Feature: Test K8ssandra deployments
  In order to verify that the stack deploys correctly
  As a K8ssandra operator
  I need to be able to run the following scenarios

  @Reaper
  Scenario: Install K8ssandra and run Reaper
    Given a kind cluster with "three workers" is running and reachable
    When I install Traefik
    And I create a namespace that will be used throughout the scenario
    Then I can see the namespace in the list of namespaces
    When I deploy a cluster with "default" options using the "three_nodes_cluster_with_reaper.yaml" values
    Then I can check that resource of type "service" with label "app.kubernetes.io/managed-by=reaper-operator" is present
    And I can check that resource of type "service" with name "k8ssandra-dc1-all-pods-service" is present
    And I can check that resource of type "service" with name "k8ssandra-dc1-service" is present
    And I can check that resource of type "service" with name "k8ssandra-seed-service" is present
    When I wait for the Reaper pod to be ready
    And I can see that the "reaper_db" keyspace exists in Cassandra
    And I can check that a cluster named "k8ssandra" was registered in Reaper
    When I trigger a repair on the "reaper_db" keyspace
    Then I wait for at least one segment to be processed
    And I can cancel the running repair
    When I delete the namespace
    Then I cannot see the namespace in the list of namespaces
    And I can delete the kind cluster
  
  @MedusaS3
  Scenario: Install K8ssandra with Medusa on S3
    Given a kind cluster with "one worker" is running and reachable
    When I create a namespace that will be used throughout the scenario
    And I create the Medusa secret applying the "~/medusa_secret.yaml" file
    Then I can see the namespace in the list of namespaces
    And I can see the "medusa-bucket-key" secret in the list of secrets of the namespace
    When I deploy a cluster with "no Traefik" options using the "one_node_cluster_with_medusa.yaml" values
    And I can check that resource of type "service" with name "k8ssandra-dc1-all-pods-service" is present
    And I can check that resource of type "service" with name "k8ssandra-dc1-service" is present
    And I can check that resource of type "service" with name "k8ssandra-seed-service" is present
    When I create the "medusa_test" table in the "medusa" keyspace
    And I load 10 rows in the "medusa_test" table in the "medusa" keyspace
    Then I can read 10 rows in the "medusa_test" table in the "medusa" keyspace
    And I perform a backup with Medusa named "backup1"
    When I load 10 rows in the "medusa_test" table in the "medusa" keyspace
    Then I can read 20 rows in the "medusa_test" table in the "medusa" keyspace
    When I restore the backup named "backup1" using Medusa
    Then I can read 10 rows in the "medusa_test" table in the "medusa" keyspace
    When I delete the namespace
    Then I cannot see the namespace in the list of namespaces
    And I can delete the kind cluster
  
  @MedusaMinio
  Scenario: Install K8ssandra with Medusa on MinIO
    Given a kind cluster with "one worker" is running and reachable
    When I deploy MinIO using helm and create the "k8ssandra-medusa" bucket
    And I create a namespace that will be used throughout the scenario
    And I create the Medusa secret applying the "../secret/medusa_minio_secret.yaml" file
    Then I can see the namespace in the list of namespaces
    And I can see the "medusa-bucket-key" secret in the list of secrets of the namespace
    When I deploy a cluster with "minio" options using the "one_node_cluster_with_medusa_minio.yaml" values
    And I can check that resource of type "service" with name "k8ssandra-dc1-all-pods-service" is present
    And I can check that resource of type "service" with name "k8ssandra-dc1-service" is present
    And I can check that resource of type "service" with name "k8ssandra-seed-service" is present
    When I create the "medusa_test" table in the "medusa" keyspace
    And I load 10 rows in the "medusa_test" table in the "medusa" keyspace
    Then I can read 10 rows in the "medusa_test" table in the "medusa" keyspace
    And I perform a backup with Medusa named "backup1"
    When I load 10 rows in the "medusa_test" table in the "medusa" keyspace
    Then I can read 20 rows in the "medusa_test" table in the "medusa" keyspace
    When I restore the backup named "backup1" using Medusa
    Then I can read 10 rows in the "medusa_test" table in the "medusa" keyspace
    When I delete the namespace
    Then I cannot see the namespace in the list of namespaces
    And I can delete the kind cluster

  @StressLite
  Scenario: Perform a stress test with k8ssandra using 500M/300M heaps
    Given a kind cluster with "three workers" is running and reachable
    When I install Traefik
    And I create a namespace that will be used throughout the scenario
    Then I can see the namespace in the list of namespaces
    When I deploy a cluster with "500M" Cassandra heap and "300M" Stargate heap using the "three_nodes_cluster_with_stargate.yaml" values
    And I wait for the Stargate pods to be ready
    Then I can run a "10k" cycles stress test with "30%" reads and a 100 ops/s rate within 900 seconds
    When I delete the namespace
    Then I cannot see the namespace in the list of namespaces
    And I can delete the kind cluster

  @Stress @Stress1
  Scenario: Perform a stress test with k8ssandra using 500M/300M heaps
    Given a kind cluster with "three workers" is running and reachable
    When I install Traefik
    And I create a namespace that will be used throughout the scenario
    Then I can see the namespace in the list of namespaces
    And I create the Medusa secret applying the "~/medusa_secret.yaml" file
    When I deploy a cluster with "reaper-medusa-monitoring" options and "500M" Cassandra heap and "300M" Stargate heap using the "three_nodes_cluster_with_stargate.yaml" values
    And I wait for the Stargate pods to be ready
    Then I can run a "10k" cycles stress test with "30%" reads and a 100 ops/s rate within 900 seconds
    And I can run a "50k" cycles stress test with "30%" reads and a 500 ops/s rate within 900 seconds
    And I can run a "100k" cycles stress test with "30%" reads and a 1000 ops/s rate within 900 seconds
    And I can run a "150k" cycles stress test with "30%" reads and a 1500 ops/s rate within 900 seconds
    When I delete the namespace
    Then I cannot see the namespace in the list of namespaces
    And I can delete the kind cluster
  
  @Stress @Stress2
  Scenario: Perform a stress test with k8ssandra using 1024M/300M heaps
    Given a kind cluster with "three workers" is running and reachable
    When I install Traefik
    And I create a namespace that will be used throughout the scenario
    Then I can see the namespace in the list of namespaces
    And I create the Medusa secret applying the "~/medusa_secret.yaml" file
    When I deploy a cluster with "reaper-medusa-monitoring" options and "1024M" Cassandra heap and "300M" Stargate heap using the "three_nodes_cluster_with_stargate.yaml" values
    And I wait for the Stargate pods to be ready
    Then I can run a "10k" cycles stress test with "30%" reads and a 100 ops/s rate within 900 seconds
    And I can run a "50k" cycles stress test with "30%" reads and a 500 ops/s rate within 900 seconds
    And I can run a "100k" cycles stress test with "30%" reads and a 1000 ops/s rate within 900 seconds
    And I can run a "150k" cycles stress test with "30%" reads and a 1500 ops/s rate within 900 seconds
    When I delete the namespace
    Then I cannot see the namespace in the list of namespaces
    And I can delete the kind cluster
  
  @Stress @Stress3
  Scenario: Perform a stress test with k8ssandra using 500M/500M heaps
    Given a kind cluster with "three workers" is running and reachable
    When I install Traefik
    And I create a namespace that will be used throughout the scenario
    Then I can see the namespace in the list of namespaces
    And I create the Medusa secret applying the "~/medusa_secret.yaml" file
    When I deploy a cluster with "reaper-medusa-monitoring" options and "500M" Cassandra heap and "500M" Stargate heap using the "three_nodes_cluster_with_stargate.yaml" values
    And I wait for the Stargate pods to be ready
    Then I can run a "10k" cycles stress test with "30%" reads and a 100 ops/s rate within 900 seconds
    And I can run a "50k" cycles stress test with "30%" reads and a 500 ops/s rate within 900 seconds
    And I can run a "100k" cycles stress test with "30%" reads and a 1000 ops/s rate within 900 seconds
    And I can run a "150k" cycles stress test with "30%" reads and a 1500 ops/s rate within 900 seconds
    When I delete the namespace
    Then I cannot see the namespace in the list of namespaces
    And I can delete the kind cluster
  
  @Stress @Stress4
  Scenario: Perform a stress test with k8ssandra using 1024M/500M heaps
    Given a kind cluster with "three workers" is running and reachable
    When I install Traefik
    And I create a namespace that will be used throughout the scenario
    Then I can see the namespace in the list of namespaces
    And I create the Medusa secret applying the "~/medusa_secret.yaml" file
    When I deploy a cluster with "reaper-medusa-monitoring" options and "1024M" Cassandra heap and "500M" Stargate heap using the "three_nodes_cluster_with_stargate.yaml" values
    And I wait for the Stargate pods to be ready
    Then I can run a "10k" cycles stress test with "30%" reads and a 100 ops/s rate within 900 seconds
    And I can run a "50k" cycles stress test with "30%" reads and a 500 ops/s rate within 900 seconds
    And I can run a "100k" cycles stress test with "30%" reads and a 1000 ops/s rate within 900 seconds
    And I can run a "150k" cycles stress test with "30%" reads and a 1500 ops/s rate within 900 seconds
    When I delete the namespace
    Then I cannot see the namespace in the list of namespaces
    And I can delete the kind cluster
  
  @StartupTest @StartupFull
  Scenario: Full stack startup test with k8ssandra using 500M/300M heaps
    Given a kind cluster with "three workers" is running and reachable
    When I install Traefik
    And I create a namespace that will be used throughout the scenario
    Then I can see the namespace in the list of namespaces
    And I create the Medusa secret applying the "~/medusa_secret.yaml" file
    When I deploy a cluster with "reaper-medusa-monitoring" options and "500M" Cassandra heap and "300M" Stargate heap using the "three_nodes_cluster_with_stargate.yaml" values
    Then I wait for the Reaper pod to be ready
    And I wait for the Stargate pods to be ready
    When I delete the namespace
    Then I cannot see the namespace in the list of namespaces
    And I can delete the kind cluster
  
  @StartupTest @StartupNoMedusa
  Scenario: No Medusa stack startup test with k8ssandra using 500M/300M heaps
    Given a kind cluster with "three workers" is running and reachable
    When I install Traefik
    And I create a namespace that will be used throughout the scenario
    Then I can see the namespace in the list of namespaces
    And I create the Medusa secret applying the "~/medusa_secret.yaml" file
    When I deploy a cluster with "reaper-nomedusa-monitoring" options and "500M" Cassandra heap and "300M" Stargate heap using the "three_nodes_cluster_with_stargate.yaml" values
    Then I wait for the Reaper pod to be ready
    And I wait for the Stargate pods to be ready
    When I delete the namespace
    Then I cannot see the namespace in the list of namespaces
    And I can delete the kind cluster
  
  @StartupTest @StartupNoMedusaReaper
  Scenario: No Reaper nor Medusa stack startup test with k8ssandra using 500M/300M heaps
    Given a kind cluster with "three workers" is running and reachable
    When I install Traefik
    And I create a namespace that will be used throughout the scenario
    Then I can see the namespace in the list of namespaces
    And I create the Medusa secret applying the "~/medusa_secret.yaml" file
    When I deploy a cluster with "noreaper-nomedusa-monitoring" options and "500M" Cassandra heap and "300M" Stargate heap using the "three_nodes_cluster_with_stargate.yaml" values
    Then I wait for the Stargate pods to be ready
    When I delete the namespace
    Then I cannot see the namespace in the list of namespaces
    And I can delete the kind cluster
  
  @StartupTest @StartupNoMedusaReaperMonitoring
  Scenario: Minimal stack startup test with k8ssandra using 500M/300M heaps
    Given a kind cluster with "three workers" is running and reachable
    When I install Traefik
    And I create a namespace that will be used throughout the scenario
    Then I can see the namespace in the list of namespaces
    And I create the Medusa secret applying the "~/medusa_secret.yaml" file
    When I deploy a cluster with "noreaper-nomedusa-nomonitoring" options and "500M" Cassandra heap and "300M" Stargate heap using the "three_nodes_cluster_with_stargate.yaml" values
    Then I wait for the Stargate pods to be ready
    When I delete the namespace
    Then I cannot see the namespace in the list of namespaces
    And I can delete the kind cluster