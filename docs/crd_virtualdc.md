
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
| necoBranch | Neco branch to use for dctest. If this field is empty, controller runs dctest with \"main\" branch | string | false |
| necoAppsBranch | Neco-apps branch to use for dctest. If this field is empty, controller runs dctest with \"main\" branch | string | false |
| skipNecoApps | Skip bootstrapping neco-apps if true | bool | false |
| command | Path to a user-defined script and its arguments to run after bootstrapping dctest | []string | false |
| resources |  | corev1.ResourceRequirements | false |

[Back to Custom Resources](#custom-resources)

#### VirtualDCStatus

VirtualDCStatus defines the observed state of VirtualDC

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| conditions | Conditions is an array of conditions. | []metav1.Condition | false |

[Back to Custom Resources](#custom-resources)
