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

## **configuration**

Create directory for the configuration file:

```shell
mkdir -p <PATH TO DIRECTORY>
```

Create a file named **settings.yml**

Copy the **settings.yml** on directory you create before:

```shell
cp settings.yml <PATH TO DIRECTORY>
```

### **settings.yml**

| **variable** | **description** |
| ------------- | --------------- |
| check_time | Time to wait for the certificate to be validated, is optional, if not set, it will be set to **86400** |
| **app_configs** |
| name | Name of the application |
| host | IP or DNS name of the application |
| port | Port of the application |
| environment | Environment of the application |
| debug | Debug mode of the application |
| **hosts** |
| name | Name of the certificate to validate |
| url | URL of the certificate to validate |
| port | Port of the certificate to validate |

```yml
---
check_time: 30

app_configs:
  - name: 'certificate-validate'
    host: '0.0.0.0'
    port: '5000'
    environment: 'development'
    debug: True 

hosts:
  - name: "github.com"
    url: 'github.com'
    port: '443'
```

**OBS:**

For validate more than one certificate, you can add more hosts in the **settings.yml** file.

```yml
hosts:
  - name: "github"
    url: "github.com"
    port: '443'
  - name: "gitlab"
    url: "gitlab.com"
    port: '443'
  - name: "twitter"
    url: "twitter.com"
    port: '443'
```

### **volume**

```shell
docker volume create --driver local -o o=bind -o type=none -o device=<DIR TO BIND> <NAME OF VOLUME>
```

**Example:**

```shell
docker volume create --driver local -o o=bind -o type=none -o device=/tmp/volume/certificate-validate certificate-validate
```

### **permissions**

```shell
chown -R 1000:1000 <DIR TO BIND ON VOLUME>
```

**Example:**

```shell
chown -R 1000:1000 /tmp/volume/certificate-validate
```

### **entrypoint**

```shell
"
usage: 

export API_HOST_ADDRESS=<hostname> or export API_HOST_ADDRESS=<ip>
export API_PORT=<port>

./entrypoint.sh [OPTIONS] [ARGUMENTS]

Ex. ./entrypoint.sh -i dev || ./entrypoint.sh -i prod || ./entrypoint.sh -h

optional arguments:
    -v, --version       show program's version number and exit
    -l, --local         run the program locally
        --check_time    Time to wait for the certificate to be validated, is optional, if not set, it will be set to **86400**
                        this time is configured in the **settings.yml** file
        --exit          Consult once and exit
    -i, --api           run the program on the API
        dev             run the program locally on the development environment
        prod            run the program on the production environment
    -h, --help          show this help message and exit
"
```

### **run local**

```shell
docker run -d --name certificate_validate_test \
-v <NAME OF VOLUME>:/app/config \
fabianoflorentino/certificate-validate:test --local --check_time
```

**Example:**

```shell
docker run -d --name certificate_validate_test \
-v certificate-validate:/app/config \
fabianoflorentino/certificate-validate:test --local --check_time
```

### **status**

```shell
CONTAINER ID   IMAGE                                         COMMAND                  CREATED          STATUS          PORTS         NAMES
d33be85a9e6b   fabianoflorentino/certificate-validate:test   "/app/entrypoint.sh …"   27 minutes ago   Up 27 minutes                 certificate_validate_test
```

### **run api**

#### **dev**

```shell
docker run -d --name certificate_validate_test \
-p 5000:5000 \
-v <NAME OF VOLUME>:/app/config \
fabianoflorentino/certificate-validate:test --api dev
```

#### **prod**

```shell
docker run -d --name certificate_validate_test \
-p 5000:5000 \
-e API_HOST_ADDRESS=<hostname> \
-e API_PORT=<port> \
-v <NAME OF VOLUME>:/app/config \
fabianoflorentino/certificate-validate:test --api prod
```

**Example:**

```shell
docker run -d --name certificate_validate_test \
-v certificate-validate:/app/config \
fabianoflorentino/certificate-validate:test --api dev
```

**OBS:**
The **api** option will run the application in **api mode**, the **check_time** option will be ignored.
Certificate information will be validated on time consult. If you want to validate the certificate periodically, you can use the **cron** option.

### **endpoints**

| **endpoints** | **description** |
| ------------- | --------------- |
| /api/v1/cert/info | Get the certificate informations |

#### **output**

```json
{
     "commonName": "github.com",
     "subjectAltName": "['github.com', 'www.github.com']",
     "issuer": "DigiCert High Assurance TLS Hybrid ECC SHA256 2020 CA1",
     "type": "Organization Validation (OV) Web Server SSL Digital Certificate",
     "notBefore": "2021-03-25 00:00:00",
     "notAfter": "2022-03-30 23:59:59",
     "daysLeft": "178",
     "crl": "['http://crl3.digicert.com/DigiCertHighAssuranceTLSHybridECCSHA2562020CA1.crl', 'http://crl4.digicert.com/DigiCertHighAssuranceTLSHybridECCSHA2562020CA1.crl']"
}
{
     "commonName": "gitlab.com",
     "subjectAltName": "['gitlab.com', 'auth.gitlab.com', 'customers.gitlab.com', 'email.customers.gitlab.com', 'gprd.gitlab.com', 'www.gitlab.com']",
     "issuer": "Sectigo RSA Domain Validation Secure Server CA",
     "type": "Domain Validation (DV) Web Server SSL Digital Certificate",
     "notBefore": "2021-04-12 00:00:00",
     "notAfter": "2022-05-11 23:59:59",
     "daysLeft": "220",
     "crl": "CRL not found for this certificate!"
}
{
     "commonName": "twitter.com",
     "subjectAltName": "['twitter.com', 'www.twitter.com']",
     "issuer": "DigiCert TLS RSA SHA256 2020 CA1",
     "type": "Organization Validation (OV) Web Server SSL Digital Certificate",
     "notBefore": "2021-02-09 00:00:00",
     "notAfter": "2022-02-07 23:59:59",
     "daysLeft": "127",
     "crl": "['http://crl3.digicert.com/DigiCertTLSRSASHA2562020CA1.crl', 'http://crl4.digicert.com/DigiCertTLSRSASHA2562020CA1.crl']"
}
```

**OBS:** Outputs are in **json** format.

### **logs**

**RFC (Request for Comments):** [Internet X.509 Public Key Infrastructure Certificate and CRL Profile](https://www.rfc-editor.org/rfc/rfc2459#section-4.1)

| **fields** | **description** |
| ------------- | --------------- |
| "commonName" | Common Name of the certificate |
| "subjectAltName" | Subject Alternative Name of the certificate |
| "issuer" | Issuer of the certificate |
| "type" | Type of the certificate |
| "notBefore" | Not Before of the certificate |
| "notAfter" | Not After of the certificate |
| "daysLeft" | Days left to expire the certificate |
| "crl" | Certificate Revocation List of the certificate |

**OBS**: daysLeft is not part of the RFC, it is calculated based on the notBefore and notAfter fields.

```shell
docker exec -it <CONTAINER NAME> cat /app/certificate.log
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
