# User Manual

## create VirtualDC Automatically

### Create AutoVirtualDC Resource
Apply the AutoVirtualDC manifest like this.
```
apiVersion: nyamber.cybozu.io/v1beta1
kind: AutoVirtualDC
metadata:
  name: auto-virtual-dc2
spec:
  template:
    spec:
        necoBranch: release
        necoAppsBranch: release
        command:
        - env
  startSchedule: "0 0 * * *"
  stopSchedule: "0 1 * * *"
  timeoutDuration: "20m"
```
If this manifest is applied, Nyamber creates virtualdc pods at 0:00 every day.
```
$ kubectl get pod -n nyamber-pod
NAME               READY   STATUS    RESTARTS   AGE
auto-virtual-dc2   1/1     Running   0          4s
```
You can access the pod with service
```
$ kubectl get service -n nyamber-pod
NAME               TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)   AGE
auto-virtual-dc2   ClusterIP   10.96.178.115   <none>        80/TCP    21s
```

## create VirtualDC manually
### Create VirtualDC Resource
Apply the VirtualDC manifest like this.
```
apiVersion: nyamber.cybozu.io/v1beta1
kind: VirtualDC
metadata:
  name: vdc-sample
spec:
  skipNecoApps: true
```
Nyamber create virtualdc pods immediately
```
$ kubectl get pod -n nyamber-pod
NAME         READY   STATUS    RESTARTS   AGE
vdc-sample   1/1     Running   0          2s
```
You can access the pod with service
```
$ kubectl get service -n nyamber-pod
NAME         TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)   AGE
vdc-sample   ClusterIP   10.96.233.168   <none>        80/TCP    13s
```
