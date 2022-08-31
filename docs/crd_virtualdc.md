
### Custom Resources

* [VirtualDC](#virtualdc)

### Sub Resources

* [VirtualDCList](#virtualdclist)
* [VirtualDCSpec](#virtualdcspec)
* [VirtualDCStatus](#virtualdcstatus)

#### VirtualDC

VirtualDC is the Schema for the virtualdcs API

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata |  | metav1.ObjectMeta | false |
| spec |  | [VirtualDCSpec](#virtualdcspec) | false |
| status |  | [VirtualDCStatus](#virtualdcstatus) | false |

[Back to Custom Resources](#custom-resources)

#### VirtualDCList

VirtualDCList contains a list of VirtualDC

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata |  | metav1.ListMeta | false |
| items |  | [][VirtualDC](#virtualdc) | true |

[Back to Custom Resources](#custom-resources)

#### VirtualDCSpec

VirtualDCSpec defines the desired state of VirtualDC

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| necoBranch | Neco branch is a target branch name for dctest | string | false |
| necoAppsBranch | Neco apps branch is a target branch name for dctest | string | false |
| skipNecoApps | Skip bootstrap of neco apps if this is true | bool | false |
| command | Command is run after creating dctest pods | []string | false |
| resources |  | corev1.ResourceRequirements | false |

[Back to Custom Resources](#custom-resources)

#### VirtualDCStatus

VirtualDCStatus defines the observed state of VirtualDC

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| conditions | Conditions is an array of conditions. | []metav1.Condition | false |

[Back to Custom Resources](#custom-resources)
