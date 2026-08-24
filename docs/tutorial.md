---
myst:
  html_meta:
    "description lang=en": "A guided tutorial for deploying the bingo charm."
---

(tutorial)=

# Deploy the bingo charm for the first time

By the end of this tutorial, you'll have bingo running on Kubernetes with Juju, integrated
with PostgreSQL for persistent paste storage, and accessible from your browser.

## What you'll need

You will need a working station, e.g., a laptop, with AMD64 architecture. Your working station
should have at least 4 CPU cores, 8 GB of RAM, and 50 GB of disk space.

````{tip}
You can use Multipass to create an isolated environment by running:
```
multipass launch 24.04 --name bingo-tutorial-vm --cpus 4 --memory 8G --disk 50G
```
````

This tutorial requires the following software to be installed on your working station
(either locally or in the Multipass VM):

- Juju 3
- MicroK8s 1.33

Use [Concierge](https://github.com/canonical/concierge) to set up Juju and MicroK8s:

```
sudo snap install --classic concierge
sudo concierge prepare -p microk8s
```

This first command installs Concierge, and the second command uses Concierge to install
and configure Juju and MicroK8s.

For this tutorial, Juju must be bootstrapped to a MicroK8s controller. Concierge should
complete this step for you, and you can verify by checking for `msg="Bootstrapped Juju" provider=microk8s`
in the terminal output and by running `juju controllers`.

If Concierge did not perform the bootstrap, run:

```
juju bootstrap microk8s tutorial-controller
```

## What you'll do

1. Deploy the bingo charm
2. Integrate with the `postgresql-k8s` charm
3. Inspect the Kubernetes resources created
4. Access the bingo app
5. Clean up the environment

### Shell into the Multipass VM

```{note}
If you're working locally, you don't need to do this step.
```

To be able to work inside the Multipass VM first you need to log in with the following command:

```
multipass shell bingo-tutorial-vm
```

### Add a Juju model for the tutorial

To easily clean up the resources and separate your workload from the contents of this tutorial, set up a new Juju model named `bingo-tutorial`:

```
juju add-model bingo-tutorial
```

### Deploy the charms

bingo requires a connection to PostgreSQL for persistent paste storage.

Deploy the charms:

```
juju deploy bingo --channel 1/stable
juju deploy postgresql-k8s --channel 14/stable --trust
```

### Integrate with the PostgreSQL k8s charm

Integrate `postgresql-k8s` to `bingo`:

```
juju integrate bingo postgresql-k8s
```

By running `juju status --relations` the current state of the deployment can be queried:

```{terminal}
:output-only:

Model           Controller          Cloud/Region        Version  SLA          Timestamp
bingo-tutorial  concierge-microk8s  microk8s/localhost  3.6.27   unsupported  15:06:54-07:00

App             Version  Status  Scale  Charm           Channel    Rev  Address        Exposed  Message
bingo                    active      1  bingo           1/stable     3  10.152.183.95  no
postgresql-k8s  14.23    active      1  postgresql-k8s  14/stable  925  10.152.183.44  no

Unit               Workload  Agent  Address      Ports  Message
bingo/0*           active    idle   10.1.185.51
postgresql-k8s/0*  active    idle   10.1.185.54         Primary

Integration provider           Requirer                       Interface          Type     Message
bingo:secret-storage           bingo:secret-storage           secret-storage     peer
postgresql-k8s:database        bingo:postgresql               postgresql_client  regular
postgresql-k8s:database-peers  postgresql-k8s:database-peers  postgresql_peers   peer
postgresql-k8s:restart         postgresql-k8s:restart         rolling_op         peer
postgresql-k8s:upgrade         postgresql-k8s:upgrade         upgrade            peer
```

The deployment finishes when all the charms show `Active` states.

Run `kubectl get pods -n bingo-tutorial` to see the pods that are being created by the charms:

```{terminal}
:output-only:

NAME                             READY   STATUS    RESTARTS   AGE
modeloperator-c584f6f9f-qf9gr    1/1     Running   0          5m30s
bingo-0                          2/2     Running   0          5m1s
postgresql-k8s-0                 2/2     Running   0          5m9s
```

```{note}
If you get an "insufficient permissions" error running `kubectl` inside the Multipass VM,
run the following commands and start a new shell session:

    sudo usermod -a -G snap_microk8s ubuntu
    sudo chown -R ubuntu ~/.kube
    newgrp snap_microk8s
```

### Access the bingo app

bingo listens on port `8080` by default. To validate that you can successfully reach the
deployed workload, forward the port to your working station:

```
microk8s kubectl port-forward --address 0.0.0.0 service/bingo 8080:8080 -n bingo-tutorial
```

If you're following the tutorial locally, open `http://localhost:8080` in your browser.

If you're using a Multipass VM, find its IP with `multipass info bingo-tutorial-vm` and
navigate to `http://<vm-ip>:8080`.

### Clean up the environment

Congratulations! You have successfully finished the bingo tutorial. You can now remove the
model environment that you've created using the following command:

```
juju destroy-model bingo-tutorial --destroy-storage
```

If you used Multipass, to remove the Multipass instance you created for this tutorial, use the following command.

```
multipass delete --purge bingo-tutorial-vm
```

## Next steps

You achieved a basic deployment of the bingo charm. If you want to go further in your deployment
or learn more about the charm, check out these pages:

- Perform basic operations with your deployment, such as configuring the
  {ref}`base URL <how_to_set_base_url>` for generated paste links, or
  {ref}`rotating the session secret <how_to_rotate_secret_key>`.
- Expose bingo externally by integrating it with the
  [Traefik](https://charmhub.io/traefik-k8s) charm over the `ingress` relation.
- Learn more about the charm's actions, configuration options, and integration
  interfaces in the {ref}`Reference <reference_index>` section.
