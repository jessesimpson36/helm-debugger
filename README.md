
## Helm debugger

## Overview

This project allows you to debug helm charts and hopefully answer how your values.yaml options change across `_helper.tpl` functions and eventually make it into the final rendered manifests.

The debugger makes use of dlv because golangs text/template library will process entire files at a time, making it difficult to track down individual operations without introspecting a running process.


## State of the project

This is still pretty experimental / proof of concept.


## Requirements

Delve version tested:

```
Delve Debugger
Version: 1.25.1
Build: $Id: 4e95e55b6b38b12e8509c91ec55261df1f7ee38f $
```

Other apps:
- Make
- Helm
- Git
- Golang

## Concepts / Terminology

This program starts a headless golang debugger in the background and translates places in memory of that debugger to helm chart logic. In the context of this program, I often refer to this as **Breakpoints**, even though there is not yet an interactive debugging mode of this program.

The execution **modes** of this program are simply alternative main functions I've been trying out to see which is most useful. I might remove the not-so-useful ones.

**Execution flow** refers to the linear path the program takes executing instructions after if/else conditions are evaluated. This may look a bit like a stacktrace, but it's not.

### Compiling Helm

Helm often discards their debug symbols, which I think I need to be able to hit breakpoints within delve. That's why there are options for specifying a custom helm binary path.

### Helm chart names vs paths

One of the things I haven't quite worked out yet is if you have a helm chart that is named something different from the folder it lives in, then it can be hard to do things like read lines from the template files that are being executed. Also, if you run the `helm template` command giving a tgz file or a path to a registry, I'm not sure that will work yet. It has to be a local folder named the same as the name of the chart.

### Modes
- **model**: This mode builds a complete data structure representing all execution flows within the chart templates and helpers. Then allows you to query which execution flows you want to follow.
- **branch**: This is the first mode I built and it only captures if/else conditions and whether they evaluate to true or false. It's not very useful. 
- **line**: After writing the branch flow, I wanted to print out every line as it's processed. This mode is pretty overwhelming without being filtered.


### Query types

Each time a breakpoint is hit, the program captures the execution path affecting that line.

- **template file queries**: these queries specify which Golang template files to set breakpoints on.
- **helper file queries**: these queries specify which helper files to set breakpoints on.
- **rendered file queries**: these queries specify which rendered manifest files to set breakpoints on.
- **values queries**: these queries specify which values to capture at each breakpoint or after rendering.

## Command line options

```
  -chart string
    	The name of the Helm chart to debug.
  -extra-command-args string
    	Additional command line arguments to pass to 'helm template' command.
  -helm-path string
    	Path to the compiled Helm binary. (default "helm")
  -helper-file string
    	Comma-delimited list of query files for helpers.
  -mode string
    	Mode of operation: model, branch, line (default "all")
  -rendered-file string
    	Comma-delimited list of query files for rendered manifest.
  -template-file string
    	Comma-delimited list of query files for templates and helpers.
  -values string
    	Comma-delimited list of values queries to capture.
```

## Running

(currently the project hardcodes the rendered manifest within the `test` folder)
To run the debugger, clone this repository and run the following commands:

```bash
make clone_helm
make compile_helm
make
```

The Makefile includes examples on how to work with the arguments.

## Example output and what it means

Full output:
```
================= HELPERS QUERY =================
test/templates/serviceaccount.yaml:5
      name: {{ include "test.serviceAccountName" . }}
  test/templates/_helpers.tpl:57
    in test.serviceAccountName
      {{- if .Values.serviceAccount.create }}
  test/templates/_helpers.tpl:57
    in test.serviceAccountName
      {{- if .Values.serviceAccount.create }}
  test/templates/_helpers.tpl:58
    in test.serviceAccountName
      {{- default (include "test.fullname" .) .Values.serviceAccount.name }}
  test/templates/_helpers.tpl:14
    in test.fullname
      {{- if .Values.fullnameOverride }}
  test/templates/_helpers.tpl:14
    in test.fullname
      {{- if .Values.fullnameOverride }}
  test/templates/_helpers.tpl:17
    in test.fullname
      {{- $name := default .Chart.Name .Values.nameOverride }}
  test/templates/_helpers.tpl:18
    in test.fullname
      {{- if contains $name .Release.Name }}
  test/templates/_helpers.tpl:18
    in test.fullname
      {{- if contains $name .Release.Name }}
  test/templates/_helpers.tpl:21
    in test.fullname
      {{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}

Relevant Values
- serviceAccount.create
- serviceAccount.create
- serviceAccount.name
- fullnameOverride
- fullnameOverride
- nameOverride

WriteBuffer
   0  apiVersion: v1
   1  kind: ServiceAccount
   2  metadata:
   3    name: release-name-test
```

