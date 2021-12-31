package instance

type Option func(interface{}) error

type Base interface {
	Install(...Option) error
	Start(...Option) error
	Stop(...Option) error
	RunUntilSigInt()
}
