
### Custom Resources

* [AutoVirtualDC](#autovirtualdc)

### Sub Resources

* [AutoVirtualDCList](#autovirtualdclist)
* [AutoVirtualDCSpec](#autovirtualdcspec)
* [AutoVirtualDCStatus](#autovirtualdcstatus)

#### AutoVirtualDC

AutoVirtualDC is the Schema for the autovirtualdcs API

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata |  | metav1.ObjectMeta | false |
| spec |  | [AutoVirtualDCSpec](#autovirtualdcspec) | false |
| status |  | [AutoVirtualDCStatus](#autovirtualdcstatus) | false |

[Back to Custom Resources](#custom-resources)

#### AutoVirtualDCList

AutoVirtualDCList contains a list of AutoVirtualDC

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata |  | metav1.ListMeta | false |
| items |  | [][AutoVirtualDC](#autovirtualdc) | true |

[Back to Custom Resources](#custom-resources)

#### AutoVirtualDCSpec

AutoVirtualDCSpec defines the desired state of AutoVirtualDC

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| template | Template is a template for VirtualDC | VirtualDC | false |
| startSchedule | StartSchedule is time to start VirtualDC. This format is cron format(UTC). | string | false |
| stopSchedule | StopSchedule is time to stop VirtualDC. this format is cron format(UTC). | string | false |
| timeoutDuration | TimeoutDuration is the duration of retry. This format is format used by ParseDuration(https://pkg.go.dev/time#ParseDuration) | string | false |

[Back to Custom Resources](#custom-resources)

#### AutoVirtualDCStatus

AutoVirtualDCStatus defines the observed state of AutoVirtualDC

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| nextStartTime | Next start time of VirtualDC's schedule. | *metav1.Time | false |
| nextStopTime | Next stop time of VirtualDC's schedule. | *metav1.Time | false |

[Back to Custom Resources](#custom-resources)
