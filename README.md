## Concept 
Cookie cutter / template for IAC + bootstrap self managed k8s cluster on azure. Building on kubernetes-the-hard-way 

References: 
- https://github.com/ivanfioravanti/kubernetes-the-hard-way-on-azure
- https://kubernetes.io/docs/reference/setup-tools/kubeadm/

## High level Components

Overall requirments are that a client should be able to run a github actions pipeline using some parameters. The pipeline should deploy all Infra using a custom terraform module and boostrap everything using kubeadm and then run some test scripts.

### IAC

- terraform module to spin up and connect required azure infra
- tests for terraform module

### Bootstrap

- use kubeadm for bootstrapping 
- Smoke test script to check cluster is up and running 

### Install Addons 

- automatic solution for installing some default monitoring or logging

### Deployment Pipeline

- repo should expose a github actions pipeline that takes some parameters and runs the module and scrips from this repo to deploy the solution 

### Repo House keeping 

- some repo-wide static analysis / code formatting 
- build pipeline that checks tests can be run, infra can be spun up and scripts can be executed sucessfully. (Exectued on PR to main)

### Documentation 

- docs details supported configurations and client usage guide.

