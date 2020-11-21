# Flecs

Flexible [ECS](https://aws.amazon.com/ecs/) deployment!

## Why Flecs?

There are many deployment tools for Amazon ECS, but these almost always only
allow updating a service.

Deployment is more than updating a service: it is often a combination of
different tasks specific to your organisation. These tasks then must be
configured in Continuous Deployment tool of choice.

Flecs intends to make pipelines a first class citizen. This means your
application deployments become a whole lot more portable.

### Versus other tools

I'm writing this for fun, so I would strongly recommend using other tools.

[Hashicorp Waypoint](https://github.com/hashicorp/waypoint) and [AWS
Copilot](https://aws.amazon.com/containers/copilot/) are two that will probably
become the de-facto way to deploy to ECS in the future.

## Configuration

All configuration exists in a single `flecs.yaml` file.

### Pipeline

The pipeline is configured by specifying the type of task. Each task
type has it's own configuration options:

| Name | What |
|------|------|
| script | Runs a local script |
| docker | Builds a Docker image and pushes it to an ECR repository |
| service | Creates and/or updates an ECS service |
| task | Runs a task in ECS using the specified definition |

An example configuring the pipeline:

```
pipeline:
  - script:
      inline: uptime
  - script:
      path: path/to/script
  - docker:
      dockerfile: Dockerfile
  - service:
      service: web
      definition: nginx
  - task:
      command: uptime
      definition: ubuntu
```

### Services

Services configure how to run a service in the cluster. The name of the service
is referred to by the `service` pipeline step:

```
pipeline:
  - service:
      service: web

services:
  web:
    name: nginx
    definition: nginx
```

### Definitions

Definitions configure your task definitions. The name of the definition is
referred to by a service definition, or the `task` pipeline step:

```

pipeline:
  - service:
      service: web

services:
  web:
    name: nginx
    definition: nginx

definitions:
  nginx:
    containers:
    - name: nginx
      image: nginx
```

### Environments

Setting different environments is completely optional, but if you've
configured your service and then want to deploy to a Production environment,
you can configure every option within `environments` configuration:

```
subnet_names:
  - default-subnet-a

environments:
  production:
    region: eu-west-1
    subnet_names:
      - production-subnet-a
      - production-subnet-b
```

Using `flecs -e production` will mean that Production configuration is used.

## Install

Clone the repository.

Run `go build`.
