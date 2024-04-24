---
runme:
  id: 01HW1677G0SDVNNYEDJS3PBKS4
  version: v3
---

```sh {"id":"01HW167A4SWNJ0AQZGNZ1Z38XZ","name":"vars","promptEnv":"auto"}
export VAR_NAME='Placeholder 1'
export VAR_NAME2='Placeholder 2'
export VAR_NAME3=""

echo "1. ${VAR_NAME}"
echo "2. ${VAR_NAME2}"
echo "3. ${VAR_NAME3}"
```

```sh {"id":"01HW1B541A9P8BVBJZJ4E1XV1F","name":"vars2","promptEnv":"no"}
export VAR2_NAME='Placeholder 3'
export VAR2_NAME2='Placeholder 4'

echo "1. ${VAR_NAME}"
echo "2. ${VAR_NAME2}"
echo "3. ${VAR2_NAME}"
echo "4. ${VAR2_NAME2}"
```
