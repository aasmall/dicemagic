
# Dice Magic

***Slack Bot for Rolling Complex Dice***

## Build Status

|Branch|Build Status |
|--|--|
| master | [![Build Status](https://semaphoreci.com/api/v1/smallnet/dicemagic/branches/master/badge.svg)](https://semaphoreci.com/smallnet/dicemagic) |
| development |  [![Build Status](https://semaphoreci.com/api/v1/smallnet/dicemagic/branches/vdev/badge.svg)](https://semaphoreci.com/smallnet/dicemagic)|

## Set up a development environment

***n.b.*** *written from the standpoint of a Manjaro user. The [Arch wiki](https://wiki.archlinux.org/) will help a lot if yo get stuck.*

### Prerequisites:
 - [Google Cloud SDK](https://cloud.google.com/sdk/downloads)
 - [Go](https://golang.org/doc/install)
 - [jq](https://stedolan.github.io/jq/)
 - [Kustomize](https://github.com/kubernetes-sigs/kustomize/blob/master/docs/INSTALL.md)
 - Skaffold
 - hugo
 - [dnsmasq](https://wiki.archlinux.org/index.php/NetworkManager#dnsmasq) (or a ton of pain mucking about with /etc/hosts)

### Clone me

Clone this repo and change current directory to this repos root directory

### Install and Enable Docker

***n.b.*** *You may need to reboot after this step.*

 ```bash
 sudo pacman -S docker
 sudo usermod -aG docker $USER
 sudo systemctl start docker
 sudo systemctl enable docker
 ```

### Install Kubectl

```bash
curl -LO https://storage.googleapis.com/kubernetes-release/release/`curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt`/bin/linux/amd64/kubectl
chmod +x ./kubectl
sudo mv ./kubectl /usr/local/bin/kubectl
kubectl version --client
```

### Install KVM and start libvirtd
```bash
sudo pacman -S virt-manager qemu vde2 ebtables dnsmasq bridge-utils openbsd-netcat
sudo groupadd libvirt
sudo usermod -aG libvirt $USER
sudo systemctl enable libvirtd.service
sudo systemctl start libvirtd.service
```

### Install Minikube

***n.b.*** *this actually only works if you build minikube from HEAD...*

> `minikube version`
> ```bash
> minikube version: v1.10.0-beta.2
> commit: 80c3324b6f526911d46033721df844174fe7f597
> ```


```bash
curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64 \
   && sudo install minikube-linux-amd64 /usr/local/bin/minikube
```

### Start and Configure Minikube

```bash
minikube start --vm-driver=kvm2 --cpus=2 --nodes 3 --network-plugin=cni \
--addons registry --enable-default-cni=false \
--insecure-registry "10.0.0.0/24" --insecure-registry "192.168.39.0/24" \
--extra-config=kubeadm.pod-network-cidr=10.244.0.0/16 \
--extra-config=kubelet.network-plugin=cni
kubectl config use-context minikube
kubectl apply -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml
kubectl patch deployment coredns -n kube-system --patch '{"spec":{"template":{"spec":{"volumes":[{"name":"emptydir-tmp","emptyDir":{}}],"containers":[{"name":"coredns","volumeMounts":[{"name":"emptydir-tmp","mountPath":"/tmp"}]}]}}}}'
#if this didn't work, you may need to rm -rf ~/.minikube and try again
```

### Configure

Verify Successful cluster creation
```bash
kubectl cluster-info
```
```bash
Kubernetes master is running at https://XXX.XXX.XXX.XXX:8443
KubeDNS is running at https://XXX.XXX.XXX.XXX:8443/api/v1/namespaces/kube-system/services/kube-dns:dns/proxy

To further debug and diagnose cluster problems, use 'kubectl cluster-info dump'.
```

### Configure dnsmasq

***n.b.*** *Steps written assuming [NetworkManager](https://wiki.archlinux.org/index.php/Network_configuration#Network_managers).*
**You'll need to do this everytime you recreate the minikube cluster**

Set dnsmasq as NetworkManager's dns resolver.

Edit/create `/etc/NetworkManager/conf.d/dns.conf`

```toml
[main]
dns=dnsmasq
```

Get minikube's IP and configure dnsmasq to forward requests to it for local.dicemagic.io.

```bash
minikube ip
```

> ```bash
> xxx.xxx.xxx.xxx
> ```

Edit/create `/etc/NetworkManager/dnsmasq.d/address.conf` Use minikube's IP from above

```toml
address=/local.dicemagic.io/xxx.xxx.xxx.xxx
```

Reload NetworkManager

```bash
sudo nmcli general reload
```

### Configure Go and Build Dicemagic

Configure Go
```bash
go env -w GO111MODULE=on
echo 'export PATH=$PATH:'"$(go env GOPATH)/bin" >> ~/.bash_profile
```

Build Sicemagic

```bash
make build-all
```

### Generate Secrets

Generate secrets used by minikube's kustomize

```bash
make build-minikube-secrets
```

### Start Skaffold
```bash
skaffold dev --default-repo $(minikube ip):5000
```

forward client traffic to fake slack server
and open an ws client
```bash
kubectl port-forward service/mock-slack-server 8443:1082 8080:2082 &
./slack-client && kill $!
```

## Get Generators
first make sure that Go's bin directory is in your $PATH then:

```bash
go get github.com/golang/protobuf/protoc-gen-go
go get golang.org/x/tools/cmd/stringer
```

## Go write code...
