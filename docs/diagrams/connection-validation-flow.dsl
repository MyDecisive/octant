workspace "Octant Connection Validation" "Slim C4 view of how Octant validates a Datadog connection." {
  model {
    user = person "User" "Runs and reviews the Datadog connection test result in Octant."

    octant = softwareSystem "Octant validation screen" "Shows Datadog connection validation results."
    datadogClients = softwareSystem "Datadog clients / agents" "Datadog clients, agents, or workloads sending logs and traces."

    smarthub = softwareSystem "SmartHub" "Receives, routes, exports, and validates Datadog telemetry." {
      envoy = container "SmartHub Envoy / ingress" "Accepts Datadog client connections and forwards telemetry into SmartHub." "Envoy"
      telemetryPipeline = container "SmartHub telemetry pipeline" "Routes telemetry through SmartHub and exports it to Datadog." "OpenTelemetry pipeline"
      validator = container "Data Fidelity validator" "Compares required Datadog fields and values, then returns policy pass/fail results." "Validator" {
        policyCheck = component "Policy check" "Checks required Datadog fields and values." "Validation logic"
        result = component "Validation result" "Returns policy pass/fail details to Octant." "Result API"
      }
    }

    datadog = softwareSystem "Datadog ingest API" "Telemetry destination."

    user -> octant "Runs and reviews connection test result"
    datadogClients -> envoy "Send logs and traces"
    envoy -> telemetryPipeline "Forwards telemetry"
    telemetryPipeline -> datadog "Exports telemetry"
    envoy -> validator "Mirrors values for required Datadog field comparison"
    validator -> octant "Returns policy pass/fail result"
  }

  views {
    systemContext octant "Octant-Connection-Validation-System-Context" {
      include *
      autoLayout lr
    }

    container smarthub "Octant-Connection-Validation-Container" {
      include *
      autoLayout lr
    }

    component validator "Octant-Connection-Validation-Validator-Component" {
      include *
      autoLayout lr
    }
  }
}