### Breakdown

#### Execution flow
```
test/templates/serviceaccount.yaml:5
      name: {{ include "test.serviceAccountName" . }}
  test/templates/_helpers.tpl:57
    in test.serviceAccountName
      {{- if .Values.serviceAccount.create }}
  test/templates/_helpers.tpl:57
    in test.serviceAccountName
      {{- if .Values.serviceAccount.create }}
  test/templates/_helpers.tpl:58
    in test.serviceAccountName
      {{- default (include "test.fullname" .) .Values.serviceAccount.name }}
  test/templates/_helpers.tpl:14
    in test.fullname
      {{- if .Values.fullnameOverride }}
  test/templates/_helpers.tpl:14
    in test.fullname
      {{- if .Values.fullnameOverride }}
  test/templates/_helpers.tpl:17
    in test.fullname
      {{- $name := default .Chart.Name .Values.nameOverride }}
  test/templates/_helpers.tpl:18
    in test.fullname
      {{- if contains $name .Release.Name }}
  test/templates/_helpers.tpl:18
    in test.fullname
      {{- if contains $name .Release.Name }}
  test/templates/_helpers.tpl:21
    in test.fullname
      {{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
```

This part is the **Execution flow**. It shows each line that got executed on it's way to being rendered.

#### Relevant Values

```
Relevant Values
- serviceAccount.create
- serviceAccount.create
- serviceAccount.name
- fullnameOverride
- fullnameOverride
- nameOverride
```

Any time the execution flow references a `values.yaml` option via the `.Values` keyword, it gets captured here. One day I'd like for the actual values to be shown here too, but for now, it tells the user that you should focus on these `values.yaml` options when debugging the function.


#### WriteBuffer

```
WriteBuffer
   0  apiVersion: v1
   1  kind: ServiceAccount
   2  metadata:
   3    name: release-name-test
```

The write buffer is the rendered output. In golangs text/template library, the write buffer is a string builder that I captured the contents of. In most cases, I try to display a diff of the before/after the execution flow happens, but in this case, it was the first function, so the entire file was added to the buffer at once.

A different WriteBuffer that shows the changes might look like the following:

```diff
WriteBuffer
   0  apiVersion: apps/v1
   1  kind: Deployment
   2  metadata:
   3    name: release-name-test
   4    labels:
   5      helm.sh/chart: test-0.1.0
   6      app.kubernetes.io/name: test
   7      app.kubernetes.io/instance: release-name
   8      app.kubernetes.io/version: "1.16.0"
   9      app.kubernetes.io/managed-by: Helm
  10  spec:
  11    replicas: 1
  12    selector:
  13      matchLabels:
  14        app.kubernetes.io/name: test
  15        app.kubernetes.io/instance: release-name
  16    template:
  17      metadata:
  18        labels:
  19          helm.sh/chart: test-0.1.0
  20          app.kubernetes.io/name: test
  21          app.kubernetes.io/instance: release-name
  22          app.kubernetes.io/version: "1.16.0"
  23          app.kubernetes.io/managed-by: Helm
  24      spec:
  25        serviceAccountName: release-name-test
  26        containers:
  27          - name: test
  28            image: "nginx:1.16.0"
  29            imagePullPolicy: IfNotPresent
+     
+               ports:
+                 - name: http
+                   containerPort: 80
```

The write buffer display might also glitch out a little if there are multiple functions being called on the same line. such as:

```
  24      spec:
  25        serviceAccountName: release-name-test
  26        containers:
  27          - name: test
  28            image: "nginx:1.16.0
+     "
+               imagePullPolicy: IfNotPresent
```

The quote does get rendered correctly, but the print of the write buffer doesn't know that.
