# jupyter-hybrid-operator

To deploy the operator in the cluster:
```sh
make deploy
```

to debug locally:

```sh
make install # first time only
export WATCH_NAMESPACE={{mynamespace}}
dlv debug --headless --listen=:2345 --api-version=2
```

and attach through a remote go debug configuration to localhost:2345

create a new CR:

```
kc apply -f local.dossier.yaml -n jhub-pecetto
```
