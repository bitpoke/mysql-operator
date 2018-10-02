[![Build Status](https://travis-ci.org/appscode/go.svg?branch=master)](https://travis-ci.org/appscode/go)
[![Go Report Card](https://goreportcard.com/badge/appscode/go "Go Report Card")](https://goreportcard.com/report/appscode/go)
[![GoDoc](https://godoc.org/github.com/appscode/go?status.svg "GoDoc")](https://godoc.org/github.com/appscode/go)

# go
Ensemble of GOlang libraries used by AppsCode

## Policy for adding new libs:
 * If codebase produces exe or needs vendoring, they live in their own top level repo.
 * appscode/go is our kitchen sink go lib repo. This must not use vendor. We do not accept contribution in /go repo,
  since they have wide ranging effect. This does not use log (glog transitively), since that will require vendoring.
   This also uses official errors pkg, to avoid facebookgo/stack dependency.
 * If we want external contribution, those stuff need their own repo.

## License
This is licensed under Apache 2.0 unless specified otherwise in individual code files.
