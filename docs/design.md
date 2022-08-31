# Design notes

## Motivation

To run dctest in kubernetes cluster

### Goals

- Develop the custom controller to run dctest in a pod
- Automate start and stop dctest

## Architecture & Components

### Kubernetes Custom Resources

#### `VirtualDC`

This is custom resource to deploy dctest pod.
Nyamber create dctest pod and service from its definition.

#### `AutoVirtualDC`

This is custom resource to automate deployment dctest pod.
Nyamber create `VirtualDC` custom resource from its definition according to the schedule field.


### Kubernetes workloads

#### `virtualdc-controller`

A deployment that controls Runner Pod.
It create Runner pod which runs dctest and service to access Runner pod.

#### `Autovirtualdc-controller`

A deployment that create and delete VirutualDC resources at the set time.

#### Runner pod (nyamber-runner)

A component to run dctest with entrypoint.
Runner pod execute entrypoint cli and start entrypoint with scripts.
