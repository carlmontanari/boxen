package instance

// Option is the option function used to apply instance options.
type Option func(interface{}) error

// Base is the bare minimum interface a "Platform" must implement in order for boxen to manage an
// "instance".
type Base interface {
	Install(...Option) error
	Start(...Option) error
	Stop(...Option) error
	RunUntilSigInt()
}
