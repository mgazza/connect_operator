# Kafka-connect operator
Loosely based on the [kafka-operator](https://github.com/confluentinc/kafka-devops) but uses dedicated resources rather
than ConfigMaps

# Manifests
This directory contains kubernetes resources used by this deployment

# args
| arg        | default      | comments                                                                                                      |
|------------|--------------|---------------------------------------------------------------------------------------------------------------|
| kubeConfig |              | Path to a kubeConfig. Only required if out-of-cluster.                                                        |
| master     |              | The address of the Kubernetes API server. Overrides any value in kubeConfig. Only required if out-of-cluster. |
| baseURL    | $CONNECT_URL | The Base URL of the kafka connect api.                                                                        |

# env
| env         | default              | comments                               |
|-------------|----------------------|----------------------------------------|
| CONNECT_URL | http://kafka-connect | The Base URL of the kafka connect api. |

# Build
This project is continuously integrated by github and produces a docker image
```bash 
docker pull ghcr.io/mgazza/connect_operator:latest
```

# Generated resources
This project uses a few generated resources.
To regenerate the generated code issue the following command.
```bash
./hack/update-codegen.sh
```
This project uses go mod
you may need to execute `go mod vendor` before the above will work.