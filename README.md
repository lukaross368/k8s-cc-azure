## Concept 
Cookie cutter / template for IAC + bootstrap self managed k8s cluster on azure. Building on kubernetes-the-hard-way 

Reference: 
-https://github.com/ivanfioravanti/kubernetes-the-hard-way-on-azure
-https://kubernetes.io/docs/reference/setup-tools/kubeadm/

## High level Components

Overall requirments are that a client should be able to run a github actions deployment pipeline providing only a single yaml file as input. This yaml file will act as the blueprint for your HA self managed cluster and the pipeline will deploy all Infra using a custom terraform module and boostrap everything using a custom anisble playbook. 

### IAC

- terraform module to spin up and connect required azure infra
- tests for terraform module

### Playbook

- Ansible playbook to bootstrap instances and install required software
- Smoke test script to check cluster is up and running 

### Install Addons 

- automatic solution for installing some default monitoring or logging

### Deployment Pipeline

- repo should expose a github actions pipeline that looks at a config file and runs the module and playbook from this repo to deploy the solution 

### Repo House keeping 

- some repo-wide static analysis / code formatting 
- build pipeline that checks tests can be run, infra can be spun up and playbook can be executed sucessfully. (Exectued on PR to main)

### Documentation 

- docs details supported configurations and client usage guide.

