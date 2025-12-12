#!/usr/bin/env bash

minikube start \
    --driver=virtualbox \
    --nodes 3 \
    --container-runtime containerd \
    --cni cilium \
    --cpus 2 \
    --memory 4g \
    --disk-size 20g
