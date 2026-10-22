module bughunt

// NOTE: keep this at 1.21 on purpose. The go directive controls loop-variable
// semantics (pre-1.22 = one shared variable per loop) and http.ServeMux behaviour.
// Requires Go 1.21 or newer installed.
go 1.21
