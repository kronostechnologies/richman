# Richman

A tool to manage [helmsman](https://github.com/Praqma/helmsman), list applications in a cluster, and allow you to run a templated application in your own node.

# Install
```
go get -u github.com/kronostechnologies/richman
# OR
go install
```

# Usage

## Exec a one-time command for an app
Applies and attaches to a "Job" template stored in a ConfigMap
```
richman apps exec '<command>' -a equisoft-connect -c name="myjob" -c cpu="1" -c memory="1G" -c templateparam="value"
```
Template parameter values are found in the ops configmap. They are strings like {{ .cpu }} or {{ .memory }} in the ops configmap itself.

## Run a one-time job for an app
Applies and attaches to a "Job" template stored in a ConfigMap
```
richman apps run -a equisoft-connect -c name="myjob" -c cpu="1" -c memory="1G" -c templateparam="value"
```
Template parameter values are found in the ops configmap. They are strings like {{ .cpu }} or {{ .memory }} in the ops configmap itself.

# Development
## Build and run
```
git clone git@github.com:kronostechnologies/richman.git
cd richman
make
./bin/richman
```

## Run test
```
make test
```

## Build container image
```
make package.image
docker run -it richman:latest
```
