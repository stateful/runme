## Running

### In Minikube

Deploy the application to Minikube using the Linkerd2 service mesh.

1. Install the `linkerd` CLI

   ```bash
   curl https://run.linkerd.io/install | sh
   ```

1. Install Linkerd2

   ```bash
   linkerd install | kubectl apply -f -
   ```

1. View the dashboard!

   ```bash
   linkerd dashboard
   ```

1. Inject, Deploy, and Enjoy

   ```bash
   kubectl kustomize kustomize/deployment | \
       linkerd inject - | \
       kubectl apply -f -
   ```

1. Use the app!

   ```bash
   minikube -n emojivoto service web-svc
   ```

> Inside a block quote
>
> ```
> a + b
> ```
