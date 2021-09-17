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

```shell
docker run -d --name certificate_validate_google \
-e CERTIFICATE_URL=google.com \
-e CERTIFICATE_PORT=443 \
<NAME_OF_IMAGE>
```

```shell
docker ps

CONTAINER ID   IMAGE                                 COMMAND                CREATED          STATUS          PORTS     NAMES
e3b9598147db   fabianoflorentino/certificate:latest   "/app/entrypoint.sh"   29 minutes ago   Up 29 minutes             certificate_validate
```

```shell
docker exec -it <CONTAINER NAME> cat /app/certificate.log

Ex. docker exec -it certificate_validate_google cat /app/certificate.log

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

on: [push]

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
# For most projects, this workflow file will not need changing; you simply need
# to commit it to your repository.
#
# You may wish to alter this file to override the set of languages analyzed,
# or to provide custom queries or build logic.
#
# ******** NOTE ********
# We have attempted to detect the languages in your repository. Please check
# the `language` matrix defined below to confirm you have the correct set of
# supported CodeQL languages.
#
name: "CodeQL"

on:
  push:
    branches: [ main ]
  pull_request:
    # The branches below must be a subset of the branches above
    branches: [ main ]
  schedule:
    - cron: '17 23 * * 2'

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
        # CodeQL supports [ 'cpp', 'csharp', 'go', 'java', 'javascript', 'python' ]
        # Learn more:
        # https://docs.github.com/en/free-pro-team@latest/github/finding-security-vulnerabilities-and-errors-in-your-code/configuring-code-scanning#changing-the-languages-that-are-analyzed

    steps:
    - name: Checkout repository
      uses: actions/checkout@v2

    # Initializes the CodeQL tools for scanning.
    - name: Initialize CodeQL
      uses: github/codeql-action/init@v1
      with:
        languages: ${{ matrix.language }}
        # If you wish to specify custom queries, you can do so here or in a config file.
        # By default, queries listed here will override any specified in a config file.
        # Prefix the list here with "+" to use these queries and those in the config file.
        # queries: ./path/to/local/query, your-org/your-repo/queries@main

    # Autobuild attempts to build any compiled languages  (C/C++, C#, or Java).
    # If this step fails, then you should remove it and run the build manually (see below)
    - name: Autobuild
      uses: github/codeql-action/autobuild@v1

    # ℹ️ Command-line programs to run using the OS shell.
    # 📚 https://git.io/JvXDl

    # ✏️ If the Autobuild fails above, remove it and uncomment the following three lines
    #    and modify them (or add more) to build your code if your project
    #    uses a compiled language

    #- run: |
    #   make bootstrap
    #   make release

    - name: Perform CodeQL Analysis
      uses: github/codeql-action/analyze@v1
```
