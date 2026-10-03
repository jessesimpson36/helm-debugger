# Understanding output

This page walks through a model-mode report.

## Full output

```text
================= HELPERS QUERY =================
test/templates/serviceaccount.yaml:5
      name: {{ include "test.serviceAccountName" . }}
  test/templates/_helpers.tpl:57
    in test.serviceAccountName
      {{- if .Values.serviceAccount.create }}
  test/templates/_helpers.tpl:58
    in test.serviceAccountName
      {{- default (include "test.fullname" .) .Values.serviceAccount.name }}
  test/templates/_helpers.tpl:14
    in test.fullname
      {{- if .Values.fullnameOverride }}
  test/templates/_helpers.tpl:17
    in test.fullname
      {{- $name := default .Chart.Name .Values.nameOverride }}
  test/templates/_helpers.tpl:18
    in test.fullname
      {{- if contains $name .Release.Name }}
  test/templates/_helpers.tpl:21
    in test.fullname
      {{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}

Relevant Values
- serviceAccount.create
- serviceAccount.name
- fullnameOverride
- nameOverride

WriteBuffer
   0  apiVersion: v1
   1  kind: ServiceAccount
   2  metadata:
   3    name: release-name-test
```

## Execution flow

```text
test/templates/deployment.yaml:30
          serviceAccountName: {{ include "test.serviceAccountName" . }}
  test/templates/_helpers.tpl:57
    in test.serviceAccountName
      {{- if .Values.serviceAccount.create }}
  test/templates/_helpers.tpl:58
    in test.serviceAccountName
      {{- default (include "test.fullname" .) .Values.serviceAccount.name }}
  test/templates/_helpers.tpl:14
    in test.fullname
      {{- if .Values.fullnameOverride }}
  test/templates/_helpers.tpl:17
    in test.fullname
      {{- $name := default .Chart.Name .Values.nameOverride }}
  test/templates/_helpers.tpl:18
    in test.fullname
      {{- if contains $name .Release.Name }}
  test/templates/_helpers.tpl:21
    in test.fullname
      {{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
```

This part is the **execution flow**. It shows each line that got executed on its
way to being rendered.

## Relevant values

```text
Relevant Values
- serviceAccount.create
- serviceAccount.name
- fullnameOverride
- nameOverride
```

Any time the execution flow references a `values.yaml` option via the `.Values`
keyword, it gets captured here. The actual values are not shown yet; for now it
tells you which `values.yaml` options to focus on when debugging the function.

## Write buffer

```text
WriteBuffer
   0  apiVersion: v1
   1  kind: ServiceAccount
   2  metadata:
   3    name: release-name-test
```

The write buffer is the rendered output. In Go's `text/template` library, the
write buffer is a string builder whose contents are captured. In most cases the
debugger displays a diff before/after the execution flow happens, but in this
case it was the first function, so the entire file was added to the buffer at
once.

A different write buffer that shows the changes might look like the following:

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

!!! note
    The write buffer display may glitch a little when multiple functions are
    called on the same line, such as:

    ```text
      24      spec:
      25        serviceAccountName: release-name-test
      26        containers:
      27          - name: test
      28            image: "nginx:1.16.0
    +     "
    +               imagePullPolicy: IfNotPresent
    ```

    The quote does get rendered correctly; the write buffer print just does not
    know that.
