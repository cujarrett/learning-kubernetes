#### This is a work in progress

## What's it do?
My notes from the [Cloud Native Certified Kubernetes Application Developer (CKAD) Program](https://www.cncf.io/certification/ckad/).

__Disclaimer: most of this is pretty verbatim in an effort to not call it my own by to ensure I don't mess up new concepts by wordsmithing the material.__

I want to learn how to deploy, configure, and test containerized applications on a multi-node Kubernetes cluster.

## Status
- [x] - [Chapter 1: Before You Begin](https://github.com/cujarrett/certified-kubernetes-application-developer-course#chapter-1-before-you-begin)
- [x] - [Chapter 2: Kubernetes Architecture](https://github.com/cujarrett/certified-kubernetes-application-developer-course/blob/master/README.md#chapter-2---kubernetes-architecture)
- [x] - [Chapter 3: Build](https://github.com/cujarrett/certified-kubernetes-application-developer-course#chapter-3-build)
- [x] - [Chapter 4: Design](https://github.com/cujarrett/certified-kubernetes-application-developer-course#chapter-4-design)
- [x] - [Chapter 5: Deployment Configuration](https://github.com/cujarrett/certified-kubernetes-application-developer-course#chapter-5-deploy-configuration)
- [x] - [Chapter 6: Security](https://github.com/cujarrett/certified-kubernetes-application-developer-course#chapter-6-security)
- [x] - [Chapter 7: Exposing Applications](https://github.com/cujarrett/certified-kubernetes-application-developer-course#chapter-7-exposing-applications)
- [x] - [Chapter 8: Troubleshooting](https://github.com/cujarrett/certified-kubernetes-application-developer-course#chapter-8-troubleshooting)
- [x] - [Kubernetes CKAD Example Questions Practical Challenge Series](https://codeburst.io/kubernetes-ckad-weekly-challenges-overview-and-tips-7282b36a2681)
- [x] - [CKAD-exercises](https://github.com/dgkanatsios/CKAD-exercises)
- [x] - [Kubernetes Certified Application Developer (CKAD) with Tests](https://www.udemy.com/course/certified-kubernetes-application-developer/)
- [x] - Take Cloud Native Certified Kubernetes Application Developer (CKAD) Program test

## Chapter 1: Before You Begin
#### Outcomes
- Containerize and deploy a new Python script
- Configure the deployment with ConfigMaps, Secrets, and SecurityContexts
- Understand multi-container pod design
- Configure probes for pod health
- Update and roll back an application
- Implement services and NetworkPolicies
- Use PersistentVolumeClaims for state persistence

#### Common utilities across distributions
- __systemd__ - system startup and service management. Used by the most common distributions, replacing the `SysVinit` and `Upstart` packages. Replaces `service` and `chkconfig` commands.

- __journald__ - manages system logs. It's a `systemd` service that collects and stores logging data. It creates and maintains structured, indexed journals based on logging information that is received from a variety of sources. Depending on distribution, text-based system logs may be replaced.

- __firewalld__ - firewall management daemon. Provides a dynamic managed firewall with support for network/firewall zones to define the trust level of network connections or interfaces. It has support for IPv4, IPv6 firewall settings and for Ethernet bridges. This replaces the `iptables` configurations.

- __ip__ - network display and configuration tool. The `ip` program is part of the `net-tools` package, and is designed to be a replacement for the `ifconfig` command. The `ip` command will show or manipulate routing, network devices, routing information and tunnels.

#### Translating Assistants
- [SysVinit Cheat Sheet](https://fedoraproject.org/wiki/SysVinit_to_Systemd_Cheatsheet)
- [Debian CHeat Sheet](https://wiki.debian.org/systemd/CheatSheet)
- [openSUSE Cheat Sheep](https://en.opensuse.org/openSUSE:Cheat_sheet_13.1#Services)

## Chapter 2: Kubernetes Architecture
#### Learning Objectives
- Discuss the history of Kubernetes
- Learn about master node components
- Lean about minion (worker) node components
Understand the container Network Interface (CNI) configuration and Network Plugins

#### What is Kubernetes?
An open-source system for automating deployment, scaling, and management of containerized applications. - kubernetes.io

Kubernetes is built on 15 years of experience at Google in a project called `borg`.

Kubernetes is written in `Go` language.

![kubernetes-architecture](/media/kubernetes-architecture.png)
* diagram from Cloud Native Certified Kubernetes Application Developer (CKAD) Program

#### Terminology

The `master` runs an API server, a scheduler, various controllers, and a storage system to keep the state of the cluster, container settings, and the network configuration.

- `kubectl` is a common local client manner of interacting with the Kubernetes API.

- `kube-scheduler` sees the requests for running containers coming to the APO and finds a suitable node to run that container in.

- Each node in the cluster runs two processes - a `kubelet` and a `kube-proxy`. The `kubelet` receives requests to run the containers, manages any necessary resources and watches over them on the local node. The `kube-proxy` creates and manages networking rules to expose the container on the network.

- A `Pod` consists of one or more containers which share an IP address, access to storage and namespace. Typically one pod runs an application, while other containers support the primary application.

- Orchestration is managed through a series of watch-loops or `controllers`.Each server interrogates the `kube-apiservener` for a particular object state, modifying the object until the declared state matches the current state.

- A `Deployment` ensures that resources are available, such as IP address and storage, and then deploys a `ReplicaSet`

- The `ReplicaSet`is a controller which deploys and restarts containers (Docker by default) until the requested number of containers is running.

- `Jobs` and `CronJobs` can be used to handle single or recurring tasks.

- `Labels` are arbitrary strings which become part of the object metadata. These can be used when checking or changing the state of objects without having to know individual names or UIDs.

- `Taints` can be used to discourage Pod assignments, unless the pod has a `toleration` in its metadata.

- `Annotations` can be added to the metadata but can not be used by Kubernetes commands. This information can be used by third party agents and other tools.

#### Master Node
The Kubernetes master runs various server and manager processes for the cluster. Among the components of the master node are the `kube-apiserver`, the `kube-scheduler`, and the `etcd` database. As the software has matured, new components have been created to handle dedicated needs such as the `cloud-controller-manager` which handles the tasks once handled by the `kube-controller-manager` to interact with other tools such as `Rancher` or `DigitalOcean` for third party cluster management and reporting.

- All calls both internal and external traffic are handled via the `kube-apiserver` and is the only connection to the `etcd` database.

- The `kube-scheduler` uses an algorithm to determine which node will host a Pod of containers. The scheduler will try to view available resources such as volumes to bind, and then try and retry to deploy the Pod based on availability and success.

- The `etcd` database stores the state of the cluster, networking, and other persistent information. Rather than finding and changing an entry, values are always appended to the end. Previous copies of the data are then marked for future removal by a compaction process. It works with `curl` and other HTTP libraries and pro
vides reliable watch queries.

- Simultaneous requests to update a value all travel via the `kube-apiserver`, which then passes along the request to `etcd` in a series. The first request would update the database. The second request would no longer have the same version number, in which case the `kube-apiserver` would reply with a `409` to the requester. There is no logic past that response on the server side, meaning the client needs to expect this and act upon the denial to update.

- There is a `master` database along with `followers`. They communicate with each other on an ongoing basis to determine which will be `master`, and determine another in the event of failure.

- The `kube-controller-manager` is a core loop daemon which interacts with the `kube-apiserver` to determine the state of the cluster. If the state does not match, the manager will contact the necessary controller to match the desired state.

- The `cloud-controller-manager` interacts with agents outside the cloud.

#### Worker Nodes

- All worker nodes run the `kubelet` and `kube-proxy`, as well as the container engine such as `Docker` or `cri-o`.

- `kubelet` interacts with the underlying Docker engine installed on all the worker nodes and makes sure the containers that need to run are actually running.

- `kube-proxy` is in charge of managing the network connectivity to the containers. This is done via use of `iptables` entries.

- `Supervisord` is a lightweight process monitor used in traditional Linux environments to monitor and notify about other processes. In a cluster this daemon will monitor both the `kubelet` and `docker` processes. It will try to restart them if they fail and log events.

- `Fluentd` is a non native solution for unified cluster wide logging.

- `kubelet` agent is a heavy lifter for changes and configuration on worker nodes. It accepts API calls for Pod specifications. It also ensures that any storage, Secrets, or ConfigMaps are created or has access.

#### Pods
- Smallest unit is a Pod in Kubernetes.

- There is only one IP address per Pod. If there is more than one container per Pod they must share the singular IP address.

#### Services
- `Services` - a microservice handling a bit of traffic such as a single `NodePort` or `LoadBalancer` to distribute inbound requests among many Pods. It also connects resources together and will reconnect should something die. The Services also handles the access policies for inbound requests which is useful for resource control as well as for security.

#### Controllers
- `Controllers` - Receives objects which is an array of deltas and as long as the delta is not of the type `Deleted` then the controller is used to create or modify some object until it matches the specification.

- The controllers `endpoints`, `namespace`, and `serviceaccounts` each manage the eponymous resources for Pods.

#### Single IP per Pod
- Pods can communicate to each other via the loopback interface, write to files on a common filesystem, or inter-process communication (IPC).

#### Network Setup
The three main networking challenges to solve in a container orchestration system are:
- Coupled container to container communications (solved by the pod concept)
- Pod to pod communications
- External to pod communications (solved by the services concept)

- Kubernetes expects the network configuration to enable pod to pod communications to be be available. Kubernetes will not do this for you.

- Container Network Interface (CNI) - Emerging specification with associated libraries  to write plugins that configure container networking and remove allocated resources when the container is deleted. Its aim is to provide a common interface between the various networking solutions and container runtimes. As the CNI specification is language-agnostic, there are many plugins from Amazon ECS, to SR-IOV, to Cloud Foundry, and more.

Sample CNI network configuration file that defines a standard __Linux__ bridge named `cni0`, which will give out IP addresses in the subnet `10.22.0.0./16`. The bridge plugin will configure the network interfaces in the correct namespaces to define the container network properly. The [README of the CNI GitHub repository](https://github.com/containernetworking/cni) has more information.
```js
{
  "cniVersion": "0.2.0",
  "name": "mynet",
  "type": "bridge",
  "bridge": "cni0",
  "isGateway": true,
  "ipMasq": true,
  "ipam": {
    "type": "host-local",
    "subnet" "10.22.0.0/16",
    "routes": [
      { "dst": "0.0.0.0/0" }
    ]
  }
}
```

#### Pod-to-Pod Communication
While the CNI plugin can be used to configure the network of a pod and provide a single IP per pod, CNI does not help you with pod-to-pod communication across nodes.

The requirement from __Kubernetes__ is the following:
- All pods can communicate with each other across nodes
- All nodes can communicate with all pods
- No Network Address Translation (NAT)

Basically, all IP involved (nodes and pods) are routable without NAT. This can be achieved at the physical network infrastructure if you have access to it (e.g. Google Kubernetes Engine). Or, this can be achieved with a software defined overlay with solutions like:
- Weave
- Flannel
- Calico
- Romana

#### The Cloud Native Computing Foundation
__Kubernetes__ is an open source software with an Apache license. Google donated __Kubernetes__ to a newly formed collaboration project within __The Linux Foundation__ in July 2015, when __Kubernetes__ reached v1.0 release. The project is known as the __Cloud Native Computing Foundation (CNCF)__. CNCF is not just about Kubernetes, it serves as the governance body for open source software that solves specific issues faced by cloud native applications (i.e. applications that are written specifically for a cloud environment).

Note: Since CNCF now owns the __Kubernetes__ copyright, contributions to the source need to sign a contributor license agreement (CLA) with CNCF, just like any contributor to an Apache-licensed project signs a CLA with the __Apache Software Foundation__.

#### Deployment(s)
Creating a pod without a deployment does not take advantage of orchestration abilities of Kubernetes. Deployments offer this ability and offer scalability, reliability, monitorable, and updates.

#### Additional Resources
- Read the [Borg Paper](https://ai.google/research/pubs/pub43438)
- Listen to [John Wilkes talking about Borg and Kubernetes](https://www.gcppodcast.com/post/episode-46-borg-and-k8s-with-john-wilkes/)
- Add the __Kubernetes__ [community hangout](https://github.com/kubernetes/community) to your calendar, and attend at least once
- Join the community on [Slack](http://slack.kubernetes.io/) and go in the #kubernetes-users channel
- Check out the very active [Stack Overflow community](https://stackoverflow.com/search?q=kubernetes)

## Chapter 2: Kubernetes Architecture - Lab
- Deploy a New Cluster
- Deploy a Master Node using `Kubeadm`
- Deploy a Worker Node using `Kubeadm` and join it to the cluster
- Create a basic Pod
- Create a multi container Pod
- Set up a service
- Set up a Deployment

#### Having worker node join cluster example
```sh
kubeadm join <master-ip>:<master-port> --token <token> --discovery-token-ca-cert-hash sha256:<hash>
```

#### Questions I have after Chapter 2: Kubernetes Architecture
- What are things to look for when looking at the `kubectl describe` output?

#### Chapter 2: Kubernetes Architecture - Review
1. Which of the following are part of a Pod
  - a. One or more containers
  - b. Shared IP address
  - c. One namespace
  - d. All of the above

2. Which company developed Borg as an internal project?
  - a. Amazon
  - b. Google
  - c. IBM
  - d. Toyota

3. In what database are the objects and the state of the cluster stored?
  - a. ZooKeeper
  - b. MySQL
  - c. etcd
  - d. Couchbase

4. Orchestration is managed through a series of watch-loops or controllers. Each interrogates the _____ for a particular object state.
  - a. kube-apiserver
  - b. etcd
  - c. kubelet
  - d. ntpd

[answers](./review-answers.md)

## Chapter 3: Build

#### Chapter 3: Build Objectives
- Lean about runtime and container options
- Containerize an application
- Host a local repository
- Deploy a multi-container pod
- Configure readinessProbes
- Configure livenessProbes

#### Container Options
__Container Runtime__ is the component which runs the containerized application upon request. Docker Engine remains the default for Kubernetes.

The containerized image is moving from Docker to one that is not bound by higher-level tools and that is more portable across operating systems and environments. The Open Container Initiative (OCI) was formed to help with this. Docker donated their __libcontainer__ project to form a new codebase called [__runC__](https://github.com/opencontainers/runc) to support these goals.

#### Docker
Launched in 2013, Docker has become synonymous with running containerized applications.Docker made containerizing, deploying, and consuming applications easy.

#### Container Runtime Interface (CRI)
The goal of the Container Runtime Interface (CRI) is to allow easy integration of container runtimes with __Kubelet__. By providing a protobuf method for API, specifications and libraries, new runtimes can easily be integrated without needing deep understanding of kublet intervals.

The project is in alpha stage, with lots of development in action. Now that the Docker-CRI integration is done, new runtimes should be easily added  and swapped out. Some examples that are ongoing development now:
  - __cri-o__
  - __rklet__
  - __frakti__

#### Rkt
The __Rkt__ pronounced rocket provides a CLI for running containers.Announced by CoreOS in 2014, it is now part of the Cloud Native Foundation family of projects. Learning from early Docker issues, it is focused on being more secure, open and interoperable. Many of its features have been met by Docker improvements. It uses the __appc__ specification and can run Docker, __appc__ and OCI images. It deploys immutable pods.

#### CRI-O
This project is currently in incubation as part of Kubernetes. It uses the Kubernetes Container Interface which OCI-computable runtimes, thus the name CRI-O. It currently supports __runC__ as default and Clear Containers, but the stated goal of the project is to work with any OCI-compliant runtime. While CRI-O is newer than Docker or Rkt it has gained major support because of its flexibility and compatibility.

#### Containerd
The intent of the __containerd__ project is not to build a user-facing tool; instead, it focused on exposing highly-decoupled low-level primitives:
  - Defaults to runC to run containers according to OCI Specifications
  - Intended to be embedded into larger systems
  - Minimal CLI, focused on debugging and development

With a focus on supporting the low-level, or backed plumbing of containers, this project is better suited to integration and operational teams building specialized products, instead of typical build, ship, and run application.

#### Containerizing an Application
To containerize and application, you begin by creating your application. Not all applications do well with containerization. The more stateless and transient the application better. You need to remove any environmental configuration, as those can be provided by other tools like ConfigMaps and secrets.Develop the application until you have a single build artifact which can be deployed to multiple environments without needing to be changed, using decoupled configuration instead.

Legacy applications become a series of objects and artifacts, residing among multiple containers.

- __buildah__ is a tool focused on creating Open Container Initiative (OCI) images. It allows for creating images with and without a Dockerfile. It allows for creating images with and without a Dockerfile. It also does not require superuser privilege.

- __podman__ is a "pod manager" tool that allows for the life cycle management of a container, including creating, starting, stoping and updating. It can be viewed as a replacement for `docker run`.

#### Creating the Dockerfile
1. Create `Dockerfile`
2. Verify the image
```
sudo docker image
sudo docker run simpleapp
```
3. Push to repository
```
sudo docker push
```

#### Hosting a Local Repository
While Docker has made it easy to upload images to their Hub, these images are then public and accessible to the world. A common alternative is to create a local repository and push images there. While it can add administrative overhead, it can save downloading bandwidth, while maintaining privacy and security.

Once the local repository has been configured, you can use `docker tag`, then `push` to populate the repository with local images. Consider setting up an insecure repository until you know it works, then configure TLS access for greater security.

#### Creating a Deployment
Once you can push and pull images using `docker` command, try to run a new deployment inside Kubernetes using the image. The string passed to the `--image` argument includes the repository to use, the name of the application, then the version.

Use `kubectl` run to test the image:
`kubectl run <Deploy-Name> --image<repo>/<app-name>:<version>`
`kubectl run time-date --image=10.110.186:5000/simpleapp:v2.2`

Be aware that the string "latest" is just that string, and has no reference to reference to actually being the latest. Only use this string if you have a process in place to name and rename versions of the application as they become available. If this is not understood, you could be using the "latest" release, which is several releases older than the most current.

Verify the Pod shows a running status and that all included containers are running as well.

`kubectl get pods`

Test that the application is performing as expected, running whichever tempest and QA testing the application has. Terminate a pod and test that the application is a transient as designed.

#### Running commands in a Container
`kubectl exec` can be used to run commands in a container. The `-it` makes this interactive versus without that the command(s) run without interaction or access.

```
kubectl exec -it <Pod-Name> --/bin/bash
```

#### Multi-Container Pod
When it doesn't make sense to recreate an entire image to add functionality like a shell or logging agent, you can add another contianer to the pod. This is called a sidecar. Each container in the pod should be transient and decoupled. If adding another container to the pod limits the scalability or transient nature of the orginal applicaiton then consider a new build.

Every container in a pod shares a single IP address and namespace.

Every container in a mult-container pod has equal access to the storage so configure your applications to not have conflicting writes.

#### readinessProbe
You can use a readinessProbe to monitor for a zero exit code menaing the pod is ready to accept traffic. HTTP GET is another type of probe that would look for a return code of 200-399. Lastly, a TCP Socket Probe (tcpSocket) will attempt to open a socket on a predetermined port, and keep trying based on periodSeconds. Once the port can be opened, the container is considered healthy.

#### livenessProbe
Similar to how we can use a readinessProbe to watch for a container to be ready for traffic, we can use a livenessProbe probe to montior for health state of a pod. If a pod is determined to be unhealthy it will be terminated. If the terminated pod is under a controller, a replacement would be spawned.

readinessProbe and livenessProbe probes are often configured into a deployment to ensure applications are ready for traffic and remain healthy.

## Exercise 3.1: Deploy a New Application
Deploying a Python application, test it using Docker, ingest it into Kubernetes and configure probes to ensure it continues to run.

Creating a Dockerfile
```
FROM python:2
ADD simple.py /

## RUN pip install pystrich
CMD [ "python", "./simple.py" ]
```

Building the container
```
sudo docker build -t simpleapp .
```

- `-t` is to tag the image
- `.` is to build the image from the current location using the current locaitons `Dockerfile`

Check images for the one just built
```
sudo docker images
```

Run the image
```
sudo docker run simpleapp
```

#### Extras
Install python with `apt-get`
```
sudo apt-get -y install python
```

- `-y` - Automatic yes to prompts. Assume "yes" as answer to all prompts and run non-interactively.

Find a file
```
sudo find / -name <file>
```

Print the last lines of a file
```
sudo tail <file>
```

Change access permissions of a file
`chmod`

For example to make temp.foo an executable
```
chmod +x temp.foo
```

## Exercise 3.2: Configure A Local Docker Repo
Creating a Kubernetes volume

volume1.yaml
```
apiVersion: v1
kind: PersistentVolume
metadata:
  labels:
    type: local
  name: task-pv-volume
spec:
  accessModes:
    - ReadWriteOnce
  capacity:
    storage: 200Mi
  hostPath:
    path: /tmp/data
  persistentVolumeReclaimPolicy: Retain
```

```
kubectl create -f volume1.yaml
```

View volumes
```
kubectl get pv
```

## Exercise 3.3: Configure Probes
readinessProbe example:
```
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  annotations:
    deployment.kubernetes.io/revision: "1"
  creationTimestamp: "2019-06-22T03:32:58Z"
  generation: 2
  labels:
    app: try1
  name: try1
  namespace: default
  resourceVersion: "30797"
  selfLink: /apis/extensions/v1beta1/namespaces/default/deployments/try1
  uid: 6e266858-949e-11e9-bdb6-42010a800002
spec:
  progressDeadlineSeconds: 600
  replicas: 6
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      app: try1
  strategy:
    rollingUpdate:
      maxSurge: 25%
      maxUnavailable: 25%
    type: RollingUpdate
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: try1
    spec:
      containers:
      - image: 10.99.149.30:5000/simpleapp:latest
```

Check deployments for readiness
```
kubectl get deployment
```

readinessProbe
```
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  annotations:
    deployment.kubernetes.io/revision: "1"
  creationTimestamp: "2019-06-22T03:32:58Z"
  generation: 2
  labels:
    app: try1
  name: try1
  namespace: default
  resourceVersion: "30797"
  selfLink: /apis/extensions/v1beta1/namespaces/default/deployments/try1
  uid: 6e266858-949e-11e9-bdb6-42010a800002
spec:
  progressDeadlineSeconds: 600
  replicas: 6
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      app: try1
  strategy:
    rollingUpdate:
      maxSurge: 25%
      maxUnavailable: 25%
    type: RollingUpdate
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: try1
    spec:
      containers:
      - image: 10.99.149.30:5000/simpleapp:latest
        imagePullPolicy: Always
        name: simpleapp
        readinessProbe:
          periodSeconds: 5
          exec:
            command:
            - cat
            - /tmp/healthy
        resources: {}
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
      - name: goproxy
        image: k8s.gcr.io/goproxy:0.1
        ports:
        - containerPort: 8080
        readinessProbe:
          tcpSocket:
            port: 8080
          initialDelaySeconds: 15
          periodSeconds: 20
      dnsPolicy: ClusterFirst
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext: {}
      terminationGracePeriodSeconds: 30
status:
  availableReplicas: 6
  conditions:
  - lastTransitionTime: "2019-06-22T03:32:58Z"
    lastUpdateTime: "2019-06-22T03:33:00Z"
    message: ReplicaSet "try1-65c44cb459" has successfully progressed.
    reason: NewReplicaSetAvailable
    status: "True"
    type: Progressing
  - lastTransitionTime: "2019-06-22T03:33:16Z"
    lastUpdateTime: "2019-06-22T03:33:16Z"
    message: Deployment has minimum availability.
    reason: MinimumReplicasAvailable
    status: "True"
    type: Available
  observedGeneration: 2
  readyReplicas: 6
  replicas: 6
  updatedReplicas:
```

Check probe readiness
```
kubectl describe pod <pod name> | grep -E 'State|Ready'
```

#### Chapter 3 Random Syntax I want to remember
- How to run an image?

```
kubectl run app-name --image=repo/app-name:version
```

#### Chapter 3: Build - Review
1. Which of the probes is used to check that a container is ready to accept traffic, but not checked after?
  - a. livenessProbe
  - b. readinessProbe
  - c. GuestAgentProbe
  - d. HTTPCheck

2. Which of the following are methods for probing container health?
  - a. Command returns exit code zero
  - b. HTTP Get returns a code from 200-399
  - c. Establish connection on a TCP socket
  - d. All of the above

3. Which of the following commands will run a command inside an active container?
  - a. `kubectl exec`
  - b. `kubectl run`
  - c. `kubectl create`
  - d. `kubectl describe`

4. Pods should never have more than one container?
  - true
  - false

[answers](./review-answers.md)

## Chapter 4: Design

#### Chapter 4: Design Learning Objectives
- Define an application's resource requirements
- Understand multi-container pod design patterns: `Ambassador`, `Adapter`, `Sidecar`
- Discuss application design concepts

#### Traditional Applications vs. Kubernetes Design
Optimal Kubernetes deployment design challenges traditional patterns. Where traditional applications were built and deployed with the expectation of long-running and strong interdependence Kubernetes patterns use decoupled resources.

Transience is a concept where each object should be developed with the expectation that other components will die and be rebuilt.

#### Managing Resource Usage
- The `kube-scheduler` or custom scheduler uses the `PodSpec` to determine the best node for deployment.

In addition to administrative tasks to grow or shrink the cluster or the number of Pods, there are `autoscalers` which can add or remove nodes or memory usage by individual containers.

By default a Pod will use as much CPU and memory as the workload requires.

Through the use of resource `requests` the scheduler will only schedule a Pod to a node if the resources exist to meet all requests on that node.

Monitoring resource usage cluster-wide is not an included feature of Kubernetes. Other projects like Prometheus are used instead.

#### CPU
CPU requests are made in CPU units, each being a millicore, using the Latin word for thousand. Some documentation uses millicore and others use millicpu.

A request for .7 of a CPU would be equal to 700 millicore.

#### Memory (RAM)
With Docker engine, the `limits.memory` value is converted to an integer value and becomes the value to the `docker run --memory <value> <image>` command.

The handling of a container which exceeds its memory limit is not definite. It may be restarted, or, if it asks for more than the memory request setting, the entire Pod may be evicted from the node.

```
spec.containers[].resources.limits.memory

spec.containers[].resources.requests.memory
```

#### Ephemeral Storage (Beta Feature in v1.14)
Container files, logs, and `EmptyDir` storage, as well as Kubernetes cluster data, reside on the root filesystem of the host node. As storage is a limited resource, you may need to manage it as other resources. The scheduler will only choose a node with enough space to meet the sum of all the container requests. Should a particular container, or the sum of the containers in a Pod, use more than the limit, the Pod will be evicted.

```
spec.containers[].resources.limits.ephemeral-storage

spec.containers[].resources.requests.ephemeral-storage
```

#### Multi-Container Pods
The idea of multiple containers in a Pod goes against the architectural idea of decoupling as much as possible however there are some situations where a second or third co-located container makes sense such as for app logging if it's not handled within the app.

#### Sidecar Container
The idea for a `sidecar container` is to add some functionality not present in the main container. Rather than bloating code, which may not be necessary in other deployments, adding a container to handle a function such as logging solves the issue, while remaining decoupled and scalable. Prometheus monitoring and Fluentd logging leverage sidecar containers to collect data.

#### Adapter Container
An `Adapter Container` could be used to standardize output of the main container so that an existing enterprise monitoring tool can read it in the desired format.

#### Ambassador
An ambassador allows for access to the outside world without having to implement a service or another entry in an ingress controller:
- Proxy local connection
- Reverse proxy
- Limits HTTP requests
- Re-route from the main container to the outside world.

[Ambassador](https://www.getambassador.io/) is an open source Kubernetes-native API gateway for microservices built on Enjoy.

#### Jobs
Jobs are part of the `batch` API group. They are used to run a set number of pods to completion. If a pod fails, it will be restarted until the number of completion is reached.

While they can be seen as a way to do batch processing in Kubernetes, they can also be used to run one-off pods. A `Job` specification will have parallelism and a completion key, both of which default to 1 if they are omitted from the spec. Several `Job` patterns can be implemented, like a traditional work queue.

`Cronjobs` - Work in a similar manner to Linux jobs with the same time syntax.

The `concurrencyPolicy` determines if an existing job is still running should the new job wait or not. `Allow` (default) another concurrency job will run. `Replace` will cancel the current job and starts a new job in its place.

```
.spec.concurrencyPolicy
```

## Exercise 4.1: Planning the Deployment

#### Evaluate Network Plugins
There are lots of Container Network Interface (CNI) options. Some include:
- [Project Calico](https://docs.projectcalico.org/v3.0/introduction/)
- [Calico with Canel](https://docs.projectcalico.org/v3.0/getting-started/kubernetes/installation/hosted/canal/)
- [Weave Works](https://www.weave.works/docs/net/latest/kubernetes/kube-addon/)
- [Flannel](https://github.com/coreos/flannel)
- [Romana](https://romana.io/how/romana_basics/)
- [Kube Router](https://www.kube-router.io/)
- [Kopeio](https://github.com/kopeio/networking)

Questions:
1. Which of the plugins allow `vxlans`?
- Canal, Flannel, Weave, Romana, Kopeio

2. Which are layer 2 plugins?
- Romana, Kopeio

3. Which are layer 3 plugins?
- Calico, Flannel, Romana

4. Which allow network policies?
- Calico, Kube Router

5. Which can encrypt all TCP and UDP traffic?
- Weave

#### Multi-container Pod Considerations
1. Which deployment method would allow the most flexibility, multiple applications per pod or one per pod?

2. Which deployment method allows for the most granular scalability?

3. Which have the best performance?

4.How many IP addresses are assigned per pod?

5. What are some ways containers can communicate within the same pod?

6. What are some reasons you should have multiple containers per pod?

#### Chapter 4: Design - Review
1. Which of the following are helper container types? Choose all answers that apply.
  - a. Ambassador
  - b. Adapter
  - c. ProbeHelper
  - d. Sidecar

2. If a Pod uses more CPU than allowed, it will be evicted. True or False?
  - True
  - False

3. If a Pod tries to use more memory than the node has, it will be evicted. True or False?
  - True
  - False

4. If a pod uses more than `spec.containers[].resources.limits.ephemral-storage` it will ___________. Select the correct answer.
  - a. Be restarted
  - b. Be migrated
  - c. Be evicted
  - d. Remain the same

[answers](./review-answers.md)

#### Questions I have after Chapter 4: Design
`. > CPU: Should a container use more resources than allowed, it won't be killed. The exact amount of overuse is not definite.

```
spec.containers[].resources.limits.cpu

spec.containers[].resources.requests.cpu
```

What? :smiley:

2. The handling of a container which exceeds its memory limit is not definite. It may be restarted, or, if it asks for more than the memory request setting, the entire Pod may be evicted from the node.

```
spec.containers[].resources.limits.memory

spec.containers[].resources.requests.memory
```

What? :smiley:

3. What is the difference between an `ingress` and an `egress?

4. Write a multi container per pod deployment, service, and ingress.

## Chapter 5: Deploy Configuration

#### Chapter 5: Deploy Configuration Learning Objectives
- Understand and create persistent volumes
- Configure persistent volume claims
- Manage volume access modes
- Deploy an application with access to persistent storage
- Discuss the dynamic provisioning of storage
- Configure secrets and ConfigMaps
- Update a deployed application
- Roll back to a previous version

#### Volumes Overview
A Kubernetes volume lives as long as the pod at least, not the containers within the pod. If a
container dies another new container can access the volume when it spins up within the pod. A
volume is a directory, possibly pre-populated, made available to containers in a pod. The creation
the directory, the backend storage of the data depend on the volume type. As of v1.14 there are 28
different volume types ranging from `rbd` for Ceph, to `NFS`, to dynamic volumes from a cloud
provider like Google's `gcePersistentDisk`. Each has particular configuration options and
dependencies.

A Persistent Volume Claim can provide state persistency allowing data to outlive the pod.

A Pod specification can declare one or more volumes and where they are made available. Each requires a name, a type, and a mount point. The same volume can be made available to multiple containers within a Pod, which can be a method of container-to-container communication. A volume can also be made available to multiple Pods, which each given  an `access mode` to write. There is no concurrency checking which means data corruption is probable, unless outside locking takes place.

#### Volume Spec
`emptyDir` is a type of volume storage. It will create a directory in the container, but not mount any storage. Any data created is written to the shared container space. It would not be persistent storage, when the Pod is destroyed the directory is deleted along with the container.

The YAML below will create a Pod with a single container with a volume named `scratch-volume` created, which would create the `/scratch` directory inside the container.

```
apiVersion: v1
kind: Pod
metadata:
  name: busybox
  namespace: default
spec:
  containers:
  - image: busybox
    name: busy
    command:
      - sleep
      - "3600"
    volumeMounts:
  - mountPath: /scratch
    name: scratch-volume
  volumes:
  - name: scratch-volume
          emptyDir: {}
```

#### Shared Volume Example

The following example creates a pod with two containers, both with access to a shared volume:

```
containers:
    - image: busybox
  volumeMounts:
    - mountPath: /busy
  name: test
  name: busy
    - image: busybox
  volumeMount: /box
  name: test
  name: box
  volumes:
    - name: test
    emptyDir: {}
```

#### Persistent Volumes and Claims
A persistent volume (pv) is a storage abstraction used to retain data longer than the Pod using it. Pods define a volume of type `persistentVolumeClaim(pvc)` with various parameters for size and possibly the type of backend storage  known as `StorageClass`. The cluster then attaches it to the `persistentVolume`.

There are several phases to persistent storage:

- `Provisioning` can be from pvs created in advanced by the cluster administrator, or requested from a dynamic source, such as the cloud provider.

- `Binding` occurs when a control loop on the master notices the PVC, containing an amount of storage, access request, and optionally, a particular `StorageClass`. The watcher locates a matching PV or waits for the StorageClass provisioner to create one. The pv must match at least the storage amount requested, but may provide more.

- The `use` phase begins when the bound volume is mounted for the Pod to use, which continues as long as the Pod requires.

- `Releasing` happens when the Pod is done with the volume and an API request is sent, deleting the PVC. The volume remains in the state from when the claim is deleted until available to a new claim. The resident data remains depending on the `persistentVolumeRelaimPolicy`

- The `reclaim` phase has three options:
  - `Retain`, which keeps the data intact, allowing for an administrator to handle the storage and data.
  - `Delete` tells the volume plugin to delete the API object, as well as the storage behind it.
  - The `Recycle` option runs an `rm -rf /mountpoint` and then makes it available to a new claim. With the stability of dynamic provisioning, the `Recycle` option is planned to be deprecated.

#### Persistent Volume
The following example shoes a basic declaration of a `PersistentVolume` using the `hostPath` type.

```
kind: PersistentVolume
apiVersion: v1
metadata:
  name: 10Gpv01
  labels:
    type: local
spec:
  capacity:
    storage: 10Gi
  accessModes:
    - ReadWriteOnce
  hostPath:
    path: "/somepath/data01"
```
Each type will have its own configuration settings.

Persistent volumes are cluster scoped, but persistent volume claims are namespace-scoped.

#### Persistent Volume Claim
```
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: myclaim
spec:
  accessModes:
    - ReadWriteOnce
  rescources:
    requests:
      storage: 8 Gi
(in the pod)
...
spec:
  containers:
...
  volumes:
    - name: test-volume
    persistentVolumeClaim:
      claimName: myClaim
```

The Pod configuration would also be as complex as this:

```
volumeMounts:
    - name: Cephd
      mountPath: /data/rbd

...
volumes:
  - name: rbdpd
    rbd:
      monitors:
      -  '10.19.14.22.6789'
      -  '10.19.14.23.6789'
      -  '10.19.14.24.6789'
      pool: k8s
      image: client
      fsType: ext4
      readOnly: true
      user: admin
      keyring: /etc/ceph/keyring
      imageformat: "2"
      imagefeatures: "layering"
```

#### Secrets
Using the Secret API you can create an encoded value. A secret is base 64 encided by defualt, it's not encrypted by default.

#### Using Secrets via Environment Variable

```
spec:
  containers:
  - image: mysql:5.5
    env:
    - name: MYSQL_ROOT_PASSWORD
      valueFrom:
        secretKeyRef:
          name: mysql
          key: password
    name: mysql
```

#### Mounting Secrets as Volumes

```
...
spec:
  containers:
  - image: busybox
    command:
      - sleep
      - "3600"
    volumeMounts:
    - mountPath: /mysqlpassword
      name: mysql
    name: busy
  volumes:
  - name: mysql
    secret:
      secretName: mysql
```

#### Portable Data with ConfigMaps
`ConfigMap` is similar to a Secret except that the data is not encoded. Using a `ConfigMap` decouples a container image from the configuration artifacts.

`ConfigMap`s can be consumed in many ways:
- Pod environment variables from single or multiple `ConfigMaps`
- Use `ConfigMap` values in Pod commands
- Populate Volume from `ConfigMap`
- Add `ConfigMap` data to specific path in Volume
- Set file names access mode in Volume `ConfigMap` data
- Can be used by system components and controllers

#### Using ConfigMaps

Like Secrets you can use `ConfigMaps` as environment variables using a volume mount. They must exist prior to being used by a Pod unless marked as `optional`.

```
env:
- name: SPECIAL_LEVEL_KEY
  valueFrom:
    configMapKeyRef:
      name: special-config
      key: special.how
```

With volumes, you define a volume with the `configMap` type in your pod and mount it where it needs to be used.

```
volumes:
  - name: config-volume
    configMap:
      name: special-config
```

#### Deployment Configuration Status

```
status:
  availableReplicas: 2
  conditions:
  - lastTransitionTime: 2017-12-21T13:57:07Z
    lastUpdateTime: 2017-12-21T13:57:07Z
    message: Deployment has minimum availability.
    reason: MinimumReplicasAvailability
    status: "True"
    type: Available
  observedGeneration: 2
  readyReplicas: 2
  replicas: 2
  updatedReplicas: 2
```

`availableReplicas` - Indicates how many were configured by the ReplicaSet. This would be compared to the later value of `readyReplicas`, which would be used to determine if all replicas have neen fully generated and without error.

`observedGeneration` - Shows how often the deployment has been updated. This information can be used to understand the rollout and rollback situation of the deployment.

#### Scaling and Rolling Updates
The API Server allows for the configurations settings to be updates for most values. There are some immutable values which may be different depending on the version of Kubernetes you have deployed.

A common change is to update the number of replicas running.

Non-immutable values can be edited with a text editor as well as the `edit` command line option. For example:

```
kubectl edit deployment nginx
```

```
    comntainers:
    - image: nginx:1.8 #<<--- Set to older version
      imagePullPolicy: IfNotPresent
      name: dev-web
```

This would trigger a rolling update of the deployment. While the deployment would show an older age, a review of the Pods would show a recent update and older version of the web server application deployed.

#### Deployment Rollbacks
With all the `ReplicaSets` of a Deployment being kept, you can also roll back to a previous revision by scaling up and down the `ReplicaSets` the other way.

```
kubectl set image deployment ghost --image=ghost:0.9 --record
kubectl get deployments ghost -o yaml
```

```
metadata:
  annotations:
    deployment.kubernetes.io/revision: "1"
    kubernetes.io/change-cause: kubectl set image deployment ghost --image=ghost:0.9 --record
```

Should an update fail, due to the improper image version, for example, you can roll back the change to a working version with `kubectl rollout undo`

You can roll back to a specific revision with the `--to-revision=2` option.

You can pause a Deployment and then resume.
```
kubectl rollout pause deployment/ghost
kubectl rollout resume deployment/ghost
```

## Exercise 5.1: Configure the Deployment Secrets and ConfigMap
## Exercise 5.2: Configure the Deployment: Attaching Storage
## Exercise 5.3: Using ConfigMaps Configure Ambassador Containers
## Exercise 5.4: Rolling Updates and Rollbacks

#### Chapter 5: Deployment Configuration - Review

1. Applications must use persistent storage?
  - a. True
  - b. False

2. Does a Deployment use a Persistent Volume or Persistent Volume Claim?
  - a. Persistent Volume
  - b. Persistent Volume Claim

3. Which of the following setting determines what happens to a persistent storage upon release?
  - a. persistentVolumeReclaimPolicy
  - b. releasePolicy
  - c. StorageSettingsK8s
  - d. None if the above

4. A Secret contains encrypted data?
  - a. True
  - b. False

5. ConfigMaps can be created from ________.?
  - a. Literal values
  - b. Individual files
  - c. Multiple files in the same directory
  - d. All of the above

6. Which of the following arguments do we pass to the `kubectl `rollout` command to view the object revisions?
  - a. history
  - b. rollout
  - c. undo
  - d. redo

7. What argument do we pass to the `kubectl rollout` command in order to return to a previous version?
  - a. history
  - b. rollout
  - c. undo
  - d. redo

[answers](./review-answers.md)

#### Review
- Rolling back a deployment (5.20)

2. Does a Deployment use a Persistent Volume or Persistent Volume Claim?
  - a. Persistent Volume
  - b. Persistent Volume Claim

## Chapter 6: Security

#### Learning Objectives
- Explain the flow of API requests
- Configure authorization rules
- Examine authentication policies
- Restrict network traffic policies

#### Accessing the API
To perform any action in a Kubernetes cluster, you will need to access the API and go through three main steps:
1. Authentication
2. Authorization (ABAC or RBAC)
3. Admission Control

Authentication is the first step a request will hit. It will use any authentication module that has been configured.

Authorization is the second step where the request will be checked against existing policies. It will be authorized if the user has the permissions to perform the requested actions.

Admission is the last step where the actual content of the objects being created and validates them before admitting the request.

In addition, requests reaching the API Server over the network are encrypted using TLS. This needs to be properly configured using SSL certificates. If you're using `kubeadm` this configuration is done for you.

#### Authentication
In straightforward form, can be done with one of:
- Certificates
- Tokens
- Basic authentication (username and password)

Users are not created via the API, they should be managed by an external system.

Advanced authentication can be done via Webhooks.

#### Authorization
Once a request is authenticated, it needs to be authorized to be able to proceed through the Kubernetes system and perform its intended action.

There are three main authorization modes and two global Deny/Allow settings. The three main modes are:
- ABAC
- RBAC
- WebHook

Those are set via the `kube-apiserver` startup options. For example:

```
--authorization-mode=ABAC
--authorization-mode=RBAC
--authorization-mode=Webhook
--authorization-mode=AlwaysDeny
--authorization-mode=AlwaysAllow
```

The authorization modes implement policies to allow requests. Attributes of the requests are checked against the policies (e.g. user, group, namespace, verb).

#### ABAC (Attribute Based Access Control)
It was the first authorization method in Kubernetes that allowed administrators to implement the right policies. Today `RBAC` is becoming the default authorization mode.

Policies are defined in a JSON file and referenced by a `kube-apiserver` startup option:

```
--autorization-policy-file=my_policy.json
```

For example, the policy file shown below authorizes user _Bob_ to read pods in the namespace `foobar`:

```
{
  "apiVersion": "abac.authorization.kubernetes.io/v1beta",
  "kind": "Policy",
  "spec": {
    "user": "bob",
    "namespace": "foobar",
    "resource": "pods",
    "readonly": true
  }
}
```

#### RBAC (Role Based Access Control)
All resources are modeled API objects in Kubernetes. The also belong to `API Groups` such as `core` and `apps`. These resources allow operations such as Create, Read, Update, and Delete. Operations are called `verbs` inside YAML files.

Rules are operations which can act upon an API group. `Roles` are a group group of rules which affect, or scope, a single namespace, whereas `ClusterRoles` have a scope of the entire cluster.

Each operation can act upon one of three subjects, which are `User Accounts` which don't exist as API objects, `Server Accounts`, and `Groups` which are known as `clusterrolebinding` when using `kubectl`

`RBAC` is then writing rules to allow or deny operations by users, roles, or groups upon resources.

`RBAC` can be complex but the basic flow is creating a certificate for a user. As a user is not a API object it requires outside authentication such as OpenSSL certificates. After generating the certificate against the cluster certificate authority, we can set the credential for the user using a `context`.

`RBAC` process:
1. Determine or create namespace
2. Create certificate credentials for user
3. Set the credentials for the user to the namespace using a context
4. Create a role for the expected task set
5. Bind the user to the role
6. Verify the user has limited access

#### Admission Controller
The last step in letting an API request into Kubernetes is admission control.

Admission controllers are pieces of software that can access the content of the objects being created by the requests. They can modify content or validate it, and potentially deny the request.

To enable or disable the command line options are available.

```
--enable-admission-plugins=Initalizers,NamespaceLifecycle,LimitRanger --disable-admission-plugins=PodNameSelector
```

#### Security Contexts
Pods and containers within pods can be given specific constraints to limit what processes running in containers can do. For example, the UID of the process, the Linux capabilities, and the filesystem group can be limited.

The security limitation is called a security context. It can be defined for the entire pod or per container, and is represented as additional sections in the resources manifests. The notable difference is that Linux capabilities are set at the container level.

Security context example enforcing a rule that containers cannot run their process as the root user:
```
apiVersion: v1
kind: Pod
metadata:
  name: nginx
spec:
  securityContext:
    runAsNonRoot: true
  containers:
- image: nginx
  name: nginx
```

Then, when you create this pod, you will see a warning that the container is trying to run as root and that is not allowed. Hence, the Pod will never run.

#### Pod Security Policies
`PodSecurityPolicies` (PSP) can be used to enforce security contexts. A PSP is defined via a standard Kubernetes manifest following the POP API schema.

These policies are cluser-level rules that govern what a pod can do, what they can access, what they run as, etc.

For example, if you do not want any of the containers in your cluster to run as root, you could define a PSP to enforce that. You can also prevent containers from being privileged or use the host network namespace, or host PID namespace.

```
apiVersion: extenstions/v1beta1
kind: PodSecurityPolicy
metadata:
  name: restricted
spec:
  seLinux:
    rule: RunAsAny
  supplementalGroups:
    rule: RunAsAny
  runAsUser:
    rule: MustRunAsNonRoot
  fsGroup:
    rule: RunAsAny
```

For Pod Security Policies to be enables, you need to configure the admission controller of the `controller-manager` to contain `PodSecurityPolicy`. These policies make even more sense when coupled with the `RBAC` configuration in your cluster. This will allow you to finely tune what your users are allowed to run and what capabilities and low level privileges their containers will have.

#### Network Security Policies
By default all pods can reach each other; all ingress and egress traffic is allowed. Network isolation can be configured and traffic to pods can be blocked. Egress traffic can also be blocked in the newest versions of Kubernetes via configuring `NetworkPolicy`. Since all traffic is allowed by default, you could implement a policy to drop all traffic then other policies which allow desired ingress and egress traffic.

The `spec` of the policy can narrow down the effect to a particular namespace which can be handy. Further settings include a `podSelector` or label to narrow down which Pods are affected. Further ingress and egress settings declare traffic to and from IP addresses and ports.

Example showing only Pods with the label of `role: db` will be affected by this policy, and the policy has both Ingress and Egress settings.

The `ingress` setting includes a 172.17 network, with a smaller range of 127.17.1.0 IPs being excluded from this traffic.

```
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: ingress-egress-policy
  namespace: default
spec:
  podSelector:
    matchLabels:
      role: db
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - ipBlock:
        cidr: 172.17.0/16
        except:
        - 172.17.0/24
    - namespaceSelector:
      matchLabels:
        project: myproject
    - podSelector:
      matchLabels:
        role: frontend
    ports:
    - protocol: TCP
      port: 6379
  egress:
  - to:
    - ipBlock:
      cidr: 10.0.0.0/24
    ports:
    - protocol: TCP
      port: 5978
```

#### Default Policy Example

The empty braces will match all Pods not selected by other `NetworkPolicy` and will not allow ingress traffic. Egress traffic would be unaffected by this policy.
```
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny
spec:
  podSelector: {}
  policyTypes:
  - Ingress
```

#### Chapter 6: Security - Review
1. Which of the following is not part of accessing the Kubernetes API.
  - a. Authentication
  - b. Keystone
  - c. Authorization
  - d. Admission Control

2. Which accepts the --authorization-mode option to change the authorization tool in use?
  - a. kubelet
  - b. kubeadm
  - c. kubectl
  - d. kube-apiserver

3. We can specify egress rules with network policies?
  - a. True
  - b. False

[answers](./review-answers.md)

## Chapter 7: Exposing Applications

#### Learning Objectives
- Use services to expose an application
- Understand the CluseterIP service
- Configure the NodePort service
- Deploy the LoadBalancer service
- Discuss the ExternalName service
- Understand an ingress controller

#### Service Types
- *ClusterIp* is the default type. It only provides access internally (except if manually creating an external endpoint). The range of ClusterIp used is defined via an API server startup option.
- *NodePort* is great for debugging or when a static IP address is needed. The NodePort range is deifned in the cluster configuration.
- *LoadBalancer* was created to pass requests to a cloud provider or private cloud if they have a cloud provider plugin such as CloudStack and OpenStack.
- *ExternalName* is newer and has no selectors nor does it define ports or endpoints. It allows the return of an alias to an external service. The redirection happens at the DNS level, not via a proxy or forward. This object can be useful for services not yet brought into the Kubernetes cluster.

The `kubectl proxy` command creates a local service to access a ClusterIP. This can be useful for troubleshooting or development work.

#### Services Diagram
The `kube-proxy` running on cluster nodes watches the API server service resources. It presents a type of virtual IP server service resource.

#### Service Update Pattern
Labels are used to determine which Pods should recieve traffic from a service. Labels can be dynamically updated for an object, which may effect which Pods continue to connect to a service.

The default update pattern is for a rolling update where new Pods are added, with different versions of an application, and due to automatic load balancing, recieve traffic along with previous versions of the application.

Should there be a difference in applications deployed, such that clients would have issues communicating with different versions, you may consider a more specific label for the deployment, which includes a version number. When the deployment creates a new replication controller for the update, the label would not match. Once the new Pods have been created, and perhaps allowed to fully initialize, we would edit the labels for which the Service connects. Traffic would shift to the new and ready version, minimizing client version confusion.

#### Accessing an Application with a Service
The basic step to access a new service is to use `kubectl`.

```
kubeclt expose deployment/nginx --port=80 --type=NodePort
```

#### ClusterIP
For inter-cluster communication, frontends talking to backends can use CluserIPs. These addresses and endpoints only work within the cluster.

```
spec:
  clusterIP: 10.108.95.67
  ports:
  - name: "443"
    port: 443
    protocol:TCP
    targetPort: 443
```

#### NodePort
NodePort is a simple connection from a high-port routed to a CluseterIP using iptables, or ipvs in newer versions. The creation of a NodePort gererates a a ClusterIP by default. Traffic is routed from the NodePort to the ClusterIP. Only high ports can be used, as declared in the source code. The NodePort is accessible via calls to `<NodeIP>:<NodePort>`.

```
spec:
  clusterIP: 10.97.191.46
  externalTrafficPolicy: Cluster
  ports:
  - nodePort: 31070
    port: 80
    protocol: TCP
    targetPort: 800a0
  selector:
    io.kompose.service: nginx
  sessionAffinity: None
  type: NodePort
```

#### LoadBalancer
Creating a `LoadBalancer` service generates a `NodePort`, which then creates a `ClusterIP`. It also sends an asynchronous call to an external load balancer, typically supplied by a cloud provider. The `External-IP` value will remain in a `<Pending>` state until the load balancer returns. Should it not return, the `NodePort` created acts as it would otherwise.

```
Type: LoadBalancer
loadBalancerIP: 12.45.105.12
clusterIP: 10.5.31.33
ports:
- protocol: TCP
  Port: 80
```

The routing of traffic to a particular backend pod depends on the cloud provider in use.

#### ExternalName
The use of an `ExternalName` service, which is a special type of service without selectors,  is to point to an external DNS server. Use of the service returns a CNAME record. Working with the `ExternalName` service is handy when using a resource external to the cluster, perhaps prior to full intergration.

```
spec:
  Type: ExternalName
  externalName: ext.db.example.com
```

#### Ingress Resource
An ingress resource is an API object containing a list of rules matched against all incoming requests. Only HTTP rules are currently supported. In order for the controller to direct traffic to the backend, the HTTP request must match both the host and the path declared in the ingress.


#### Ingress Controller
Handling a few services can be easily done. However managing thousands or tens of thousands of services can create ineffciencies. The use of an Ingress Controller manages ingress rules to route traffic to existing services. Ingress can be used to fan out services, name-based hosting, TLS, or load balancing.

There are a few Ingress Controllers with nginx and GCE that are officaly supported by the community. Traffic and HAProxy are in common use also. Future support for HTTPS/ TLS is planned combining Level 4 and Level 7 ingress and requesting name or IP via claims.

#### Chapter 7: Exposing Applications - Review
1. A ClusterIP is accessible from outside the cluster?
  - a. True
  - b. False

2. Creating a LoadBalancer service starts a Pod running a loadbalancer?
  - a. True
  - b. False

3. Which of the following service does not use selectors?
  - a. ClusterIP
  - b. NodePort
  - c. LoadBalancer
  - d. ExternalName

4. To expose a low port using a Kubernetes object, you would have to use a __________.
  - a. ClusterIP
  - b. NodePort
  - c. LoadBalancer
  - d. Ingress Controller

[answers](./review-answers.md)

## Chapter 8: Troubleshooting

#### Learning Objectives
- Understand and use the troubleshooting flow
- Monitor applications
- Review system logs
- Review agent logs
- Discuss conformance testing
- Discuss cluster certification

#### Chapter 6: Exposing Applications - Review
1. Kubernetes includes cluster-wide logging?
  - a. True
  - b. False

2. Which is the CNCF project for monitoring and alerting?
  - a. Fluentd
  - b. Prometheus
  - c. Jaeger
  - d. Open Tracing

3. What does conformance ensure?
  - a. Workloads on one distribution work on others
  - b. API functions the same
  - c. Minimum functionality
  - d. All of the above
