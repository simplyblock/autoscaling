# NeonVM Field Mutability

**Note:** ALL fields **can** be mutable if the VM is completely stopped (`powerState == Stopped`).
The table below indicates mutability while the VM is **Running**.

| Field Path | Mutable? | Notes |
| :--- | :---: | :--- |
| `.spec.cpuScalingMode` | ❌ No | |
| `.spec.disks` | ❌ No | Completely immutable, except `blockDevice.persistentVolumeClaim.resources.requests.storage`. |
| `.spec.enableAcceleration` | ❌ No | |
| `.spec.enableNetworkMonitoring`| ❌ No | |
| `.spec.enableSSH` | ❌ No | |
| `.spec.extraNetwork` | ✅ Yes | Includes sub-fields like `.enable`, `.interface`. |
| `.spec.guest.cpus.limit` | ✅ Yes | |
| `.spec.guest.cpus.max` | ❌ No | |
| `.spec.guest.cpus.min` | ❌ No | |
| `.spec.guest.cpus.use` | ✅ Yes | |
| `.spec.guest.memorySlotSize` | ✅ Yes | |
| `.spec.guest.memorySlots.limit`| ✅ Yes | |
| `.spec.guest.memorySlots.max` | ❌ No | |
| `.spec.guest.memorySlots.min` | ❌ No | |
| `.spec.guest.memorySlots.use` | ✅ Yes | |
| `.spec.guest.ports` | ❌ No | Includes all elements (`name`, `port`, `protocol`). |
| `.spec.guest.rootDisk` | ❌ No | Includes sub-fields (`image`, `imagePullPolicy`, `size`). |
| `.spec.powerState` | ✅ Yes | |
| `.spec.qmp` | ✅ Yes | |
| `.spec.qmpManual` | ✅ Yes | |
| `.spec.restartPolicy` | ✅ Yes | |
| `.spec.runnerPort` | ✅ Yes | |
| `.spec.schedulerName` | ✅ Yes | |
| `.spec.terminationGracePeriodSeconds`| ✅ Yes | |

## Example YAML with Annotations

```yaml
spec:
  cpuScalingMode: QmpScaling # immutable
  disks: # immutable (except for blockDevice.persistentVolumeClaim.resources.requests.storage)
  - blockDevice: 
      persistentVolumeClaim: 
        claimName: vela-autoscaler-vm-block-data 
    mountPath: /var/lib/postgresql 
    name: data 
    readOnly: false 
    watch: false 
  - configMap: 
      name: vela-autoscaler-vm-compose 
    mountPath: /vela/config 
    name: compose 
    readOnly: false 
    watch: false 
  - mountPath: /vela/secrets/db 
    name: vela-db-secret 
    readOnly: false 
    secret: 
      secretName: vela-db 
    watch: false 
  enableAcceleration: true # immutable
  enableNetworkMonitoring: false # immutable
  enableSSH: true # immutable
  extraNetwork: # mutable
    enable: true
    interface: net1
  guest:
    cpus:
      limit: 500m # mutable
      max: 64 # immutable
      min: 500m # immutable
      use: 500m # mutable
    memorySlotSize: 256Mi # mutable
    memorySlots:
      limit: 9 # mutable
      max: 128 # immutable
      min: 4 # immutable
      use: 4 # mutable
    ports: # immutable
    - name: rest 
      port: 3000 
      protocol: TCP 
    rootDisk: # immutable
      image: docker.io/simplyblock/vela-image:sha-0c84d51 
      imagePullPolicy: Always 
      size: 4G 
  powerState: Running # mutable
  qmp: 20183 # mutable
  qmpManual: 20184 # mutable
  restartPolicy: Always # mutable
  runnerPort: 25183 # mutable
  schedulerName: autoscale-scheduler # mutable
  terminationGracePeriodSeconds: 5 # mutable
```
