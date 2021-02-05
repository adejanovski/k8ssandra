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
    When I deploy a cluster with "default" options in the namespace using the "three_nodes_cluster_with_reaper.yaml" values
    Then I can check that resource of type "service" with label "app.kubernetes.io/managed-by=reaper-operator" is present in the namespace
    And I can check that resource of type "service" with name "k8ssandra-dc1-all-pods-service" is present in the namespace
    And I can check that resource of type "service" with name "k8ssandra-dc1-service" is present in the namespace
    And I can check that resource of type "service" with name "k8ssandra-seed-service" is present in the namespace
    When I wait for the Reaper pod to be ready in the namespace
    And I can see that the "reaper_db" keyspace exists in Cassandra in the namespace
    And I can check that a cluster named "k8ssandra" was registered in Reaper in the namespace
    When I trigger a repair on the "reaper_db" keyspace
    Then I wait for at least one segment to be processed
    And I can cancel the running repair
    When I delete the namespace
    Then I cannot see the namespace in the list of namespaces
    And I can delete the kind cluster
  
  @Medusa
  Scenario: Install K8ssandra with Medusa
    Given a kind cluster with "one worker" is running and reachable
    When I create a namespace that will be used throughout the scenario
    And I create the Medusa secret in the namespace applying the "~/medusa_secret.yaml" file
    Then I can see the namespace in the list of namespaces
    And I can see the "medusa-bucket-key" secret in the list of secrets of the namespace
    When I deploy a cluster with "no Traefik" options in the namespace using the "one_node_cluster_with_medusa.yaml" values
    #Then I can check that resource of type "service" with label "app.kubernetes.io/name=grafana" is present in the namespace
    #And I can check that resource of type "service" with name "prometheus-operated" is present in the namespace
    And I can check that resource of type "service" with name "k8ssandra-dc1-all-pods-service" is present in the namespace
    And I can check that resource of type "service" with name "k8ssandra-dc1-service" is present in the namespace
    And I can check that resource of type "service" with name "k8ssandra-seed-service" is present in the namespace
    When I create the "medusa_test" table in the "medusa" keyspace in the namespace
    And I load 10 rows in the "medusa_test" table in the "medusa" keyspace in the namespace
    Then I can read 10 rows in the "medusa_test" table in the "medusa" keyspace in the namespace
    And I perform a backup with Medusa named "backup1" in the namespace
    When I load 10 rows in the "medusa_test" table in the "medusa" keyspace in the namespace
    Then I can read 20 rows in the "medusa_test" table in the "medusa" keyspace in the namespace
    When I restore the backup named "backup1" using Medusa in the namespace
    Then I can read 10 rows in the "medusa_test" table in the "medusa" keyspace in the namespace
    When I delete the namespace
    Then I cannot see the namespace in the list of namespaces
    And I can delete the kind cluster