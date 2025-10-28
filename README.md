
## Helm debugger

## Overview

This project allows you to debug helm charts and hopefully answer how your values.yaml options change across _helper.tpl functions and eventually make it into the final rendered manifests.

The debugger makes use of dlv because golangs text/template library will process entire files at a time, making it difficult to track down individual operations without introspecting a running process.


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


## Running

(currently the project hardcodes the rendered manifest within the `test` folder)
To run the debugger, clone this repository and run the following commands:

```bash
make clone_helm
make compile_helm
make
```
