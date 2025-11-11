
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
