# ACME webhook for google-domains DNS API
Usage:
```bash
helm repo add nblxa https://nblxa.github.io/charts
helm install my-release nblxa/cert-manager-webhook-google-domains
```

To test:
```bash
TEST_DOMAIN_NAME=<domain name> TEST_SECRET=$(echo -n '<google domains ACME API Key>' | base64) make test
```

# Example Issuer
```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: my.domain.com
spec:
  acme:
    email: me@my.domain.com
    server: https://dv.acme-v02.api.pki.goog/directory
    privateKeySecretRef:
      name: cert-domain-tls-auth-my.domain.com
    externalAccountBinding:
      keyID: <EAB KEY ID>
      keySecretRef:
        name: cert-domain-tls-key-skyloft.cc
        key: eab-key
    solvers:
    - dns01:
        webhook:
          groupName: acmedns.googleapis.com
          solverName: google-domains
          config:
            apiUrl: https://acmedns.googleapis.com/v1
            domainName: my.domain.com
            secretName: cert-domain-tls-key-my.domain.com
            secretKeyName: acme-key
```

# Example Secret
**Note**: Make sure to change the values.yaml `secretName` variable accordingly.
```yaml
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: cert-domain-tls-key-my.domain.com
  namespace: <YOUR NAMESPACE>
stringData:
  eab-key: <EAB KEY>
  acme-key: <ACME API KEY>
```

# Credits
This is based on the project [deyaeddin/cert-manager-webhook-hetzner](https://github.com/deyaeddin/cert-manager-webhook-hetzner) and [cert-manager/webhook-example](https://github.com/cert-manager/webhook-example)

Please feel free to fork/optimize/make an official version of this for release to https://artifacthub.io/.