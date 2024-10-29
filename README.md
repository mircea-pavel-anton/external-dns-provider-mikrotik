# ExternalDNS Webhook Provider for Mikrotik

> [!IMPORTANT]
> While this software has reached version `v1.0.0`, it has not yet undergone extensive testing in large-scale, real-world environments. As such, it may still have bugs and may not yet be fully suitable for production use.
>
> I encourage users to report any issues or suggest improvements, as this project remains under active development. Thank you for contributing!

[ExternalDNS](https://github.com/kubernetes-sigs/external-dns) is a Kubernetes add-on for automatically managing DNS records for Kubernetes ingresses and services by using different DNS providers. This webhook provider allows you to automate DNS records from your Kubernetes clusters into your MikroTik router.

Supported DNS record types: `A`, `AAAA`, `CNAME`, `MX`, `NS`, `SRV`, `TXT`

For examples of creating DNS records either via CRDs or via Ingress/Service annotations, check out the [`example/` directory](./example/).

## 🎯 Requirements

> [!Note]
> `v0.15.0` of ExternalDNS added support for `providerSpecific` annotations in Ingress/Service objects for webhook providers.
>
> While older versions of ExternalDNS may work, but support for this feature will not be present.

- ExternalDNS >= `v0.15.0`
- Mikrotik RouterOS (tested on `7.16` stable)

## 🚫 Limitations

### Multiple `Targets`

Currently, `DNSEndpoints` with multiple `targets` are **not** supported. No error will be thrown, but only one record will be created with the first target from the list.

This means that when creating a `DNSEndpoint` like this, only the first of the two targets will be taken into account (i.e. `192.192.192.192`).

```yaml
---
apiVersion: externaldns.k8s.io/v1alpha1
kind: DNSEndpoint
metadata:
  name: a-record
spec:
  endpoints:
    - dnsName: a.example.com
      recordTTL: 3605
      recordType: A
      targets:
        - 192.192.192.192
        - 193.193.193.193

```

The problem is that the External DNS controller will detect a drift on this and it will continuously attempt to update the DNS record, thus it will constantly send `PUT` requests to your RouterOS instance on every reconciliation loop.

## ⚙️ Configuration Options

### MikroTik Configuration

| Environment Variable        | Description                                                         | Default Value |
|-----------------------------|---------------------------------------------------------------------|---------------|
| `MIKROTIK_BASEURL`          | URL at which the RouterOS API is available. (ex. `https://192.168.88.1:443`)               | N/A           |
| `MIKROTIK_USERNAME`         | Username for the RouterOS API authentication.                   | N/A        |
| `MIKROTIK_PASSWORD`         |    Password for the RouterOS API authentication.         | N/A     |
| `MIKROTIK_SKIP_TLS_VERIFY`  | Whether to skip TLS verification (true or false).               | `false`           |
| `LOG_FORMAT` | The format in which logs will be printed. (`text` or `json`) | `json`       |
| `LOG_LEVEL`                 | The verbosity at which logs are printed logs. (`debug`, `info`, `warn` or `error`)        | `info`        |

### Webhook Server Configuration

| Environment Variable             | Description                                                      | Default Value |
|----------------------------------|------------------------------------------------------------------|---------------|
| `SERVER_HOST`                    | The host address where the server listens.                       | `localhost`   |
| `SERVER_PORT`                    | The port where the server listens.                               | `8888`        |
| `SERVER_READ_TIMEOUT`            | Duration the server waits before timing out on read operations.  | N/A           |
| `SERVER_WRITE_TIMEOUT`           | Duration the server waits before timing out on write operations. | N/A           |
| `DOMAIN_FILTER`                  | List of domains to include in the filter.                        | Empty         |
| `EXCLUDE_DOMAIN_FILTER`          | List of domains to exclude from filtering.                       | Empty         |
| `REGEXP_DOMAIN_FILTER`           | Regular expression for filtering domains.                        | Empty         |
| `REGEXP_DOMAIN_FILTER_EXCLUSION` | Regular expression for excluding domains from the filter.        | Empty         |

## 🚀 Deployment

1. Create a service account in RouterOS. This local user needs read and write access to manage static DNS.
2. Create a Kubernetes namespace for your External DNS deployment

    ```yaml
    ---
    apiVersion: v1
    kind: Namespace
    metadata:
      name: external-dns
    ```

3. Create a Kubernetes secret with the connection details for your RouterOS instance:

    ```yaml
    ---
    apiVersion: v1
    kind: Secret
    metadata:
      name: mikrotik-credentials
      namespace: external-dns
    stringData:
      MIKROTIK_BASEURL: "https://192.168.88.1:443"
      MIKROTIK_USERNAME: "external-dns"
      MIKROTIK_PASSWORD: "external-dns"
      MIKROTIK_SKIP_TLS_VERIFY: "true"
    ```

4. Add the External DNS helm repository and update your local cache

    ```bash
    helm repo add external-dns https://kubernetes-sigs.github.io/external-dns/
    helm repo update
    ```

5. Configure your helm values. Take a look at the [example values.yaml](./example/values.yaml)
6. Install the External DNS helm chart

    ```bash
    helm upgrade --install --namespace external-dns external-dns external-dns/external-dns -f values.yaml
    ```

> [!TIP]
> By default, support for MX, NS and SRV records is disabled and needs to be enabled via the `--managed-record-types` argument.
> Make sure to set `--managed-record-types=SRV` if you want to enable SRV records, and so on.

## ⭐ Stargazers

[![Star History Chart](https://api.star-history.com/svg?repos=mirceanton/external-dns-mikrotik-webhook&type=Date)](https://star-history.com/#mirceanton/external-dns-mikrotik-webhook&Date)

---

## 🤝 Gratitude and Thanks

Thanks to all the people who donate their time to the [Home Operations](https://discord.gg/home-operations) Discord community.
