This package is considered deprecated. New code should use https://github.com/pkg/errors . To learn more, visit: https://github.com/pkg/errors

- Tamal

---------------------------------------------------------------

# errors
Package errors provides detailed error types, and context support on top of go error.

Simple Usage
============

```go
type Context string

func (c Context) String() string {
	return "context: "+ string(c)
}
errors.New().WithMessage("error message").
	WithCause(err).
	WithContext(Context("custom context")).
	Failed()
```
######To Build Error
```go
err := errors.New().Internal() //Create external error
err := errors.New("Custom error message").
	WithCause(err).
	WithContext(Context("Custom context")).
	BadRequest() //Create BadRequest error with custom message, existing error and custom context
err := errors.New().
	WithMessage("Custom error message").
	External() // Create External error with custom message
```
######To Parse Error
```go
aError := errors.Parse(extError)
```
######To Extract Error Element
```go
c := aError.Context()
m :=  aError.Message()
code := aError.Code()
trace := aError.TraceString()
```
######To Test Error Type
```go
if errors.IsBadRequest(badReqError) {
	//Code
}

if errors.Is(badReqError, errors.BadRequest) {
    //Code
}
```

Available Error Types
=====================

```go
 External
 Failed
 Internal
 InvalidData
 InvalidPaymentInformation
 InvalidQuota
 NotFound
 PaymentInformationUnavailable
 PermissionDenied
 QuotaLimitExceed
 Unauthorized
 Unimplemented
 Unknown
```

Full documentation is available on [godoc](https://godoc.org/github.com/appscode/go/errors)
