# Steward Console

This guide will introduce you to the basics of the Steward Console, a web UI to help you to view and control your Steward setup.

When you login to the console you are brought to the Tenant Control Planes, which allows you to quickly understand the state of your Steward setup at a glance. It shows summary information about all the Tenant Control Plane objects, including: name, namespace, status, endpoint, version, and datastore.

![Steward Console](../images/steward-console.png)

## Install with Helm
The Steward Console is a web interface running on the Steward Management Cluster that you can install with Helm. Check the Helm Chart [documentation](https://github.com/butlerlabs/steward-console) for all the available settings.

The Steward Console requires a Secret in the Steward Management Cluster that contains the configuration and credentials to access the console from the browser. You can have the Helm Chart generate it for you, or create it yourself and provide the name of the Secret during installation.

Before to install the Steward Console, access your workstation, replace the placeholders with actual values, and execute the following command:

```bash
# The secret is required, otherwise the installation will fail
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: steward-console
  namespace: steward-system
data:
  # Credentials to login into console
  ADMIN_EMAIL: <email>
  ADMIN_PASSWORD: <password>
  # Secret used to sign the browser session
  JWT_SECRET: <jwtSecret>
  # URL where the console is accessible: https://<hostname>/ui
  NEXTAUTH_URL: <nextAuthUrl>
EOF
```

Install the Chart with the release name `console` in the `steward-system` namespace:

```
helm repo add butlerlabs https://butlerlabs.github.io/charts
helm repo update
helm -n steward-system install console butlerlabs/steward-console
helm status console -n steward-system
```

## Access the Steward Console
Once installed, forward the console service to the local machine:

```
kubectl -n steward-system port-forward service/console-steward-console 8080:80
Forwarding from 127.0.0.1:8080 -> 3000
Forwarding from [::1]:8080 -> 3000
```

and point the browser to `http://127.0.0.1:8080/ui` to access the console. Login with credentials you stored into the secret.

!!! note "Expose with Ingress"
     The Steward Console can be exposed with an ingress. Refer the Helm Chart documentation on how to configure it properly.

## Additional Operations
The Steward Console offers additional capabilities unlocked by Butler Labs Enterprise Platform:

- Infrastructure Drivers Management
- Applications Delivery
- Centralized Authentication and Access Control
- Auditing and Logging
- Monitoring
- Backup & Restore

