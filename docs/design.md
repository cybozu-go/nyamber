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
You can specify the `VirtualDC` startSchedule and stopSchedule with cron format.


### Kubernetes workloads

#### `virtualdc-controller`

A deployment that controls Runner Pod.
It create Runner pod which runs dctest and service to access Runner pod.

#### `autovirtualdc-controller`

A deployment that create and delete VirutualDC resources according to set schedule.
- startSchedule and stopSchedule are empty: controller create virtualdc immediately
- startSchedule and stopSchedule are set: calculate NextTime from those schedule and set time to status
  - nextStartTime < now and nextstopTime < now: do nothing
  - now < nextStartTime and nextStopTime < now: start virtualdc
  - nextStopTime < now and now < nextStartTime: stop virtualdc
  - nextStartTime < now and nextStopTime < now: stop virtualdc

#### Runner pod (nyamber-runner)

A component to run dctest with entrypoint.
Runner pod execute entrypoint cli and start entrypoint with scripts.
