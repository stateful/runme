[object Object]

## Category1

```sh { name=test1 background=false category=cat1 excludeFromRunAll=false mimeType=text/plain promptEnv=true }
export SPOT_INSTANCE_REQUEST="$(aws ec2 describe-spot-instance-requests --filters 'Name=tag:foo,Values=bar' 'Name=state,Values=active,open' | jq -r '.SpotInstanceRequests[].SpotInstanceRequestId')"
export INSTANCE_ID="$(aws ec2 describe-spot-instance-requests --spot-instance-request-ids $SPOT_INSTANCE_REQUEST | jq -r '.SpotInstanceRequests[].InstanceId')"
aws ec2 stop-instances --instance-ids $INSTANCE_ID
export MY_VARIABLE=this is my long description
export VAR4=  $(echo hi)
export VAR=$(echo hello) world!
export VAR2=$(echo hello)
export VAR3='$(echo hello)'
echo $MY_VARIABLE
export MY_VARIABLE2="test2"
echo $MY_VARIABLE2
echo "vamos aca"
```

3. Setup pre-commit

```sh { name=test2 category=cat1 promptEnv=false }
echo $MY_VARIABLE
```

4. Initialize or update git submodules

```sh { name=test3 category=cat2 excludeFromRunAll=true }
echo "Category 2 value 1 3"
```

## Category 2

```sh { background=true category=cat2 }
echo "Category 2 value 2"
```

Or ingress

```sh { background=true category=cat2 }
echo "Category 2 value 3"
```

9. Run tests

```sh
echo "Without category"
```
