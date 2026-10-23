#!/usr/bin/env bats

@test "removes cpu limits from a pod" {
  run kwctl run annotated-policy.wasm -r test_data/pod_with_cpu_limits.json --settings-json '{}'

  [ "$status" -eq 0 ]
  echo "$output"
  [ $(expr "$output" : '.*"allowed":true.*') -ne 0 ]
  [ $(expr "$output" : '.*"patchType":"JSONPatch".*') -ne 0 ]

  patch=$(echo "$output" | grep -o '"patch":"[^"]*"' | head -1 | cut -d'"' -f4 | base64 -d)
  echo "$patch"
  [ $(expr "$patch" : '.*"/spec/containers/0/resources/limits/cpu".*') -ne 0 ]
  [ $(expr "$patch" : '.*"/spec/containers/1/resources/limits".*') -ne 0 ]
  [ $(expr "$patch" : '.*"/spec/initContainers/0/resources".*') -ne 0 ]
  [ $(expr "$patch" : '.*"/spec/ephemeralContainers/0/resources/limits/cpu".*') -ne 0 ]
  # memory limits and cpu requests are never touched
  [ $(expr "$patch" : '.*memory.*') -eq 0 ]
  [ $(expr "$patch" : '.*requests.*') -eq 0 ]
}

@test "leaves a pod without cpu limits untouched" {
  run kwctl run annotated-policy.wasm -r test_data/pod_without_cpu_limits.json --settings-json '{}'

  [ "$status" -eq 0 ]
  echo "$output"
  [ $(expr "$output" : '.*"allowed":true.*') -ne 0 ]
  [ $(expr "$output" : '.*"patchType".*') -eq 0 ]
}

@test "accepts omitted settings" {
  run kwctl run annotated-policy.wasm -r test_data/pod_with_cpu_limits.json

  [ "$status" -eq 0 ]
  [ $(expr "$output" : '.*"allowed":true.*') -ne 0 ]
}
