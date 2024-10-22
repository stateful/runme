---
runme:
  id: 01JAS615VRFZGEK6GV7QQ9MBN4
  version: v3
terminalRows: 26
---

## Fake JWTs for testing

Make it easy to generate faux JWTs for testing purposes.

First set up the payload:

```json {"id":"01JAS617E49F122NDBYH3WT93J","name":"PAYLOAD"}
{
  "https://us-central1.stateful.com/app_metadata": {
    "userId": "43fe679c-3a7f-4465-a566-77120d390607"
  },
  "iss": "https://identity.stateful.com/",
  "sub": "google-faux-oauth2|9919447619479012",
  "aud": [
    "https://us-central1.stateful.com/",
    "https://stateful-inc.us.auth0.com/userinfo"
  ],
  "scope": "openid profile email",
  "azp": "go-test",
  "permissions": []
}
```

Sign it using a HMAC-based signature. The secret key is `key`.

```sh {"id":"01JAS639FAQM2NFHPRNHND84CM","name":"JWT","terminalRows":"8"}
export KEY="secret-key"
echo $PAYLOAD | step crypto jwt sign --alg=HS512 --exp=2722548701 --iat=1722548701 --subtle --key=<(echo $KEY) -
```

Verify and decode the JWT.

```sh {"id":"01JAS6E4C53XMQ56S38RP1N8B1"}
echo $JWT | step crypto jwt verify --alg=HS512 --subtle --key=<(echo $KEY)
```
