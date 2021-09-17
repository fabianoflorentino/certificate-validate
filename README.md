# **certificate-validate**

![CI](https://img.shields.io/github/workflow/status/fabianoflorentino/certificate-validate/CI?label=CI) ![CodeQL](https://img.shields.io/github/workflow/status/fabianoflorentino/certificate-validate/CodeQL?label=CodeQL) ![Pylint](https://img.shields.io/github/workflow/status/fabianoflorentino/certificate-validate/Pylint?label=Pylint)

Validate some info in SSL/TLS Certificates

## **prerequisites**

* Docker
* Internet Access

## **build**

```shell
docker build --no-cache --rm -t <NAME_OF_IMAGE> -f ./Dockerfile .
```

## **run**

| **variable** | **description** |
| ------------- | --------------- |
| CERTIFICATE_URL | URL of the certificate to validate |
| CERTIFICATE_PORT | Port of the certificate to validate |
| CERTIFICATE_TIME_TO_WAIT | Time to wait for the certificate to be validated, is optional, if not set, it will be set to **86400** |

### **daemon**

```shell
docker run -d --name certificate_validate \
-e CERTIFICATE_URL=google.com \
-e CERTIFICATE_PORT=443 \
-e CERTIFICATE_TIME_TO_WAIT=6300 \
<NAME_OF_IMAGE>
```

#### **status**

```shell
docker ps
```

```shell
CONTAINER ID   IMAGE                                 COMMAND                CREATED          STATUS          PORTS     NAMES
e3b9598147db   fabianoflorentino/certificate-validate:latest   "/app/entrypoint.sh"   29 minutes ago   Up 29 minutes             certificate_validate
```

### **once**

```shell
docker run -it --name certificate_validate_test \
--entrypoint "" \
<NAME_OF_IMAGE> \
python /app/certificate.py github.com 443 --exit
```

### **logs**

**RFC (Request for Comments):** [Internet X.509 Public Key Infrastructure Certificate and CRL Profile](https://www.rfc-editor.org/rfc/rfc2459#section-4.1)

| **fields** | **description** |
| ------------- | --------------- |
| "commonName" | Common Name of the certificate |
| "SAN" | Subject Alternative Name of the certificate |
| "issuer" | Issuer of the certificate |
| "crl" | Certificate Revocation List of the certificate |
| "notBefore" | Not Before of the certificate |
| "notAfter" | Not After of the certificate |
| "type" | Type of the certificate |

```shell
docker exec -it <CONTAINER NAME> cat /app/certificate.log
```

```shell
{
     "commonName": "www.github.com",
     "SAN": "['www.github.com', '*.github.com', 'github.com', '*.github.io', 'github.io', '*.githubusercontent.com', 'githubusercontent.com']",
     "issuer": "DigiCert SHA2 High Assurance Server CA",
     "crl": "['http://crl3.digicert.com/sha2-ha-server-g6.crl', 'http://crl4.digicert.com/sha2-ha-server-g6.crl']",
     "notBefore": "2020-05-06 00:00:00",
     "notAfter": "2022-04-14 12:00:00",
     "type": "Organization Validation (OV) Web Server SSL Digital Certificate"
}
```

## **actions**

| **environment** | **description** |
| --------------- | ---------------- |
| DOCKERHUB | Environment configured on Github |

[**Environments**](https://docs.github.com/en/actions/reference/environments)

* [**Creating**](https://docs.github.com/en/actions/reference/environments#creating-an-environment)

| **variable** | **description** |
| ------------- | --------------- |
| secrets.DOCKERHUB_USERNAME | Username of the dockerhub account |
| secrets.DOCKERHUB_TOKEN | Token of the dockerhub account |
| GITHUB_REPOSITORY | Your GitHub repository needs to have the same name of Dockerhub Repository |

* [**secrets**](https://docs.github.com/en/actions/reference/encrypted-secrets)

    "Encrypted secrets allow you to store sensitive information in your organization, repository, or repository environments."

* [**Workflow syntax for GitHub Actions**](https://docs.github.com/en/actions/reference/workflow-syntax-for-github-actions)

    "A workflow is a configurable automated process made up of one or more jobs. You must create a YAML file to define your workflow configuration."

### **CI**

```yaml
---
name: CI

on:
  push:
    branches:
      - main
    paths-ignore:
      - 'README.md'
      - 'LICENSE'
      - 'docs/**'
      - '.github/**'

jobs:  
  build:
    environment: DOCKERHUB
    name: Build and Push to Docker Hub
    runs-on: ubuntu-latest

    steps:
      # Checkout the repository
      - name: Checkout
        uses: actions/checkout@v2

      # Login to Docker Hub
      - name: Login
        run: docker login -u ${{ secrets.DOCKERHUB_USERNAME }} -p ${{ secrets.DOCKERHUB_TOKEN }}

      # Build the image
      - name: Build
        run: |
          docker build \
          --no-cache \
          --rm \
          -t $GITHUB_REPOSITORY:latest \
          -f ./Dockerfile .
      
      # Push the image to Docker Hub
      - name: Push
        run: docker push $GITHUB_REPOSITORY:latest

```

### **Pylint**

```yaml
name: Pylint

on:
  push:
    branches:
      - main
    paths-ignore:
      - 'README.md'
      - 'LICENSE'
      - 'docs/**'
      - '.github/**'

jobs:
  build:

    runs-on: ubuntu-latest

    steps:
    - uses: actions/checkout@v2
    - name: Set up Python 3.9
      uses: actions/setup-python@v2
      with:
        python-version: 3.9
    - name: Install dependencies
      run: |
        python -m pip install --upgrade pip
        python -m pip install -r ./requirements.txt
        pip install pylint
    - name: Analysing the code with pylint
      run: |
        pylint `ls -R|grep .py$|xargs`

```

### **CodeQL**

```yaml
name: "CodeQL"

on:
  push:
    branches:
      - main
    paths-ignore:
      - 'README.md'
      - 'LICENSE'
      - 'docs/**'
      - '.github/**'

jobs:
  analyze:
    name: Analyze
    runs-on: ubuntu-latest
    permissions:
      actions: read
      contents: read
      security-events: write

    strategy:
      fail-fast: false
      matrix:
        language: [ 'python' ]

    steps:
    - name: Checkout repository
      uses: actions/checkout@v2

    # Initializes the CodeQL tools for scanning.
    - name: Initialize CodeQL
      uses: github/codeql-action/init@v1
      with:
        languages: ${{ matrix.language }}

    - name: Autobuild
      uses: github/codeql-action/autobuild@v1

    - name: Perform CodeQL Analysis
      uses: github/codeql-action/analyze@v1
```
