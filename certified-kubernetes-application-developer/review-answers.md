#### Chapter 2: Kubernetes Architecture - Review
1. d
2. b
3. c
4. a

#### Chapter 3: Build - Review
1. b
2. d
3. a
4. false

#### Chapter 4
Evaluate Network Plugins

1. Which of the plugins allow `vxlans`?
- Canal, Flannel, Kopeio, Weave

2. Which are layer 2 plugins?
- Canal, Flannel, Kopeio, Weave

3. Which are layer 3 plugins?
- Calico, Romana, Kube Router

4. Which allow network policies?
- Calico, Canal, Kube Router, Romana, Weave

5. Which can encrypt all TCP and UDP traffic?
- Calico, Kopeio, Weave

#### Multi-container Pod Considerations
1. Which deployment method would allow the most flexibility, multiple applications per pod or one per pod?
- One per pod

2. Which deployment method allows for the most granular scalability?
- One per pod

3. Which have the best performance?
- Multi applications per pod

4.How many IP addresses are assigned per pod?
- 1

5. What are some ways containers can communicate within the same pod?
- IPC, loopback or shared filesystem access

6. What are some reasons you should have multiple containers per pod?
- When there is missing functionality or functionality you can decouple from the main container. Ambassadors and Sidecars are examples.

#### Chapter 4: Design - Review
1. Ambassador, Adapter, Sidecar
2. False
3. True
4. C

#### Chapter 5: Deployment Configuration - Review
1. False
2. Persistent Volume Claim
3. persistentVolumeReclaimPolicy
4. False
5. All of the above
6. history
7. undo

#### Chapter 6: Security - Review
1. Keystone
2. kube-apiserver
3. True

#### Chapter 7: Exposing Applications - Review
1. False
2. False
3. ExternalName
4. Ingress Controller

#### Chapter 8: Troubleshooting - Review
1. False
2. Prometheus
3. All of the above
