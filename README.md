# Richman

A tool to manage [helmsman](https://github.com/Praqma/helmsman) specification toml files.

# Install
```
go get -u github.com/kronostechnologies/richman
# OR
go install
```

# Usage

## Update charts in a helmsman toml file
```
# Dry-run
richman chart update cluster.toml
# Update repositories
richman chart update cluster.toml -c stable --apply
# Update charts
richman chart update cluster.toml -c stable/prometheus-operator --apply
# Update charts by app name
richman chart update cluster.toml -a prometheus-operator -a nginx-ingress --apply
```

## List app version overrides in a helmsman toml file
Reads all image.tag overrides in "setString" sections
```
# Show all apps
richman apps list cluster.toml
# List a few apps
richman apps list cluster.toml -a myapp -a otherapp
APP       VERSION
myapp     1.2.2
otherapp  1.1.15
```

## Run a one-time job for an app
Applies and attaches to a "Job" template stored in a ConfigMap
```
richman apps run cluster.toml -a myapp -c cpu="1" -c memory="1G" -c templateparam="value"
```

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
